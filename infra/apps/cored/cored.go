package cored

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmosed25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v4/app"
	"github.com/CoreumFoundation/coreum/v4/pkg/client"
	"github.com/CoreumFoundation/coreum/v4/pkg/config"
	"github.com/CoreumFoundation/coreum/v4/pkg/config/constant"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
	"github.com/CoreumFoundation/crust/infra/targets"
)

// AppType is the type of cored application.
const AppType infra.AppType = "cored"

// Config stores cored app config.
type Config struct {
	Name              string
	HomeDir           string
	BinDir            string
	WrapperDir        string
	NetworkConfig     *config.NetworkConfig
	AppInfo           *infra.AppInfo
	Ports             Ports
	IsValidator       bool
	StakerMnemonic    string
	StakerBalance     int64
	FundingMnemonic   string
	FaucetMnemonic    string
	GasPriceStr       string
	RootNode          *Cored
	ImportedMnemonics map[string]string
	BinaryVersion     string
	TimeoutCommit     time.Duration
}

// New creates new cored app.
func New(cfg Config) Cored {
	nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)

	var valPrivateKey ed25519.PrivateKey
	if cfg.IsValidator {
		var (
			err          error
			valPublicKey ed25519.PublicKey
		)
		valPublicKey, valPrivateKey, err = ed25519.GenerateKey(rand.Reader)
		must.OK(err)

		stakerPrivKey, err := PrivateKeyFromMnemonic(cfg.StakerMnemonic)
		must.OK(err)

		minimumSelfDelegation := sdk.NewInt64Coin(cfg.NetworkConfig.Denom(), 20_000_000_000) // 20k core

		clientCtx := client.NewContext(client.DefaultContextConfig(), newBasicManager()).WithChainID(string(cfg.NetworkConfig.ChainID()))

		// leave 10% for slashing and commission
		stake := sdk.NewInt64Coin(cfg.NetworkConfig.Denom(), int64(float64(cfg.StakerBalance)*0.9))

		createValidatorTx, err := prepareTxStakingCreateValidator(
			cfg.NetworkConfig.ChainID(),
			clientCtx.TxConfig(),
			valPublicKey,
			stakerPrivKey,
			stake,
			minimumSelfDelegation.Amount,
		)
		must.OK(err)
		networkProvider := cfg.NetworkConfig.Provider.(config.DynamicConfigProvider)
		cfg.NetworkConfig.Provider = networkProvider.WithGenesisTx(createValidatorTx)
	}

	return Cored{
		config:              cfg,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: valPrivateKey,
		mu:                  &sync.RWMutex{},
		importedMnemonics:   cfg.ImportedMnemonics,
	}
}

// prepareTxStakingCreateValidator generates transaction of type MsgCreateValidator.
func prepareTxStakingCreateValidator(
	chainID constant.ChainID,
	txConfig cosmosclient.TxConfig,
	validatorPublicKey ed25519.PublicKey,
	stakerPrivateKey cosmossecp256k1.PrivKey,
	stakedBalance sdk.Coin,
	selfDelegation sdkmath.Int,
) ([]byte, error) {
	// the passphrase here is the trick to import the private key into the keyring
	const passphrase = "tmp"

	commission := stakingtypes.CommissionRates{
		Rate:          sdk.MustNewDecFromStr("0.1"),
		MaxRate:       sdk.MustNewDecFromStr("0.2"),
		MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
	}

	stakerAddress := sdk.AccAddress(stakerPrivateKey.PubKey().Address())
	msg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(stakerAddress),
		&cosmosed25519.PubKey{Key: validatorPublicKey},
		stakedBalance,
		stakingtypes.Description{Moniker: stakerAddress.String()},
		commission,
		selfDelegation,
	)
	if err != nil {
		return nil, errors.Wrap(err, "not able to make CreateValidatorMessage")
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "not able to validate CreateValidatorMessage")
	}

	inMemKeyring := keyring.NewInMemory(config.NewEncodingConfig(app.ModuleBasics).Codec)

	armor := crypto.EncryptArmorPrivKey(&stakerPrivateKey, passphrase, string(hd.Secp256k1Type))
	if err := inMemKeyring.ImportPrivKey(stakerAddress.String(), armor, passphrase); err != nil {
		return nil, errors.Wrap(err, "not able to import private key into new in memory keyring")
	}

	txf := tx.Factory{}.
		WithChainID(string(chainID)).
		WithKeybase(inMemKeyring).
		WithTxConfig(txConfig)

	txBuilder, err := txf.BuildUnsignedTx(msg)
	if err != nil {
		return nil, err
	}

	if err := tx.Sign(txf, stakerAddress.String(), txBuilder, true); err != nil {
		return nil, err
	}

	txBytes, err := txConfig.TxJSONEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, err
	}

	return txBytes, nil
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
	rpcClient, err := cosmosclient.NewClientFromNode(infra.JoinNetAddr("http", c.Info().HostFromHost, c.Config().Ports.RPC))
	must.OK(err)

	mm := newBasicManager()
	grpcClient, err := cosmoschain.GRPCClient(infra.JoinNetAddr("", c.Info().HostFromHost, c.Config().Ports.GRPC), mm)
	must.OK(err)

	return client.NewContext(client.DefaultContextConfig(), mm).
		WithChainID(string(c.config.NetworkConfig.ChainID())).
		WithRPCClient(rpcClient).
		WithGRPCClient(grpcClient)
}

// TxFactory returns factory with present values for the chain.
func (c Cored) TxFactory(clientCtx client.Context) tx.Factory {
	return tx.Factory{}.
		WithKeybase(clientCtx.Keyring()).
		WithChainID(string(c.config.NetworkConfig.ChainID())).
		WithTxConfig(clientCtx.TxConfig())
}

// HealthCheck checks if cored chain is ready to accept transactions.
func (c Cored) HealthCheck(ctx context.Context) error {
	return infra.CheckCosmosNodeHealth(ctx, c.ClientContext(), c.Info())
}

// Deployment returns deployment of cored.
func (c Cored) Deployment() infra.Deployment {
	deployment := infra.Deployment{
		RunAsUser: true,
		Image:     "cored:znet",
		Name:      c.Name(),
		Info:      c.config.AppInfo,
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "DAEMON_HOME",
					Value: filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID())),
				},
				{
					Name:  "DAEMON_NAME",
					Value: "cored",
				},
			}
		},
		Volumes: []infra.Volume{
			{
				Source:      filepath.Join(c.config.HomeDir, "config"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID()), "config"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "data"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID()), "data"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "cosmovisor", "genesis"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID()), "cosmovisor", "genesis"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "cosmovisor", "upgrades"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.NetworkConfig.ChainID()), "cosmovisor", "upgrades"),
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
				"--grpc-web.address", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.GRPCWeb),
				"--rpc.pprof_laddr", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.PProf),
				"--inv-check-period", "1",
				"--chain-id", string(c.config.NetworkConfig.ChainID()),
				"--minimum-gas-prices", fmt.Sprintf("0.000000000000000001%s", c.config.NetworkConfig.Denom()),
			}
			if c.config.RootNode != nil {
				args = append(args,
					"--p2p.seeds", c.config.RootNode.NodeID()+"@"+infra.JoinNetAddr("", c.config.RootNode.Info().HostFromContainer, c.config.RootNode.Config().Ports.P2P),
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
	if c.config.RootNode != nil {
		deployment.Requires = infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				infra.IsRunning(*c.config.RootNode),
			},
		}
	}
	return deployment
}

func (c Cored) prepare() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	saveTendermintConfig(config.NodeConfig{
		Name:           c.config.Name,
		PrometheusPort: c.config.Ports.Prometheus,
		NodeKey:        c.nodePrivateKey,
		ValidatorKey:   c.validatorPrivateKey,
	}, c.config.TimeoutCommit, c.config.HomeDir)

	if err := os.MkdirAll(filepath.Join(c.config.HomeDir, "data"), 0o700); err != nil {
		return errors.WithStack(err)
	}

	appCfg := srvconfig.DefaultConfig()
	appCfg.API.Enable = true
	appCfg.API.Swagger = true
	appCfg.API.EnableUnsafeCORS = true
	appCfg.API.Address = infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.API)
	appCfg.GRPC.Enable = true
	appCfg.GRPCWeb.Enable = true
	appCfg.GRPCWeb.EnableUnsafeCORS = true
	appCfg.Telemetry.Enabled = true
	appCfg.Telemetry.PrometheusRetentionTime = 600
	srvconfig.WriteConfigFile(filepath.Join(c.config.HomeDir, "config", "app.toml"), appCfg)

	if err := importMnemonicsToKeyring(c.config.HomeDir, c.importedMnemonics); err != nil {
		return err
	}

	if err := c.SaveGenesis(c.config.HomeDir); err != nil {
		return errors.WithStack(err)
	}

	if err := os.MkdirAll(filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin"), 0o700); err != nil {
		return errors.WithStack(err)
	}
	// the path is defined by the build
	dockerLinuxBinaryPath := filepath.Join(c.config.BinDir, ".cache", "cored", "docker.linux."+runtime.GOARCH, "bin")
	// by default the binary version is latest, but if `BinaryVersion` is provided we take it as initial
	binaryPath := filepath.Join(dockerLinuxBinaryPath, "cored")
	if c.Config().BinaryVersion != "" {
		binaryPath += "-" + c.Config().BinaryVersion
	}
	if err := copyFile(binaryPath, filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin", "cored"), 0o755); err != nil {
		return err
	}

	// upgrade to binary mapping
	upgrades := map[string]string{
		"v4":       "cored",
		"v3":       "cored-v3.0.0",
		"v3patch1": "cored",
	}
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
	client := `#!/bin/bash
OPTS=""
if [ "$1" == "tx" ] || [ "$1" == "q" ] || [ "$1" == "query" ]; then
	OPTS="$OPTS --node ""` + infra.JoinNetAddr("tcp", hostname, c.config.Ports.RPC) + `"""
fi
if [ "$1" == "tx" ] || [ "$1" == "keys" ]; then
	OPTS="$OPTS --keyring-backend ""test"""
fi

exec "` + c.config.BinDir + `/cored" --chain-id "` + string(c.config.NetworkConfig.ChainID()) + `" --home "` + filepath.Dir(c.config.HomeDir) + `" "$@" $OPTS
`
	return errors.WithStack(os.WriteFile(filepath.Join(wrapperDir, c.Name()), []byte(client), 0o700))
}

func newBasicManager() module.BasicManager {
	return module.NewBasicManager(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
	)
}

// SaveGenesis saves json encoded representation of the genesis config into file.
func (c Cored) SaveGenesis(homeDir string) error {
	genDocBytes, err := c.config.NetworkConfig.EncodeGenesis()
	if err != nil {
		return err
	}

	if strings.HasPrefix(c.config.BinaryVersion, "v1") || strings.HasPrefix(c.config.BinaryVersion, "v2") {
		genDocBytes = applyPatchToV3Genesis(genDocBytes)
	}

	if err := os.MkdirAll(homeDir+"/config", 0o700); err != nil {
		return errors.Wrap(err, "unable to make config directory")
	}

	err = os.WriteFile(homeDir+"/config/genesis.json", genDocBytes, 0644)
	return errors.Wrap(err, "unable to write genesis bytes to file")
}

// This is temporary solution to make both v1 & v2 & v3 cored version work.
// Since cosmos SDK changed structure of genesis file, we need to apply some patches to v3 (sdk v47)
// to make it compatible with v2 (sdk v45) binary.
func applyPatchToV3Genesis(originalGenDocBytes []byte) []byte {
	// Deleting "send_enabled" from "bank"
	modifiedGenDocStr, _ := sjson.Delete(string(originalGenDocBytes), "app_state.bank.send_enabled")

	// Iterating over "denom_metadata" in "bank"
	denomMetadataCount := gjson.Get(modifiedGenDocStr, "app_state.bank.denom_metadata.#").Int()
	for i := 0; i < int(denomMetadataCount); i++ {
		// Removing "uri" and "uri_hash" fields
		modifiedGenDocStr, _ = sjson.Delete(modifiedGenDocStr, fmt.Sprintf("app_state.bank.denom_metadata.%d.uri", i))
		modifiedGenDocStr, _ = sjson.Delete(modifiedGenDocStr, fmt.Sprintf("app_state.bank.denom_metadata.%d.uri_hash", i))
	}

	// Iterating over "gen_txs" in "genutil"
	genTxsCount := gjson.Get(modifiedGenDocStr, "app_state.genutil.gen_txs.#").Int()
	for i := 0; i < int(genTxsCount); i++ {
		// Deleting "tip" from "auth_info" in each "gen_txs" item
		modifiedGenDocStr, _ = sjson.Delete(modifiedGenDocStr, fmt.Sprintf("app_state.genutil.gen_txs.%d.auth_info.tip", i))
	}

	return []byte(modifiedGenDocStr)
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
