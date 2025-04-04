package cored

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	cometbftcrypto "github.com/cometbft/cometbft/crypto"
	cbfted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v5/pkg/client"
	coreumconfig "github.com/CoreumFoundation/coreum/v5/pkg/config"
	coreumconstant "github.com/CoreumFoundation/coreum/v5/pkg/config/constant"
	assetft "github.com/CoreumFoundation/coreum/v5/x/asset/ft"
	assetfttypes "github.com/CoreumFoundation/coreum/v5/x/asset/ft/types"
	"github.com/CoreumFoundation/coreum/v5/x/dex"
	dextypes "github.com/CoreumFoundation/coreum/v5/x/dex/types"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/cosmoschain"
	"github.com/CoreumFoundation/crust/znet/infra/targets"
)

const (
	// AppType is the type of cored application.
	AppType infra.AppType = "cored"

	// DockerImageStandard uses standard docker image of cored.
	DockerImageStandard = "cored:znet"
)

var basicModuleList = []module.AppModuleBasic{
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	staking.AppModuleBasic{},
}

// Config stores cored app config.
type Config struct {
	Name              string
	HomeDir           string
	BinDir            string
	WrapperDir        string
	DockerImage       string
	GenesisInitConfig *GenesisInitConfig
	AppInfo           *infra.AppInfo
	Ports             Ports
	IsValidator       bool
	StakerMnemonic    string
	StakerBalance     int64
	FundingMnemonic   string
	FaucetMnemonic    string
	GasPriceStr       string
	ValidatorNodes    []Cored
	SeedNodes         []Cored
	ImportedMnemonics map[string]string
	BinaryVersion     string
	TimeoutCommit     time.Duration
	Upgrades          map[string]string
}

// GenesisDEXConfig is the dex config of the GenesisInitConfig.
//
//nolint:tagliatelle
type GenesisDEXConfig struct {
	MaxOrdersPerDenom uint64 `json:"max_orders_per_denom"`
}

// GenesisInitConfig is used to pass parameters for genesis creation to cored binary.
//
//nolint:tagliatelle
type GenesisInitConfig struct {
	ChainID            coreumconstant.ChainID `json:"chain_id"`
	Denom              string                 `json:"denom"`
	DisplayDenom       string                 `json:"display_denom"`
	AddressPrefix      string                 `json:"address_prefix"`
	GenesisTime        time.Time              `json:"genesis_time"`
	GovConfig          GovConfig              `json:"gov_config"`
	CustomParamsConfig CustomParamsConfig     `json:"custom_params_config"`
	BankBalances       []banktypes.Balance    `json:"bank_balances"`
	Validators         []GenesisValidator     `json:"validators"`
	DEXConfig          GenesisDEXConfig       `json:"dex_config"`
	GenTxs             []json.RawMessage      `json:"gen_txs"`
}

// GovConfig contains the gov config part of genesis.
//
//nolint:tagliatelle
type GovConfig struct {
	MinDeposit            sdk.Coins     `json:"min_deposit"`
	ExpeditedMinDeposit   sdk.Coins     `json:"expedited_min_deposit"`
	VotingPeriod          time.Duration `json:"voting_period"`
	ExpeditedVotingPeriod time.Duration `json:"expedited_voting_period"`
}

// CustomParamsConfig contains the custom params used to generate genesis.
//
//nolint:tagliatelle
type CustomParamsConfig struct {
	MinSelfDelegation sdkmath.Int `json:"min_self_delegation"`
}

// GenesisValidator defines the validator to be added to the genesis.
//
//nolint:tagliatelle
type GenesisValidator struct {
	DelegatorMnemonic string                `json:"delegator_mnemonic"`
	PubKey            cometbftcrypto.PubKey `json:"pub_key"`
	ValidatorName     string                `json:"validator_name"`
}

// New creates new cored app.
func New(cfg Config) Cored {
	nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)

	valPrivateKey := cbfted25519.GenPrivKey()
	if cfg.IsValidator {
		cfg.GenesisInitConfig.Validators = append(cfg.GenesisInitConfig.Validators, GenesisValidator{
			DelegatorMnemonic: cfg.StakerMnemonic,
			PubKey:            valPrivateKey.PubKey(),
			ValidatorName:     NodeID(nodePublicKey),
		})
	}

	return Cored{
		config:              cfg,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: valPrivateKey.Bytes(),
		mu:                  &sync.RWMutex{},
		importedMnemonics:   cfg.ImportedMnemonics,
	}
}

// Cored represents cored.
type Cored struct {
	config              Config
	nodeID              string
	nodePrivateKey      ed25519.PrivateKey
	validatorPrivateKey ed25519.PrivateKey

	mu                *sync.RWMutex
	importedMnemonics map[string]string
}

// Type returns type of application.
func (c Cored) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (c Cored) Name() string {
	return c.config.Name
}

// Info returns deployment info.
func (c Cored) Info() infra.DeploymentInfo {
	return c.config.AppInfo.Info()
}

// NodeID returns node ID.
func (c Cored) NodeID() string {
	return c.nodeID
}

// Config returns cored config.
func (c Cored) Config() Config {
	return c.config
}

// ClientContext creates new cored ClientContext.
func (c Cored) ClientContext() client.Context {
	rpcClient, err := cosmosclient.
		NewClientFromNode(infra.JoinNetAddr("http", c.Info().HostFromHost, c.Config().Ports.RPC))
	must.OK(err)

	grpcClient, err := cosmoschain.GRPCClient(infra.JoinNetAddr("", c.Info().HostFromHost, c.Config().Ports.GRPC))
	must.OK(err)

	return client.NewContext(client.DefaultContextConfig(), basicModuleList...).
		WithChainID(string(c.config.GenesisInitConfig.ChainID)).
		WithClient(rpcClient).
		WithGRPCClient(grpcClient)
}

// TxFactory returns factory with present values for the chain.
func (c Cored) TxFactory(clientCtx client.Context) tx.Factory {
	return tx.Factory{}.
		WithKeybase(clientCtx.Keyring()).
		WithChainID(string(c.config.GenesisInitConfig.ChainID)).
		WithTxConfig(clientCtx.TxConfig())
}

// HealthCheck checks if cored chain is ready to accept transactions.
func (c Cored) HealthCheck(ctx context.Context) error {
	return infra.CheckCosmosNodeHealth(ctx, c.ClientContext(), c.Info())
}

// Deployment returns deployment of cored.
//
//nolint:funlen
func (c Cored) Deployment() infra.Deployment {
	deployment := infra.Deployment{
		RunAsUser: true,
		Image:     c.config.DockerImage,
		Name:      c.Name(),
		Info:      c.config.AppInfo,
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "DAEMON_HOME",
					Value: filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID)),
				},
				{
					Name:  "DAEMON_NAME",
					Value: "cored",
				},
				{
					Name:  "GOCOVERDIR",
					Value: c.GoCoverDir(),
				},
			}
		},
		Volumes: []infra.Volume{
			{
				Source:      filepath.Join(c.config.HomeDir, "config"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID), "config"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "data"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID), "data"),
			},
			{
				Source: filepath.Join(c.config.HomeDir, "cosmovisor", "genesis"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID), "cosmovisor",
					"genesis"),
			},
			{
				Source: filepath.Join(c.config.HomeDir, "cosmovisor", "upgrades"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.GenesisInitConfig.ChainID), "cosmovisor",
					"upgrades"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, covdataDirName),
				Destination: c.GoCoverDir(),
			},
		},
		ArgsFunc: func() []string {
			args := []string{
				"start",
				"--home", targets.AppHomeDir,
				"--log_level", "info",
				"--trace",
				"--rpc.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.RPC),
				"--p2p.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.P2P),
				"--grpc.address", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.GRPC),
				"--rpc.pprof_laddr", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.PProf),
				"--inv-check-period", "1",
				"--chain-id", string(c.config.GenesisInitConfig.ChainID),
				"--minimum-gas-prices", "0.000000000000000001" + c.config.GenesisInitConfig.Denom,
				"--wasm.memory_cache_size", "100",
				"--wasm.query_gas_limit", "3000000",
			}
			if len(c.config.ValidatorNodes) > 0 {
				peers := make([]string, 0, len(c.config.ValidatorNodes))
				peerIDs := make([]string, 0, len(c.config.ValidatorNodes))

				for _, valNode := range c.config.ValidatorNodes {
					peers = append(peers,
						valNode.NodeID()+"@"+infra.JoinNetAddr("", valNode.Info().HostFromContainer, valNode.Config().Ports.P2P),
					)
					peerIDs = append(peerIDs, valNode.NodeID())
				}

				args = append(args,
					"--p2p.persistent_peers", strings.Join(peers, ","),
					"--p2p.private_peer_ids", strings.Join(peerIDs, ","),
				)
			}
			if len(c.config.SeedNodes) > 0 {
				seeds := make([]string, 0, len(c.config.SeedNodes))

				for _, seedNode := range c.config.SeedNodes {
					seeds = append(seeds,
						seedNode.NodeID()+"@"+infra.JoinNetAddr("", seedNode.Info().HostFromContainer, seedNode.Config().Ports.P2P),
					)
				}

				args = append(args,
					"--p2p.seeds", strings.Join(seeds, ","),
				)
			}

			return args
		},
		Ports:       infra.PortsToMap(c.config.Ports),
		PrepareFunc: c.prepare,
		ConfigureFunc: func(ctx context.Context, deployment infra.DeploymentInfo) error {
			return c.saveClientWrapper(c.config.WrapperDir, deployment.HostFromHost)
		},
	}

	if len(c.config.ValidatorNodes) > 0 || len(c.config.SeedNodes) > 0 {
		dependencies := make([]infra.HealthCheckCapable, 0, len(c.config.ValidatorNodes)+len(c.config.SeedNodes))
		for _, valNode := range c.config.ValidatorNodes {
			dependencies = append(dependencies, infra.IsRunning(valNode))
		}
		for _, seedNode := range c.config.SeedNodes {
			dependencies = append(dependencies, infra.IsRunning(seedNode))
		}

		deployment.Requires = infra.Prerequisites{
			Timeout:      60 * time.Second,
			Dependencies: dependencies,
		}
	}

	return deployment
}

func (c Cored) dockerBinaryPath() string {
	coredStandardBinName := "cored"
	coredBinName := coredStandardBinName
	coredStandardPath := filepath.Join(
		c.config.BinDir, ".cache", "cored", tools.TargetPlatformLinuxLocalArchInDocker.String(), "bin",
	)
	coredPath := coredStandardPath

	// by default the binary version is latest, but if `BinaryVersion` is provided we take it as initial
	if c.Config().BinaryVersion != "" {
		return filepath.Join(coredStandardPath, coredStandardBinName+"-"+c.Config().BinaryVersion)
	}
	return filepath.Join(coredPath, coredBinName)
}

func (c Cored) prepare(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	saveTendermintConfig(coreumconfig.NodeConfig{
		Name:           c.config.Name,
		PrometheusPort: c.config.Ports.Prometheus,
		NodeKey:        c.nodePrivateKey,
		ValidatorKey:   c.validatorPrivateKey,
	}, c.config.TimeoutCommit, c.config.HomeDir)

	if err := os.MkdirAll(filepath.Join(c.config.HomeDir, "data"), 0o700); err != nil {
		return errors.WithStack(err)
	}

	// We need to pre-create empty covdata dir. Otherwise, docker creates empty dir with root ownership and go fails to
	// create coverage files because of permissions.
	if err := os.MkdirAll(filepath.Join(c.config.HomeDir, covdataDirName), 0o700); err != nil {
		return errors.WithStack(err)
	}

	appCfg := srvconfig.DefaultConfig()
	appCfg.API.Enable = true
	appCfg.API.Swagger = true
	appCfg.API.EnableUnsafeCORS = true
	appCfg.API.Address = infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.API)
	appCfg.GRPC.Enable = true
	appCfg.GRPCWeb.Enable = true
	appCfg.Telemetry.Enabled = true
	appCfg.Telemetry.PrometheusRetentionTime = 600
	appCfg.Mempool.MaxTxs = 5000
	srvconfig.WriteConfigFile(filepath.Join(c.config.HomeDir, "config", "app.toml"), appCfg)

	if err := importMnemonicsToKeyring(c.config.HomeDir, c.importedMnemonics); err != nil {
		return err
	}

	if err := c.SaveGenesis(ctx, c.config.HomeDir); err != nil {
		return errors.WithStack(err)
	}

	if err := os.MkdirAll(filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin"), 0o700); err != nil {
		return errors.WithStack(err)
	}
	// the path is defined by the build
	if err := copyFile(
		c.dockerBinaryPath(),
		filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin", "cored"),
		0o755); err != nil {
		return err
	}

	// upgrade to binary mapping
	upgrades := c.Config().Upgrades
	dockerLinuxBinaryPath := filepath.Join(
		c.config.BinDir, ".cache", "cored", tools.TargetPlatformLinuxLocalArchInDocker.String(), "bin",
	)
	for upgrade, binary := range upgrades {
		err := copyFile(filepath.Join(dockerLinuxBinaryPath, binary),
			filepath.Join(c.config.HomeDir, "cosmovisor", "upgrades", upgrade, "bin", "cored"), 0o755)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c Cored) saveClientWrapper(wrapperDir, hostname string) error {
	clientWrapper := `#!/bin/bash
OPTS=""
if [ "$1" == "tx" ] || [ "$1" == "q" ] || [ "$1" == "query" ]; then
	OPTS="$OPTS --node ""` + infra.JoinNetAddr("tcp", hostname, c.config.Ports.RPC) + `"""
fi
if [ "$1" == "tx" ] || [ "$1" == "keys" ]; then
	OPTS="$OPTS --keyring-backend ""test"""
fi

exec "` +
		c.config.BinDir +
		`/cored" --chain-id "` +
		string(c.config.GenesisInitConfig.ChainID) +
		`" --home "` +
		filepath.Dir(c.config.HomeDir) +
		`" "$@" $OPTS
`
	return errors.WithStack(os.WriteFile(filepath.Join(wrapperDir, c.Name()), []byte(clientWrapper), 0o700))
}

// SaveGenesis saves json encoded representation of the genesis config into file.
func (c Cored) SaveGenesis(ctx context.Context, homeDir string) error {
	configDir := filepath.Join(homeDir, "config")

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return errors.Wrap(err, "unable to make config directory")
	}

	genesisFile := filepath.Join(configDir, "genesis.json")

	inputConfig, err := cmtjson.MarshalIndent(c.config.GenesisInitConfig, "", "  ")
	if err != nil {
		return err
	}

	inputPath := filepath.Join(configDir, "genesis-creation-input.json")

	if err := os.WriteFile(inputPath, inputConfig, 0644); err != nil {
		return err
	}

	fullArgs := []string{
		"generate-genesis",
		"--output-path", genesisFile,
		"--input-path", inputPath,
		"--chain-id", string(c.config.GenesisInitConfig.ChainID),
	}

	// get particular binary path from or run using the default(compiled) binary
	var binaryPath string
	if c.config.BinaryVersion != "" {
		binaryPath = filepath.Join(
			c.config.BinDir,
			".cache",
			"cored",
			tools.TargetPlatformLocal.String(), "bin",
			"cored"+"-"+c.Config().BinaryVersion,
		)
	} else {
		binaryPath = filepath.Join(c.config.BinDir, "cored")
	}

	return libexec.Exec(
		ctx,
		exec.Command(binaryPath, fullArgs...),
	)
}

// AddDEXGenesisConfig adds DEX related genesis config.
func AddDEXGenesisConfig(ctx context.Context, genesisConfig GenesisInitConfig) (GenesisInitConfig, error) {
	// issue an asset FT to place an order
	issuer := FundingAddress
	issuerMnemonic := FundingMnemonic
	ordersCount := 2_000
	issuerMsgs := make([]sdk.Msg, 0)

	genesisConfig.DEXConfig.MaxOrdersPerDenom = uint64(ordersCount) // allow to place all orders

	orderSeqQuantity := sdkmath.NewIntFromUint64(100_000)
	issueMsg := &assetfttypes.MsgIssue{
		Issuer:        issuer,
		Symbol:        "DEXSU",
		Subunit:       "dexsu",
		Precision:     8,
		InitialAmount: orderSeqQuantity.MulRaw(int64(ordersCount)),
	}
	issuerMsgs = append(issuerMsgs, issueMsg)

	denom := issueMsg.Subunit + "-" + issueMsg.Issuer
	for i := range ordersCount {
		issuerMsgs = append(issuerMsgs, &dextypes.MsgPlaceOrder{
			Sender:      issuer,
			Type:        dextypes.ORDER_TYPE_LIMIT,
			ID:          fmt.Sprintf("id-%d", i),
			BaseDenom:   denom,
			QuoteDenom:  genesisConfig.Denom,
			Price:       lo.ToPtr(dextypes.MustNewPriceFromString("1")),
			Quantity:    orderSeqQuantity,
			Side:        dextypes.SIDE_SELL,
			TimeInForce: dextypes.TIME_IN_FORCE_GTC,
		})
	}

	txData, err := signTxsWithMnemonic(ctx, string(genesisConfig.ChainID), issuerMnemonic, issuerMsgs...)
	if err != nil {
		return GenesisInitConfig{}, err
	}
	genesisConfig.GenTxs = append(genesisConfig.GenTxs, txData)

	return genesisConfig, nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	fr, err := os.Open(src)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fr.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return errors.WithStack(err)
	}

	fw, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fw.Close()

	if _, err = io.Copy(fw, fr); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func signTxsWithMnemonic(
	ctx context.Context,
	chainID string,
	mnemonic string,
	msgs ...sdk.Msg,
) ([]byte, error) {
	const signerKeyName = "signer"
	encodingConfig := coreumconfig.NewEncodingConfig(
		assetft.AppModuleBasic{},
		dex.AppModuleBasic{},
	)
	inMemKeyring := keyring.NewInMemory(encodingConfig.Codec)
	_, err := inMemKeyring.NewAccount(
		signerKeyName,
		mnemonic,
		"",
		hd.CreateHDPath(coreumconstant.CoinType, 0, 0).String(),
		hd.Secp256k1,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to import account with mnemonic")
	}
	txf := tx.Factory{}.
		WithChainID(chainID).
		WithKeybase(inMemKeyring).
		WithTxConfig(encodingConfig.TxConfig)
	txBuilder, err := txf.BuildUnsignedTx(msgs...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build MsgCreateValidator transaction")
	}
	if err := tx.Sign(ctx, txf, signerKeyName, txBuilder, true); err != nil {
		return nil, errors.Wrap(err, "failed to sign MsgCreateValidator transaction")
	}
	return encodingConfig.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
}
