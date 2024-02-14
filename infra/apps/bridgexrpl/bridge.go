//nolint:tagliatelle // json naming
package bridgexrpl

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	rippledata "github.com/rubblelabs/ripple/data"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum/v4/app"
	"github.com/CoreumFoundation/coreum/v4/pkg/client"
	"github.com/CoreumFoundation/coreum/v4/pkg/config"
	"github.com/CoreumFoundation/coreum/v4/pkg/config/constant"
	"github.com/CoreumFoundation/coreum/v4/testutil/event"
	assetfttypes "github.com/CoreumFoundation/coreum/v4/x/asset/ft/types"
	"github.com/CoreumFoundation/crust/infra"
	coreumhelper "github.com/CoreumFoundation/crust/infra/apps/bridgexrpl/coreum"
	xrplhelper "github.com/CoreumFoundation/crust/infra/apps/bridgexrpl/xrpl"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/xrpl"
	"github.com/CoreumFoundation/crust/infra/targets"
)

const (
	// AppType is the type of cored application.
	AppType infra.AppType = "bridgexrpl"

	coreumAdminBalance   = 10_000_000_000
	coreumRelayerBalance = 100_000_000
)

//go:embed relayer.tmpl.yaml
var configTmpl string

// Config stores cored app config.
type Config struct {
	Name         string
	HomeDir      string
	ContractPath string
	Mnemonics    Mnemonics
	Quorum       uint32
	AppInfo      *infra.AppInfo
	Ports        Ports
	Leader       *Bridge
	Cored        cored.Cored
	XRPL         xrpl.XRPL
}

// New creates new cored app.
func New(cfg Config) Bridge {
	return Bridge{
		config:       cfg,
		contractAddr: lo.ToPtr(""),
	}
}

// Bridge represents XRPL bridge.
type Bridge struct {
	config       Config
	contractAddr *string
}

// Type returns type of application.
func (b Bridge) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (b Bridge) Name() string {
	return b.config.Name
}

// Info returns deployment info.
func (b Bridge) Info() infra.DeploymentInfo {
	return b.config.AppInfo.Info()
}

// Config returns cored config.
func (b Bridge) Config() Config {
	return b.config
}

// ContractAddr returns address of the smart contract.
func (b Bridge) ContractAddr() string {
	return *b.contractAddr
}

// Deployment returns deployment of XRPL bridge.
func (b Bridge) Deployment() infra.Deployment {
	d := infra.Deployment{
		RunAsUser: true,
		Image:     "coreumbridge-xrpl-relayer:local",
		Name:      b.Name(),
		Info:      b.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      b.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		ArgsFunc: func() []string {
			return []string{
				"start",
				"--home", targets.AppHomeDir,
				"--keyring-backend", "test",
			}
		},
		Ports:       infra.PortsToMap(b.config.Ports),
		PrepareFunc: b.setupBridge,
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				b.config.XRPL,
				b.config.Cored,
			},
		},
	}

	if b.config.Leader != nil {
		d.Requires.Dependencies = append(d.Requires.Dependencies, infra.IsRunning(*b.config.Leader))
	}

	return d
}

func (b Bridge) setupBridge(ctx context.Context) error {
	//nolint:nestif // ifs are here to return errors
	if b.config.Leader == nil {
		xrplClient := xrplhelper.NewRPCClient(infra.JoinNetAddr("http", b.config.XRPL.Info().HostFromHost,
			b.config.XRPL.Config().RPCPort))

		if err := fundXRPLAccounts(ctx, xrplClient); err != nil {
			return err
		}

		if err := b.fundCoreumAccounts(ctx); err != nil {
			return err
		}

		if err := b.setupBridgeMultisigAccount(ctx, xrplClient); err != nil {
			return err
		}

		if err := b.deploySmartContract(ctx); err != nil {
			return err
		}
	} else {
		*b.contractAddr = b.config.Leader.ContractAddr()
	}

	if err := b.importKeys(); err != nil {
		return err
	}

	return b.saveConfig()
}

func (b Bridge) setupBridgeMultisigAccount(ctx context.Context, rpcClient *xrplhelper.RPCClient) error {
	fee, err := rippledata.NewNativeValue(10)
	if err != nil {
		return err
	}

	adminKey, err := xrplhelper.KeyFromMnemonic(XRPLAdminMnemonic)
	if err != nil {
		return err
	}

	adminAccount := xrplhelper.AccountFromKey(adminKey)

	accInfo, err := rpcClient.AccountInfo(ctx, adminAccount)
	if err != nil {
		return err
	}
	seq := *accInfo.AccountData.Sequence

	enableRipplingTx := &rippledata.AccountSet{
		SetFlag: lo.ToPtr(uint32(rippledata.TxDefaultRipple)),
		TxBase: rippledata.TxBase{
			TransactionType: rippledata.ACCOUNT_SET,
			Fee:             *fee,
			Account:         adminAccount,
			Sequence:        seq,
		},
	}

	if err := rippledata.Sign(enableRipplingTx, adminKey, lo.ToPtr[uint32](0)); err != nil {
		return errors.WithStack(err)
	}

	if err := rpcClient.SubmitAndAwaitSuccess(ctx, enableRipplingTx); err != nil {
		return err
	}

	signerEntries := make([]rippledata.SignerEntry, 0, len(RelayerMnemonics))
	for _, m := range RelayerMnemonics {
		acc, err := xrplhelper.AccountFromMnemonic(m.XRPL)
		if err != nil {
			return err
		}
		signerEntries = append(signerEntries, rippledata.SignerEntry{
			SignerEntry: rippledata.SignerEntryItem{
				Account:      &acc,
				SignerWeight: lo.ToPtr(uint16(1)),
			},
		})
	}

	setSignerListTx := &rippledata.SignerListSet{
		SignerQuorum:  b.config.Quorum,
		SignerEntries: signerEntries,
		TxBase: rippledata.TxBase{
			TransactionType: rippledata.SIGNER_LIST_SET,
			Fee:             *fee,
			Account:         adminAccount,
			Sequence:        seq + 1,
		},
	}

	if err := rippledata.Sign(setSignerListTx, adminKey, lo.ToPtr[uint32](0)); err != nil {
		return errors.WithStack(err)
	}

	if err := rpcClient.SubmitAndAwaitSuccess(ctx, setSignerListTx); err != nil {
		return err
	}

	disableMasterKeyTx := &rippledata.AccountSet{
		SetFlag: lo.ToPtr(uint32(rippledata.TxSetDisableMaster)),
		TxBase: rippledata.TxBase{
			TransactionType: rippledata.ACCOUNT_SET,
			Fee:             *fee,
			Account:         adminAccount,
			Sequence:        seq + 2,
		},
	}

	if err := rippledata.Sign(disableMasterKeyTx, adminKey, lo.ToPtr[uint32](0)); err != nil {
		return errors.WithStack(err)
	}

	return rpcClient.SubmitAndAwaitSuccess(ctx, disableMasterKeyTx)
}

//nolint:funlen
func (b Bridge) deploySmartContract(ctx context.Context) error {
	wasmCode, err := os.ReadFile(b.config.ContractPath)
	if err != nil {
		return err
	}

	clientCtx := b.config.Cored.ClientContext().
		WithBroadcastMode(flags.BroadcastSync).
		WithAwaitTx(true)
	clientCtx = clientCtx.WithKeyring(keyring.NewInMemory(clientCtx.Codec()))
	txf := b.config.Cored.TxFactory(clientCtx).
		WithSimulateAndExecute(true).
		WithGasAdjustment(1.5)

	adminKeyInfo, err := clientCtx.Keyring().NewAccount(
		uuid.New().String(),
		CoreumAdminMnemonic,
		"",
		hd.CreateHDPath(constant.CoinType, 0, 0).String(),
		hd.Secp256k1,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	adminAddr, err := adminKeyInfo.GetAddress()
	if err != nil {
		return errors.WithStack(err)
	}

	clientCtx = clientCtx.WithFromAddress(adminAddr)

	deployMsg := &wasmtypes.MsgStoreCode{
		Sender:       adminAddr.String(),
		WASMByteCode: wasmCode,
	}

	res, err := client.BroadcastTx(ctx, clientCtx, txf, deployMsg)
	if err != nil {
		return err
	}

	codeID, err := event.FindUint64EventAttribute(res.Events, wasmtypes.EventTypeStoreCode, wasmtypes.AttributeKeyCodeID)
	if err != nil {
		return err
	}

	trustSetLimitAmount, ok := sdk.NewIntFromString("100000000000000000000000000000000000")
	if !ok {
		return errors.New("converting string to sdk.Int failed")
	}

	xrplBridgeAddr, err := xrplhelper.AccountFromMnemonic(XRPLAdminMnemonic)
	if err != nil {
		return err
	}

	type relayer struct {
		CoreumAddress sdk.AccAddress `json:"coreum_address"`
		XRPLAddress   string         `json:"xrpl_address"`
		XRPLPubKey    string         `json:"xrpl_pub_key"`
	}

	relayers := make([]relayer, 0, len(RelayerMnemonics))
	for _, m := range RelayerMnemonics {
		coreumAcc, err := coreumhelper.AccountFromMnemonic(m.Coreum)
		if err != nil {
			return err
		}

		xrplPrivKey, err := xrplhelper.KeyFromMnemonic(m.XRPL)
		if err != nil {
			return err
		}

		xrplAcc, err := xrplhelper.AccountFromMnemonic(m.XRPL)
		if err != nil {
			return err
		}

		relayers = append(relayers, relayer{
			CoreumAddress: coreumAcc,
			XRPLAddress:   xrplAcc.String(),
			XRPLPubKey:    fmt.Sprintf("%X", xrplPrivKey.Public(lo.ToPtr[uint32](0))),
		})
	}

	assetftClient := assetfttypes.NewQueryClient(clientCtx)
	assetFtParamsRes, err := assetftClient.Params(ctx, &assetfttypes.QueryParamsRequest{})
	if err != nil {
		return errors.Wrap(err, "failed to get asset ft issue fee")
	}

	payload, err := json.Marshal(struct {
		Owner                       sdk.AccAddress `json:"owner"`
		Relayers                    []relayer      `json:"relayers"`
		EvidenceThreshold           uint32         `json:"evidence_threshold"`
		UsedTicketSequenceThreshold uint32         `json:"used_ticket_sequence_threshold"`
		TrustSetLimitAmount         sdkmath.Int    `json:"trust_set_limit_amount"`
		BridgeXRPLAddress           string         `json:"bridge_xrpl_address"`
		XRPLBaseFee                 uint32         `json:"xrpl_base_fee"`
	}{
		Owner:                       adminAddr,
		Relayers:                    relayers,
		EvidenceThreshold:           b.config.Quorum,
		UsedTicketSequenceThreshold: 150,
		TrustSetLimitAmount:         trustSetLimitAmount,
		BridgeXRPLAddress:           xrplBridgeAddr.String(),
		XRPLBaseFee:                 10,
	})
	if err != nil {
		return errors.Wrap(err, "failed to marshal instantiate payload")
	}

	instantiateMsg := &wasmtypes.MsgInstantiateContract{
		Sender: adminAddr.String(),
		Admin:  adminAddr.String(),
		CodeID: codeID,
		Label:  "bridgexrpl",
		Msg:    wasmtypes.RawContractMessage(payload),
		Funds:  sdk.NewCoins(assetFtParamsRes.Params.IssueFee),
	}

	res, err = client.BroadcastTx(ctx, clientCtx, txf, instantiateMsg)
	if err != nil {
		return err
	}

	contractAddr, err := event.FindStringEventAttribute(
		res.Events,
		wasmtypes.EventTypeInstantiate,
		wasmtypes.AttributeKeyContractAddr,
	)
	if err != nil {
		return err
	}

	logger.Get(ctx).Info("Contract address of the XRPL bridge", zap.String("contractAddress", contractAddr))

	*b.contractAddr = contractAddr

	return nil
}

func (b Bridge) importKeys() error {
	keyringDir := filepath.Join(b.config.HomeDir, "keyring")
	if err := addKeyToTestKeyring(keyringDir, "xrpl-relayer", "xrpl", xrplhelper.HDPath,
		b.config.Mnemonics.XRPL); err != nil {
		return err
	}

	return addKeyToTestKeyring(keyringDir, "coreum-relayer", "coreum",
		hd.CreateHDPath(constant.CoinType, 0, 0).String(),
		b.config.Mnemonics.Coreum,
	)
}

func (b Bridge) saveConfig() error {
	f, err := os.OpenFile(filepath.Join(b.config.HomeDir, "relayer.yaml"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o400)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	return errors.WithStack(template.Must(template.New("").Parse(configTmpl)).Execute(f, struct {
		XRPLRPCURL            string
		CoreumGRPCURL         string
		CoreumContractAddress string
	}{
		XRPLRPCURL: infra.JoinNetAddr("http", b.config.XRPL.Info().HostFromContainer,
			b.config.XRPL.Config().RPCPort),
		CoreumGRPCURL: infra.JoinNetAddr("http", b.config.Cored.Info().HostFromContainer,
			b.config.Cored.Config().Ports.GRPC),
		CoreumContractAddress: *b.contractAddr,
	}))
}

func fundXRPLAccounts(ctx context.Context, rpcClient *xrplhelper.RPCClient) error {
	const (
		fundingSeed          = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
		bridgeAdminBalance   = 100_000_000_000
		bridgeRelayerBalance = 100_000_000_000
	)

	accounts := map[string]int64{
		XRPLAdminMnemonic: bridgeAdminBalance,
	}
	for _, m := range RelayerMnemonics {
		accounts[m.XRPL] = bridgeRelayerBalance
	}

	fundingKey, err := xrplhelper.KeyFromSeed(fundingSeed)
	if err != nil {
		return err
	}
	sender := xrplhelper.AccountFromKey(fundingKey)

	fee, err := rippledata.NewNativeValue(10)
	if err != nil {
		return err
	}

	accInfo, err := rpcClient.AccountInfo(ctx, sender)
	if err != nil {
		return err
	}

	seq := *accInfo.AccountData.Sequence
	for mnemonic, balance := range accounts {
		recipient, err := xrplhelper.AccountFromMnemonic(mnemonic)
		if err != nil {
			return err
		}

		value, err := rippledata.NewNativeValue(balance)
		if err != nil {
			return errors.WithStack(err)
		}

		amount := rippledata.Amount{
			Value: value,
		}

		paymentTx := &rippledata.Payment{
			Destination: recipient,
			Amount:      amount,
			TxBase: rippledata.TxBase{
				TransactionType: rippledata.PAYMENT,
				Fee:             *fee,
				Account:         sender,
				Sequence:        seq,
			},
		}
		seq++

		if err := rippledata.Sign(paymentTx, fundingKey, lo.ToPtr[uint32](0)); err != nil {
			return errors.WithStack(err)
		}

		if err := rpcClient.SubmitAndAwaitSuccess(ctx, paymentTx); err != nil {
			return err
		}
	}

	return nil
}

func (b Bridge) fundCoreumAccounts(ctx context.Context) error {
	coredConfig := b.config.Cored.Config()

	clientCtx := b.config.Cored.ClientContext().
		WithBroadcastMode(flags.BroadcastSync).
		WithAwaitTx(true)
	clientCtx = clientCtx.WithKeyring(keyring.NewInMemory(clientCtx.Codec()))
	txf := b.config.Cored.TxFactory(clientCtx).
		WithSimulateAndExecute(true)

	// TODO: Once we have more services requiring funds we will have to create dedicated faucet (by sharing code from
	// integration tests) to avoid problems with parallel tx creation and conflicting sequence numbers.
	faucetKeyInfo, err := clientCtx.Keyring().NewAccount(
		uuid.New().String(),
		coredConfig.FaucetMnemonic,
		"",
		hd.CreateHDPath(constant.CoinType, 0, 0).String(),
		hd.Secp256k1,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	faucetAddr, err := faucetKeyInfo.GetAddress()
	if err != nil {
		return errors.WithStack(err)
	}

	clientCtx = clientCtx.WithFromAddress(faucetAddr)

	adminCoreumAccount, err := coreumhelper.AccountFromMnemonic(CoreumAdminMnemonic)
	if err != nil {
		return err
	}

	sendMsg := &banktypes.MsgMultiSend{
		Inputs: []banktypes.Input{
			{
				Address: faucetAddr.String(),
				Coins: sdk.NewCoins(sdk.NewCoin(coredConfig.GenesisInitConfig.Denom,
					sdk.NewInt(coreumAdminBalance).Add(
						sdk.NewInt(coreumRelayerBalance).MulRaw(int64(len(RelayerMnemonics))),
					),
				)),
			},
		},
		Outputs: []banktypes.Output{
			{
				Address: adminCoreumAccount.String(),
				Coins:   sdk.NewCoins(sdk.NewInt64Coin(coredConfig.GenesisInitConfig.Denom, coreumAdminBalance)),
			},
		},
	}
	for _, m := range RelayerMnemonics {
		relayerAccount, err := coreumhelper.AccountFromMnemonic(m.Coreum)
		if err != nil {
			return err
		}
		sendMsg.Outputs = append(sendMsg.Outputs, banktypes.Output{
			Address: relayerAccount.String(),
			Coins:   sdk.NewCoins(sdk.NewInt64Coin(coredConfig.GenesisInitConfig.Denom, coreumRelayerBalance)),
		})
	}

	_, err = client.BroadcastTx(ctx, clientCtx, txf, sendMsg)
	return err
}

func addKeyToTestKeyring(keyringDir, keyName, suffix, hdPath, mnemonic string) error {
	keyringDir += "-" + suffix
	encodingConfig := config.NewEncodingConfig(app.ModuleBasics)
	clientCtx := cosmosclient.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithKeyringDir(keyringDir)

	kr, err := cosmosclient.NewKeyringFromBackend(clientCtx, keyring.BackendTest)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = kr.NewAccount(
		keyName,
		mnemonic,
		"",
		hdPath,
		hd.Secp256k1,
	)
	return errors.WithStack(err)
}
