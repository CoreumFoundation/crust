package contracts

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
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
	// if NeedRebuild is true. Will be guessed from ArtefactPath if NeedRebuild is true.
	WorkspaceDir string

	// NeedRebuild option forces an optimized rebuild of the WASM artefact, even if it exists. Requires
	// WorkspaceDir to be present and valid.
	NeedRebuild bool

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
	AccessType string
	// AccessAddress is respected when AccessTypeOnlyAddress is chosen as AccessType.
	AccessAddress string
	// NeedAdmin controls the option to set admin address explicitly. If false, there will be no admin.
	NeedAdmin bool
	// AdminAddress sets the address of an admin, optional. Used if `NeedAdmin` is true.
	AdminAddress string
	// InstantiatePayload is a path to a file containing JSON-encoded contract instantiate args, or JSON-encoded body itself.
	InstantiatePayload string
	// Amount specifies Coins to send to the contract during instantiation.
	Amount string
	// Label sets the human-readable label for the contract instance.
	Label string

	instantiatePayloadBody json.RawMessage
	accessTypeParsed       wasmtypes.AccessType
	accessAddressParsed    sdk.AccAddress
	adminAddressParsed     sdk.AccAddress
	amountParsed           sdk.Coin
}

// AccessType encodes possible values of the access type flag
type AccessType string

const (
	// AccessTypeUnspecified placeholder for empty value
	AccessTypeUnspecified AccessType = "undefined"
	// AccessTypeNobody forbidden
	AccessTypeNobody AccessType = "nobody"
	// AccessTypeOnlyAddress restricted to an address
	AccessTypeOnlyAddress AccessType = "address"
	// AccessTypeEverybody unrestricted
	AccessTypeEverybody AccessType = "unrestricted"
)

// Deploy implements logic for "contracts deploy" CLI command.
func Deploy(ctx context.Context, config DeployConfig) (*DeployOutput, error) {
	log := logger.Get(ctx)

	if err := config.Validate(); err != nil {
		err = errors.Wrap(err, "failed to validate the deployment config")
		return nil, err
	}

	if len(config.ArtefactPath) == 0 {
		if err := ensureRustToolchain(ctx); err != nil {
			err = errors.Wrap(err, "problem with checking the Rust toolchain")
			return nil, err
		}

		crateName, err := readCrateMetadata(ctx, config.WorkspaceDir)
		if err != nil {
			err = errors.Wrap(err, "problem with ensuring the target crate workspace")
			return nil, err
		}

		config.ArtefactPath = filepath.Join(
			config.WorkspaceDir, "artifacts",
			fmt.Sprintf("%s.wasm", crateNameToArtefactName(crateName)),
		)
	}

	if _, err := os.Stat(config.ArtefactPath); err != nil {
		log.With(
			zap.String("artefactPath", config.ArtefactPath),
		).Info("WASM artefact is missing at path, triggering a rebuild")

		config.NeedRebuild = true
	} else if !checkWasmFile(config.ArtefactPath) {
		log.With(
			zap.String("artefactPath", config.ArtefactPath),
		).Info("WASM artefact is not valid at path, triggering a rebuild")

		config.NeedRebuild = true
	}

	if config.NeedRebuild {
		if len(config.WorkspaceDir) == 0 {
			// making the best guess, considering artefact is in ./artifacts
			config.WorkspaceDir = filepath.Base(filepath.Dir(config.ArtefactPath))

			log.With(
				zap.String("artefactPath", config.ArtefactPath),
				zap.String("workspaceDir", config.WorkspaceDir),
			).Info("Guessed workspace dir out of the artefact path")
		}

		artefactPath, err := Build(ctx, config.WorkspaceDir, BuildConfig{
			NeedOptimizedBuild: true,
		})
		if err != nil {
			err = errors.Wrap(err, "failed to run a release optimized build")
			return nil, err
		}

		config.ArtefactPath = artefactPath
	}

	artefactBase := filepath.Base(config.ArtefactPath)
	contractName := strings.TrimSuffix(artefactBase, filepath.Ext(artefactBase))
	deployLog := log.With(zap.String("name", contractName))

	wasmData, err := ioutil.ReadFile(config.ArtefactPath)
	if err != nil {
		err = errors.Wrap(err, "failed to read artefact data from the fs")
		return nil, err
	}

	if ioutils.IsWasm(wasmData) {
		wasmData, err = ioutils.GzipIt(wasmData)

		if err != nil {
			err = errors.Wrap(err, "failed to gzip the wasm data")
			return nil, err
		}
	} else if !ioutils.IsGzip(wasmData) {
		err := errors.New("invalid input file. Use wasm binary or gzip")
		return nil, err
	}

	out := &DeployOutput{
		CodeID: config.CodeID,
	}
	if len(config.CodeID) == 0 {
		deployLog.Sugar().
			With(zap.String("from", config.From.AddressString())).
			Infof("Deploying %s on chain", artefactBase)

		var accessConfig *wasmtypes.AccessConfig
		if config.InstantiationConfig.accessTypeParsed != wasmtypes.AccessTypeUnspecified {
			accessConfig = &wasmtypes.AccessConfig{
				Permission: config.InstantiationConfig.accessTypeParsed,
				Address:    config.InstantiationConfig.accessAddressParsed.String(),
			}
		}

		codeID, storeTxHash, err := runContractStore(ctx, config.Network, config.From, wasmData, accessConfig)
		if err != nil {
			err = errors.Wrap(err, "failed to run contract code store")
			return nil, err
		}

		config.CodeID = codeID
		out.CodeID = codeID
		out.StoreTxHash = storeTxHash
		out.Creator = config.From.AddressString()
		out.CodeDataHash = hashContractCode(wasmData)
	} else {
		info, err := queryContractCodeInfo(ctx, config.Network, config.CodeID)
		if err != nil {
			err = errors.Wrap(err, "failed to check contract code on chain")
			return nil, err
		}

		out.Creator = info.Creator
		out.CodeDataHash = info.CodeDataHash
	}

	if !config.InstantiationConfig.NeedInstantiation {
		// code ID is known (stored) and 2nd stage is not needed
		return out, nil
	}

	// TODO: instantiate there

	return out, nil
}

type DeployOutput struct {
	Creator      string `json:"creator"`
	CodeID       string `json:"codeID"`
	CodeDataHash string `json:"codeDataHash"`
	StoreTxHash  string `json:"storeTxHash,omitempty"`
	InitTxHash   string `json:"initTxHash,omitempty"`
}

func (c *DeployConfig) Validate() error {
	if len(c.ArtefactPath) == 0 && len(c.WorkspaceDir) == 0 {
		err := errors.New("both ArtefactPath and WorkspaceDir are empty in the deployment config, either is required")
		return err
	}

	if len(c.InstantiationConfig.InstantiatePayload) > 0 {
		if body := []byte(c.InstantiationConfig.InstantiatePayload); json.Valid(body) {
			c.InstantiationConfig.instantiatePayloadBody = json.RawMessage(body)
		} else {
			payloadFilePath := c.InstantiationConfig.InstantiatePayload

			body, err := ioutil.ReadFile(payloadFilePath)
			if err != nil {
				err = errors.Wrapf(err, "file specified for instantiate payload, but couldn't be read: %s", payloadFilePath)
				return err
			}

			if !json.Valid(body) {
				err = errors.Wrapf(err, "file specified for instantiate payload, but doesn't contain valid JSON: %s", payloadFilePath)
				return err
			}

			c.InstantiationConfig.instantiatePayloadBody = json.RawMessage(body)
		}
	}

	if len(c.InstantiationConfig.Amount) > 0 {
		amount, err := sdk.ParseCoinNormalized(c.InstantiationConfig.Amount)
		if err != nil {
			err = errors.Wrapf(err, "failed to parse instantiation transfer amount as sdk.Coin: %s", c.InstantiationConfig.Amount)
			return err
		}

		c.InstantiationConfig.amountParsed = amount
	}

	switch AccessType(c.InstantiationConfig.AccessType) {
	case AccessType(""):
		c.InstantiationConfig.accessTypeParsed = wasmtypes.AccessTypeUnspecified
	case AccessTypeUnspecified:
		c.InstantiationConfig.accessTypeParsed = wasmtypes.AccessTypeUnspecified
	case AccessTypeNobody:
		c.InstantiationConfig.accessTypeParsed = wasmtypes.AccessTypeNobody
	case AccessTypeEverybody:
		c.InstantiationConfig.accessTypeParsed = wasmtypes.AccessTypeEverybody
	case AccessTypeOnlyAddress:
		addr, err := sdk.AccAddressFromBech32(c.InstantiationConfig.AccessAddress)
		if err != nil {
			err = errors.Wrapf(err, "failed to parse instantiation access address from bech32: %s", c.InstantiationConfig.AccessAddress)
			return err
		}

		c.InstantiationConfig.accessAddressParsed = addr
	}

	if c.InstantiationConfig.NeedAdmin {
		if len(c.InstantiationConfig.AdminAddress) > 0 {
			addr, err := sdk.AccAddressFromBech32(c.InstantiationConfig.AdminAddress)
			if err != nil {
				err = errors.Wrapf(err, "failed to parse admin address from bech32: %s", c.InstantiationConfig.AdminAddress)
				return err
			}

			c.InstantiationConfig.adminAddressParsed = addr
		} else {
			c.InstantiationConfig.adminAddressParsed = c.From.Address()
		}
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

func runContractStore(
	ctx context.Context,
	network ChainConfig,
	from cored.Wallet,
	wasmData []byte,
	accessConfig *wasmtypes.AccessConfig,
) (codeID string, txHash string, err error) {
	log := logger.Get(ctx)
	chainClient := cored.NewClient(network.ChainID, network.RPCEndpoint)

	input := cored.BaseInput{
		Signer:   from,
		GasPrice: network.minGasPriceParsed,
	}

	msgStoreCode := &wasmtypes.MsgStoreCode{
		Sender:                from.AddressString(),
		WASMByteCode:          wasmData,
		InstantiatePermission: accessConfig,
	}

	gasLimit, err := chainClient.EstimateGas(ctx, input, msgStoreCode)
	if err != nil {
		err = errors.Wrap(err, "failed to estimate gas for MsgStoreCode")
		return "", "", err
	} else {
		log.Info("Estimated gas limit",
			zap.Int("bytecode_size", len(wasmData)),
			zap.Uint64("gas_limit", gasLimit),
		)

		input.GasLimit = uint64(float64(gasLimit) * gasEstimationAdj)
	}

	signedTx, err := chainClient.Sign(ctx, input, msgStoreCode)
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

	for _, ev := range res.EventLogs {
		if ev.Type == wasmtypes.EventTypeStoreCode {
			if value, ok := attrFromEvent(ev, wasmtypes.AttributeKeyCodeID); ok {
				codeID = value
				break
			}

			log.With(
				zap.String("txHash", txHash),
			).Warn("contract code stored MsgStoreCode, but events don't have codeID")
		}
	}

	return codeID, txHash, nil
}

type ContractCodeInfo struct {
	CodeID       uint64
	Creator      string
	CodeDataHash string
}

func queryContractCodeInfo(
	ctx context.Context,
	network ChainConfig,
	codeID string,
) (info *ContractCodeInfo, err error) {
	chainClient := cored.NewClient(network.ChainID, network.RPCEndpoint)

	codeInt, err := strconv.ParseUint(codeID, 10, 64)
	if err != nil {
		err = errors.Wrap(err, "failed to parse contract code ID as integer")
		return nil, err
	}

	resp, err := chainClient.WASMQueryClient().Code(ctx, &wasmtypes.QueryCodeRequest{
		CodeId: codeInt,
	})
	if err != nil {
		// FIXME: proper error unwrapping (module > sdk > rpc > rpc client)
		if strings.Contains(err.Error(), "code = InvalidArgument desc = not found") {
			err = errors.Errorf("contract codeID=%d not found on chain %s", codeInt, network.ChainID)
			return nil, err
		}

		err = errors.Wrap(err, "WASMQueryClient failed to query the chain")
		return nil, err
	}

	info = &ContractCodeInfo{
		CodeID:       resp.CodeID,
		Creator:      resp.Creator,
		CodeDataHash: resp.DataHash.String(),
	}
	return info, nil
}

func attrFromEvent(ev sdk.StringEvent, attr string) (value string, ok bool) {
	for _, attrItem := range ev.Attributes {
		if attrItem.Key == attr {
			value = attrItem.Value
			ok = true
			return
		}
	}

	return
}

func checkWasmFile(path string) bool {
	wasmData, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}

	return ioutils.IsWasm(wasmData) || ioutils.IsGzip(wasmData)
}

func hashContractCode(wasmData []byte) string {
	_ = wasmData
	return hex.EncodeToString([]byte("not implemented"))
}
