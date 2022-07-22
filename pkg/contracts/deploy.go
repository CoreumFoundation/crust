package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CosmWasm/wasmd/x/wasm/ioutils"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
)

const gasEstimationAdj = 1.5

// DeployConfig provides params for the deploying stage.
type DeployConfig struct {
	// ArtefactPath is a filesystem path to *.wasm artefact to deploy. The blob might be gzipped.
	// If not provided, will be guessed from WorkspaceDir. Make sure that either WorkspaceDir or ArtefactPath
	// is provied and exists.
	ArtefactPath string

	// WorkspaceDir is used to locate ArtefactPath if none is provided, also used to rebuild the artefact
	// if NeedsRebuild is true. Will be guessed from ArtefactPath if NeedsRebuild is true.
	WorkspaceDir string

	// NeedsRebuild option forces an optimized rebuild of the WASM artefact, even if it exists. Requires
	// WorkspaceDir to be present and valid.
	NeedsRebuild bool

	// CodeID allows to specify existing program code ID to skip the store stage. If CodeID has been provided
	// and NeedInstantiation if false, the deployment just checks the program for existence on the chain.
	CodeID string

	// InstantiationConfig sets params specific to contract instantiation. If the instantiation phase is
	// skipped, make sure to have correct access type setting for the code store.
	InstantiationConfig ContractInstanceConfig

	// Network holds the chain config of the network
	Network ChainConfig

	// From specifies credentials for signing deployement / instantiation transactions.
	From cored.Wallet
}

// ChainConfig encapsulates chain-specific parameters, used to communicate with daemon.
type ChainConfig struct {
	// ChainID used to sign transactions
	ChainID string
	// MinGasPrice sets the minimum gas price required to be paid to get the transaction
	// included in a block. The real gasPrice is a dynamic value, so this option sets its minimum.
	MinGasPrice string
	// RPCAddr is the Tendermint RPC endpoint for the chain client
	RPCEndpoint string

	minGasPriceParsed cored.Coin
}

// ContractInstanceConfig contains params specific to contract instantiation.
type ContractInstanceConfig struct {
	// NeedInstantiation enables 2nd stage (contract instantiation) to be executed after code has been stored on chain.
	NeedInstantiation bool
	// AccessType sets the permission flag, affecting who can instantiate this contract.
	AccessType AccessType
	// AccessAddress is respected when AccessTypeOnlyAddress is chosen as AccessType.
	AccessAddress string
	// NeedAdmin controls the option to set admin address explicitly. If false, there will be no admin.
	NeedAdmin bool
	// AdminAddress sets the address of an admin, optional. Used if `NeedAdmin` is true.
	AdminAddress string
	// InstantiatePayload contains JSON-encoded contract instantiate args.
	InstantiatePayload json.RawMessage
	// Amount specifies Coins to send to the contract during instantiation.
	Amount string
	// Label sets the human-readable label for the contract instance.
	Label string
}

// AccessType encodes possible values of the access flag
type AccessType int

const (
	// AccessTypeUnspecified placeholder for empty value
	AccessTypeUnspecified AccessType = 0
	// AccessTypeNobody forbidden
	AccessTypeNobody AccessType = 1
	// AccessTypeOnlyAddress restricted to an address
	AccessTypeOnlyAddress AccessType = 2
	// AccessTypeEverybody unrestricted
	AccessTypeEverybody AccessType = 3
)

// Deploy implements logic for "contracts deploy" CLI command.
func Deploy(ctx context.Context, config DeployConfig) error {
	log := logger.Get(ctx)

	if len(config.Network.MinGasPrice) == 0 {
		config.Network.minGasPriceParsed = cored.Coin{
			Amount: big.NewInt(1),
			Denom:  "core",
		}
	} else {
		coinValue, err := sdk.ParseDecCoin(config.Network.MinGasPrice)
		if err != nil {
			err = errors.Wrapf(err, "failed to parse min gas price coin spec as sdk.Coin: %s", config.Network.MinGasPrice)
			return err
		}

		config.Network.minGasPriceParsed = cored.Coin{
			Amount: coinValue.Amount.BigInt(),
			Denom:  coinValue.Denom,
		}
	}

	artefactBase := filepath.Base(config.ArtefactPath)
	contractName := strings.TrimSuffix(artefactBase, filepath.Ext(artefactBase))
	deployLog := log.With(zap.String("name", contractName))

	wasmData, err := ioutil.ReadFile(config.ArtefactPath)
	if err != nil {
		err = errors.Wrap(err, "failed to read artefact data from the fs")
		return err
	}

	if ioutils.IsWasm(wasmData) {
		wasmData, err = ioutils.GzipIt(wasmData)

		if err != nil {
			err = errors.Wrap(err, "failed to gzip the wasm data")
			return err
		}
	} else if !ioutils.IsGzip(wasmData) {
		return errors.New("invalid input file. Use wasm binary or gzip")
	}

	if len(config.CodeID) == 0 {
		deployLog.Sugar().
			With(zap.String("from", config.From.AddressString())).
			Infof("Deploying %s on chain", artefactBase)

		codeID, err := runContractStore(ctx, config.Network, config.From, wasmData)
		if err != nil {
			err = errors.Wrap(err, "failed to run contract code store")
			return err
		}

		config.CodeID = codeID
	}

	return nil
}

func runContractStore(
	ctx context.Context,
	network ChainConfig,
	from cored.Wallet,
	wasmData []byte,
) (codeID string, err error) {
	chainClient := cored.NewClient(network.ChainID, network.RPCEndpoint)

	input := cored.BaseInput{
		Signer:   from,
		GasPrice: network.minGasPriceParsed,
	}

	msgStoreCode := &wasmtypes.MsgStoreCode{
		Sender:       from.AddressString(),
		WASMByteCode: wasmData,
		// InstantiatePermission: 0,
	}

	gasLimit, err := chainClient.EstimateGas(ctx, input, msgStoreCode)
	if err != nil {
		err = errors.Wrap(err, "failed to estimate gas for MsgXXX")
		return "", err
	} else {
		log := logger.Get(ctx)
		log.Info("Estimated gas limit",
			zap.Int("bytecode_size", len(wasmData)),
			zap.Uint64("gas_limit", gasLimit),
		)

		input.GasLimit = uint64(float64(gasLimit) * gasEstimationAdj)
	}

	signedTx, err := chainClient.Sign(ctx, input, msgStoreCode)
	if err != nil {
		err = errors.Wrapf(err, "failed to sign transaction as %s", from.AddressString())
		return "", err
	}

	txBytes := chainClient.Encode(signedTx)
	res, err := chainClient.Broadcast(ctx, txBytes)
	if err != nil {
		err = errors.Wrapf(err, "failed to broadcast Tx %X", tmtypes.Tx(txBytes).Hash())
		return "", err
	}

	for _, ev := range res.EventLogs {
		fmt.Println("[LOG] event", ev.Type, "attrs:", ev.Attributes)
	}

	return "", nil
}
