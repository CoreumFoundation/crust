package cored

import (
	"context"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmoserrors "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/pkg/errors"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/CoreumFoundation/crust/pkg/retry"
)

const (
	requestTimeout       = 10 * time.Second
	txTimeout            = time.Minute
	txStatusPollInterval = 500 * time.Millisecond
)

var expectedSequenceRegExp = regexp.MustCompile(`account sequence mismatch, expected (\d+), got \d+`)

// NewClient creates new client for cored
func NewClient(chainID string, addr string) Client {
	switch {
	case strings.HasPrefix(addr, "tcp://"),
		strings.HasPrefix(addr, "http://"),
		strings.HasPrefix(addr, "https://"):
	default:
		addr = "http://" + addr
	}

	rpcClient, err := client.NewClientFromNode(addr)
	must.OK(err)
	clientCtx := NewContext(chainID, rpcClient)
	return Client{
		clientCtx:       clientCtx,
		authQueryClient: authtypes.NewQueryClient(clientCtx),
		bankQueryClient: banktypes.NewQueryClient(clientCtx),
		wasmQueryClient: wasmtypes.NewQueryClient(clientCtx),
	}
}

// Client is the client for cored blockchain
type Client struct {
	clientCtx       client.Context
	authQueryClient authtypes.QueryClient
	bankQueryClient banktypes.QueryClient
	wasmQueryClient wasmtypes.QueryClient
}

// GetNumberSequence returns account number and account sequence for provided address
func (c Client) GetNumberSequence(ctx context.Context, address string) (uint64, uint64, error) {
	addr, err := sdk.AccAddressFromBech32(address)
	must.OK(err)

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var header metadata.MD
	res, err := c.authQueryClient.Account(requestCtx, &authtypes.QueryAccountRequest{Address: addr.String()}, grpc.Header(&header))
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	var acc authtypes.AccountI
	if err := c.clientCtx.InterfaceRegistry.UnpackAny(res.Account, &acc); err != nil {
		return 0, 0, errors.WithStack(err)
	}

	return acc.GetAccountNumber(), acc.GetSequence(), nil
}

// QueryBankBalances queries for bank balances owned by wallet
func (c Client) QueryBankBalances(ctx context.Context, wallet Wallet) (map[string]Coin, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	// FIXME (wojtek): support pagination
	resp, err := c.bankQueryClient.AllBalances(requestCtx, &banktypes.QueryAllBalancesRequest{Address: wallet.Key.Address()})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	balances := map[string]Coin{}
	for _, b := range resp.Balances {
		balances[b.Denom] = Coin{Amount: b.Amount.BigInt(), Denom: b.Denom}
	}
	return balances, nil
}

// Sign takes message, creates transaction and signs it
func (c Client) Sign(ctx context.Context, input BaseInput, msgs ...sdk.Msg) (authsigning.Tx, error) {
	signer := input.Signer
	if signer.AccountNumber == 0 && signer.AccountSequence == 0 {
		var err error
		signer.AccountNumber, signer.AccountSequence, err = c.GetNumberSequence(ctx, signer.Key.Address())
		if err != nil {
			return nil, err
		}

		input.Signer = signer
	}

	return signTx(c.clientCtx, input, msgs...)
}

// EstimateGas runs the transaction cost estimation and returns new suggested gas limit,
// in contrast with the default Cosmos SDK gas estimation logic, this method returns unadjusted gas used.
func (c Client) EstimateGas(ctx context.Context, input BaseInput, msgs ...sdk.Msg) (uint64, error) {
	signer := input.Signer
	if signer.AccountNumber == 0 && signer.AccountSequence == 0 {
		var err error
		signer.AccountNumber, signer.AccountSequence, err = c.GetNumberSequence(ctx, signer.Key.Address())
		if err != nil {
			return 0, err
		}

		input.Signer = signer
	}

	simTxBytes, err := buildSimTx(c.clientCtx, input, msgs...)
	if err != nil {
		err = errors.Wrap(err, "failed to build sim tx bytes")
		return 0, err
	}

	txSvcClient := txtypes.NewServiceClient(c.clientCtx)
	simRes, err := txSvcClient.Simulate(ctx, &txtypes.SimulateRequest{
		TxBytes: simTxBytes,
	})
	if err != nil {
		err = errors.Wrap(err, "failed to simulate the transaction execution")
		return 0, err
	}

	// usually gas has to be multiplied by some adjustment coeff: e.g. *1,5
	// but in this case we return unadjusted, so every module can decide the adjustment value
	unadjustedGas := simRes.GasInfo.GasUsed

	return unadjustedGas, nil
}

// Encode encodes transaction to be broadcasted
func (c Client) Encode(signedTx authsigning.Tx) []byte {
	return must.Bytes(c.clientCtx.TxConfig.TxEncoder()(signedTx))
}

// BroadcastResult contains results of transaction broadcast
type BroadcastResult struct {
	TxHash    string
	GasUsed   int64
	EventLogs sdk.StringEvents
}

// Broadcast broadcasts encoded transaction and returns tx hash
func (c Client) Broadcast(ctx context.Context, encodedTx []byte) (BroadcastResult, error) {
	var txHash string
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	res, err := c.clientCtx.Client.BroadcastTxSync(requestCtx, encodedTx)
	// nolint:nestif // This code is still easy to understand
	if err != nil {
		if errors.Is(err, requestCtx.Err()) {
			return BroadcastResult{}, errors.WithStack(err)
		}

		errRes := client.CheckTendermintError(err, encodedTx)
		if !isTxInMempool(errRes) {
			return BroadcastResult{}, errors.WithStack(err)
		}
		txHash = errRes.TxHash
	} else {
		txHash = res.Hash.String()
		if res.Code != 0 {
			if err := checkSequence(res.Codespace, res.Code, res.Log); err != nil {
				return BroadcastResult{}, err
			}
			return BroadcastResult{}, errors.Errorf("node returned non-zero code for tx '%s' (code: %d, codespace: %s): %s",
				txHash, res.Code, res.Codespace, res.Log)
		}
	}

	txHashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return BroadcastResult{}, errors.WithStack(err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, txTimeout)
	defer cancel()

	var resultTx *coretypes.ResultTx
	err = retry.Do(timeoutCtx, txStatusPollInterval, func() error {
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		var err error
		resultTx, err = c.clientCtx.Client.Tx(requestCtx, txHashBytes, false)
		if err != nil {
			if errors.Is(err, requestCtx.Err()) {
				return retry.Retryable(errors.WithStack(err))
			}
			if errRes := client.CheckTendermintError(err, encodedTx); errRes != nil {
				if isTxInMempool(errRes) {
					return retry.Retryable(errors.WithStack(err))
				}
				return errors.WithStack(err)
			}
			return retry.Retryable(errors.WithStack(err))
		}
		if resultTx.TxResult.Code != 0 {
			res := resultTx.TxResult
			if err := checkSequence(res.Codespace, res.Code, res.Log); err != nil {
				return err
			}
			return errors.Errorf("node returned non-zero code for tx '%s' (code: %d, codespace: %s): %s",
				txHash, res.Code, res.Codespace, res.Log)
		}
		if resultTx.Height == 0 {
			return retry.Retryable(errors.Errorf("transaction '%s' hasn't been included in a block yet", txHash))
		}
		return nil
	})
	if err != nil {
		return BroadcastResult{}, err
	}
	return BroadcastResult{
		TxHash:    txHash,
		GasUsed:   resultTx.TxResult.GasUsed,
		EventLogs: sdk.StringifyEvents(resultTx.TxResult.Events),
	}, nil
}

// BaseInput holds input data common to every transaction
type BaseInput struct {
	Signer   Wallet
	GasLimit uint64
	GasPrice Coin
	Memo     string
}

// TxBankSendInput holds input data for PrepareTxBankSend
type TxBankSendInput struct {
	Sender   Wallet
	Receiver Wallet
	Amount   Coin

	Base BaseInput
}

// PrepareTxBankSend creates a transaction sending tokens from one wallet to another
func (c Client) PrepareTxBankSend(ctx context.Context, input TxBankSendInput) ([]byte, error) {
	fromAddress, err := sdk.AccAddressFromBech32(input.Sender.Key.Address())
	must.OK(err)
	toAddress, err := sdk.AccAddressFromBech32(input.Receiver.Key.Address())
	must.OK(err)

	if err := input.Amount.Validate(); err != nil {
		return nil, errors.Wrap(err, "amount to send is invalid")
	}

	signedTx, err := c.Sign(ctx, input.Base, banktypes.NewMsgSend(fromAddress, toAddress, sdk.Coins{
		{
			Denom:  input.Amount.Denom,
			Amount: sdk.NewIntFromBigInt(input.Amount.Amount),
		},
	}))
	if err != nil {
		return nil, err
	}

	return c.Encode(signedTx), nil
}

func isTxInMempool(errRes *sdk.TxResponse) bool {
	if errRes == nil {
		return false
	}
	return isSDKErrorResult(errRes.Codespace, errRes.Code, cosmoserrors.ErrTxInMempoolCache)
}

func isSDKErrorResult(codespace string, code uint32, sdkErr *cosmoserrors.Error) bool {
	return codespace == sdkErr.Codespace() &&
		code == sdkErr.ABCICode()
}

func signTx(clientCtx client.Context, input BaseInput, msgs ...sdk.Msg) (authsigning.Tx, error) {
	signer := input.Signer

	privKey := &cosmossecp256k1.PrivKey{Key: signer.Key}
	txBuilder := clientCtx.TxConfig.NewTxBuilder()
	must.OK(txBuilder.SetMsgs(msgs...))
	txBuilder.SetGasLimit(input.GasLimit)
	txBuilder.SetMemo(input.Memo)

	if input.GasPrice.Amount != nil {
		if err := input.GasPrice.Validate(); err != nil {
			return nil, errors.Wrap(err, "gas price is invalid")
		}

		gasLimit := sdk.NewInt(int64(input.GasLimit))
		gasPrice := sdk.NewIntFromBigInt(input.GasPrice.Amount)
		fee := sdk.NewCoin(input.GasPrice.Denom, gasLimit.Mul(gasPrice))
		txBuilder.SetFeeAmount(sdk.NewCoins(fee))
	}

	signerData := authsigning.SignerData{
		ChainID:       clientCtx.ChainID,
		AccountNumber: signer.AccountNumber,
		Sequence:      signer.AccountSequence,
	}
	sigData := &signing.SingleSignatureData{
		SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
		Signature: nil,
	}
	sig := signing.SignatureV2{
		PubKey:   privKey.PubKey(),
		Data:     sigData,
		Sequence: signer.AccountSequence,
	}
	must.OK(txBuilder.SetSignatures(sig))

	bytesToSign := must.Bytes(clientCtx.TxConfig.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_DIRECT, signerData, txBuilder.GetTx()))
	sigBytes, err := privKey.Sign(bytesToSign)
	must.OK(err)

	sigData.Signature = sigBytes

	must.OK(txBuilder.SetSignatures(sig))

	return txBuilder.GetTx(), nil
}

type sequenceError struct {
	expectedSequence uint64
	message          string
}

func (e sequenceError) Error() string {
	return e.message
}

func checkSequence(codespace string, code uint32, log string) error {
	// Cosmos SDK doesn't return expected sequence number as a parameter from RPC call,
	// so we must parse the error message in a hacky way.

	if !isSDKErrorResult(codespace, code, cosmoserrors.ErrWrongSequence) {
		return nil
	}
	matches := expectedSequenceRegExp.FindStringSubmatch(log)
	if len(matches) != 2 {
		return errors.Errorf("cosmos sdk hasn't returned expected sequence number, log mesage received: %s", log)
	}
	expectedSequence, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return errors.Wrapf(err, "can't parse expected sequence number, log mesage received: %s", log)
	}
	return errors.WithStack(sequenceError{message: log, expectedSequence: expectedSequence})
}

// FetchSequenceFromError checks if error is related to account sequence mismatch, and returns expected account sequence
func FetchSequenceFromError(err error) (uint64, bool) {
	var seqErr sequenceError
	if errors.As(err, &seqErr) {
		return seqErr.expectedSequence, true
	}
	return 0, false
}

// buildSimTx creates an unsigned tx with an empty single signature and returns
// the encoded transaction or an error if the unsigned transaction cannot be
// built.
func buildSimTx(
	clientCtx client.Context,
	base BaseInput,
	msgs ...sdk.Msg,
) ([]byte, error) {
	factory := new(tx.Factory).
		WithTxConfig(clientCtx.TxConfig).
		WithChainID(clientCtx.ChainID).
		WithGasPrices(base.GasPrice.String()).
		WithMemo(base.Memo).
		WithSignMode(signing.SignMode_SIGN_MODE_DIRECT)

	txb, err := factory.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, err
	}

	// TODO: once keyring is introduced in the client, better to get the pubkey from keyring,
	// not pass the private key around.
	var pubKey cryptotypes.PubKey = &cosmossecp256k1.PubKey{
		Key: base.Signer.Key.PubKey(),
	}

	// Create an empty signature literal as the ante handler will populate with a
	// sentinel pubkey.
	sig := signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode: factory.SignMode(),
		},

		Sequence: base.Signer.AccountSequence,
	}
	if err := txb.SetSignatures(sig); err != nil {
		return nil, err
	}

	return clientCtx.TxConfig.TxEncoder()(txb.GetTx())
}

// WASMQueryClient returns a WASM module querying client, initialized
// using the internal clientCtx.
func (c Client) WASMQueryClient() wasmtypes.QueryClient {
	return c.wasmQueryClient
}

// LogEventLogsInfo sends all events logs as Info to the logger.
func LogEventLogsInfo(l *zap.Logger, eventLogs sdk.StringEvents) {
	for _, ev := range eventLogs {
		fields := make([]zap.Field, 0, len(ev.Attributes))
		for _, attr := range ev.Attributes {
			fields = append(fields, zap.String(attr.Key, attr.Value))
		}

		l.With(fields...).Info(ev.Type)
	}
}
