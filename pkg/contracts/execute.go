package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

// ExecuteConfig contains contract execution arguments and options.
type ExecuteConfig struct {
	// Network holds the chain config of the network
	Network ChainConfig

	// From specifies credentials for signing the execution transactions.
	From cored.Wallet

	// ExecutePayload is a path to a file containing JSON-encoded contract exec args, or JSON-encoded body itself.
	ExecutePayload string

	// Amount specifies Coins to send to the contract during execution.
	Amount string

	amountParsed       sdk.Coins
	executePayloadBody json.RawMessage
}

type ExecuteOutput struct {
	ContractAddress string `json:"contractAddress"`
	MethodExecuted  string `json:"methodExecuted"`
	ExecuteTxHash   string `json:"execTxHash"`
}

// Execute implements logic for "contracts exec" CLI command.
func Execute(ctx context.Context, contractAddr string, config ExecuteConfig) (*ExecuteOutput, error) {
	log := logger.Get(ctx)

	if len(contractAddr) == 0 {
		err := errors.New("contract address cannot be empty")
		return nil, err
	} else if err := config.Validate(); err != nil {
		err = errors.Wrap(err, "failed to validate the execution config")
		return nil, err
	}

	out := &ExecuteOutput{
		ContractAddress: contractAddr,
	}
	log.Sugar().
		With(zap.String("from", config.From.AddressString())).
		Infof("Executing %s on chain", contractAddr)

	methodName, execTxHash, err := runContractExecution(
		ctx,
		config.Network,
		config.From,
		contractAddr,
		config.executePayloadBody,
		config.amountParsed,
	)
	if err != nil {
		err = errors.Wrap(err, "failed to run contract execution")
		return nil, err
	}

	out.MethodExecuted = methodName
	out.ExecuteTxHash = execTxHash

	return out, nil
}

func runContractExecution(
	ctx context.Context,
	network ChainConfig,
	from cored.Wallet,
	contractAddr string,
	execMsg json.RawMessage,
	amount sdk.Coins,
) (methodName, txHash string, err error) {
	log := logger.Get(ctx)
	chainClient := cored.NewClient(network.ChainID, network.RPCEndpoint)

	input := cored.BaseInput{
		Signer:   from,
		GasPrice: network.minGasPriceParsed,
	}

	msgExecuteContract := &wasmtypes.MsgExecuteContract{
		Sender:   from.AddressString(),
		Contract: contractAddr,
		Msg:      wasmtypes.RawContractMessage(execMsg),
		Funds:    amount,
	}

	gasLimit, err := chainClient.EstimateGas(ctx, input, msgExecuteContract)
	if err != nil {
		err = errors.Wrap(err, "failed to estimate gas for MsgExecuteContract")
		return "", "", err
	} else {
		log.Info("Estimated gas limit",
			zap.Int("contract_msg_size", len(execMsg)),
			zap.Uint64("gas_limit", gasLimit),
		)

		input.GasLimit = uint64(float64(gasLimit) * gasEstimationAdj)
	}

	signedTx, err := chainClient.Sign(ctx, input, msgExecuteContract)
	if err != nil {
		err = errors.Wrapf(err, "failed to sign transaction as %s", from.AddressString())
		return "", "", err
	}

	txBytes := chainClient.Encode(signedTx)
	txHash = fmt.Sprintf("%X", tmtypes.Tx(txBytes).Hash())
	res, err := chainClient.Broadcast(ctx, txBytes)
	if err != nil {
		err = errors.Wrapf(err, "failed to broadcast Tx %s", txHash)
		return "", txHash, err
	}

	if len(res.EventLogs) > 0 {
		cored.LogEventLogsInfo(log, res.EventLogs)
	}

	for _, ev := range res.EventLogs {
		if ev.Type == wasmtypes.WasmModuleEventType {
			if value, ok := attrFromEvent(ev, "method"); ok {
				methodName = value
				break
			}
		}
	}

	return methodName, txHash, nil
}

func (c *ExecuteConfig) Validate() error {
	if body := []byte(c.ExecutePayload); json.Valid(body) {
		c.executePayloadBody = json.RawMessage(body)
	} else {
		payloadFilePath := c.ExecutePayload

		body, err := ioutil.ReadFile(payloadFilePath)
		if err != nil {
			err = errors.Wrapf(err, "file specified for exec payload, but couldn't be read: %s", payloadFilePath)
			return err
		}

		if !json.Valid(body) {
			err = errors.Wrapf(err, "file specified for exec payload, but doesn't contain valid JSON: %s", payloadFilePath)
			return err
		}

		c.executePayloadBody = json.RawMessage(body)
	}

	if len(c.Amount) > 0 {
		amount, err := sdk.ParseCoinsNormalized(c.Amount)
		if err != nil {
			err = errors.Wrapf(err, "failed to parse exec transfer amount as sdk.Coins: %s", c.Amount)
			return err
		}

		c.amountParsed = amount
	}

	if len(c.Network.MinGasPrice) == 0 {
		c.Network.minGasPriceParsed = cored.Coin{
			Amount: big.NewInt(1500), // matches InitialGasPrice in cored
			Denom:  "core",
		}
	} else {
		coinValue, err := sdk.ParseCoinNormalized(c.Network.MinGasPrice)
		if err != nil {
			err = errors.Wrapf(err, "failed to parse min gas price coin spec as sdk.Coin: %s", c.Network.MinGasPrice)
			return err
		}

		c.Network.minGasPriceParsed = cored.Coin{
			Amount: coinValue.Amount.BigInt(),
			Denom:  coinValue.Denom,
		}
	}

	return nil
}
