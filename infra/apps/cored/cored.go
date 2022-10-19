package cored

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmosed25519 "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cosmossecp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/CoreumFoundation/coreum/pkg/tx"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
)

// AppType is the type of cored application
const AppType infra.AppType = "cored"

// Config stores cored app config
type Config struct {
	Name              string
	HomeDir           string
	BinDir            string
	WrapperDir        string
	Network           *config.Network
	AppInfo           *infra.AppInfo
	Ports             Ports
	IsValidator       bool
	StakerMnemonic    string
	RootNode          *Cored
	ImportedMnemonics map[string]string
}

// New creates new cored app
func New(cfg Config) Cored {
	nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)

	var validatorPrivateKey ed25519.PrivateKey
	if cfg.IsValidator {
		valPublicKey, valPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)

		stakerPrivKey, err := PrivateKeyFromMnemonic(cfg.StakerMnemonic)
		must.OK(err)

		stakerPubKey := stakerPrivKey.PubKey()
		validatorPrivateKey = valPrivateKey

		stake := sdk.NewInt64Coin(cfg.Network.TokenSymbol(), 1000000000)
		// the additional balance will be used to pay for the tx submitted from the stakers accounts
		additionalBalance := sdk.NewInt64Coin(cfg.Network.TokenSymbol(), 1000000000)

		must.OK(cfg.Network.FundAccount(stakerPubKey, stake.Add(additionalBalance).String()))

		clientCtx := tx.NewClientContext(newBasicManager()).WithChainID(string(cfg.Network.ChainID()))

		createValidatorTx, err := prepareTxStakingCreateValidator(cfg.Network.ChainID(), clientCtx.TxConfig(), valPublicKey, stakerPrivKey, stake)
		must.OK(err)
		cfg.Network.AddGenesisTx(createValidatorTx)
	}

	return Cored{
		config:              cfg,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: validatorPrivateKey,
		mu:                  &sync.RWMutex{},
		importedMnemonics:   cfg.ImportedMnemonics,
	}
}

// prepareTxStakingCreateValidator generates transaction of type MsgCreateValidator
func prepareTxStakingCreateValidator(
	chainID config.ChainID,
	txConfig cosmosclient.TxConfig,
	validatorPublicKey ed25519.PublicKey,
	stakerPrivateKey cosmossecp256k1.PrivKey,
	stakedBalance sdk.Coin,
) ([]byte, error) {
	// the passphrase here is the trick to import the private key into the keyring
	const passphrase = "tmp"

	commission := stakingtypes.CommissionRates{
		Rate:          sdk.MustNewDecFromStr("0.1"),
		MaxRate:       sdk.MustNewDecFromStr("0.2"),
		MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
	}

	stakerAddress := sdk.AccAddress(stakerPrivateKey.PubKey().Address())
	msg, err := stakingtypes.NewMsgCreateValidator(sdk.ValAddress(stakerAddress), &cosmosed25519.PubKey{Key: validatorPublicKey}, stakedBalance, stakingtypes.Description{Moniker: stakerAddress.String()}, commission, sdk.OneInt())
	if err != nil {
		return nil, errors.Wrap(err, "not able to make CreateValidatorMessage")
	}

	if err := msg.ValidateBasic(); err != nil {
		return nil, errors.Wrap(err, "not able to validate CreateValidatorMessage")
	}

	inMemKeyring := keyring.NewInMemory()

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

// Cored represents cored
type Cored struct {
	config              Config
	nodeID              string
	nodePrivateKey      ed25519.PrivateKey
	validatorPrivateKey ed25519.PrivateKey

	mu                *sync.RWMutex
	importedMnemonics map[string]string
}

// Type returns type of application
func (c Cored) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (c Cored) Name() string {
	return c.config.Name
}

// Info returns deployment info
func (c Cored) Info() infra.DeploymentInfo {
	return c.config.AppInfo.Info()
}

// NodeID returns node ID
func (c Cored) NodeID() string {
	return c.nodeID
}

// Config returns cored config.
func (c Cored) Config() Config {
	return c.config
}

// ClientContext creates new cored ClientContext.
func (c Cored) ClientContext() tx.ClientContext {
	rpcClient, err := cosmosclient.NewClientFromNode(infra.JoinNetAddr("", c.Info().HostFromHost, c.Config().Ports.RPC))
	must.OK(err)

	return tx.NewClientContext(newBasicManager()).
		WithChainID(string(c.config.Network.ChainID())).
		WithClient(rpcClient).
		WithKeyring(keyring.NewInMemory()).
		WithBroadcastMode(flags.BroadcastBlock)
}

// TxFactory returns factory with present values for the chain.
func (c Cored) TxFactory(clientCtx tx.ClientContext) tx.Factory {
	return tx.Factory{}.
		WithKeybase(clientCtx.Keyring()).
		WithChainID(string(c.config.Network.ChainID())).
		WithTxConfig(clientCtx.TxConfig())
}

// HealthCheck checks if cored chain is ready to accept transactions
func (c Cored) HealthCheck(ctx context.Context) error {
	if c.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("cored chain hasn't started yet"))
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", c.Info().HostFromHost, c.config.Ports.RPC), Path: "/status"}
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, statusURL.String(), nil))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}

	if resp.StatusCode != http.StatusOK {
		return retry.Retryable(errors.Errorf("health check failed, status code: %d, response: %s", resp.StatusCode, body))
	}

	data := struct {
		Result struct {
			SyncInfo struct {
				LatestBlockHash string `json:"latest_block_hash"` //nolint: tagliatelle
			} `json:"sync_info"` //nolint: tagliatelle
		} `json:"result"`
	}{}

	if err := json.Unmarshal(body, &data); err != nil {
		return retry.Retryable(errors.WithStack(err))
	}

	if data.Result.SyncInfo.LatestBlockHash == "" {
		return retry.Retryable(errors.New("genesis block hasn't been mined yet"))
	}

	return nil
}

// Deployment returns deployment of cored
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
					Value: filepath.Join(targets.AppHomeDir, string(c.config.Network.ChainID())),
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
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.Network.ChainID()), "config"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "data"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.Network.ChainID()), "data"),
			},
			{
				Source:      filepath.Join(c.config.HomeDir, "cosmovisor", "upgrades"),
				Destination: filepath.Join(targets.AppHomeDir, string(c.config.Network.ChainID()), "cosmovisor", "upgrades"),
			},
		},
		ArgsFunc: func() []string {
			args := []string{
				"start",
				"--home", targets.AppHomeDir,
				"--log_level", "debug",
				"--trace",
				"--rpc.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.RPC),
				"--p2p.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.config.Ports.P2P),
				"--grpc.address", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.GRPC),
				"--grpc-web.address", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.GRPCWeb),
				"--rpc.pprof_laddr", infra.JoinNetAddrIP("", net.IPv4zero, c.config.Ports.PProf),
				"--chain-id", string(c.config.Network.ChainID()),
			}
			if c.config.RootNode != nil {
				args = append(args,
					"--p2p.persistent_peers", c.config.RootNode.NodeID()+"@"+infra.JoinNetAddr("", c.config.RootNode.Info().HostFromContainer, c.config.RootNode.Config().Ports.P2P),
				)
			}

			return args
		},
		Ports: infra.PortsToMap(c.config.Ports),
		PrepareFunc: func() error {
			c.mu.RLock()
			defer c.mu.RUnlock()

			nodeConfig := config.NodeConfig{
				Name:           c.config.Name,
				PrometheusPort: c.config.Ports.Prometheus,
				NodeKey:        c.nodePrivateKey,
				ValidatorKey:   c.validatorPrivateKey,
			}
			SaveConfig(nodeConfig, c.config.HomeDir)

			importMnemonicsToKeyring(c.config.HomeDir, c.importedMnemonics)

			if err := c.config.Network.SaveGenesis(c.config.HomeDir); err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin"), 0o700); err != nil {
				return errors.WithStack(err)
			}
			if err := os.Symlink("/bin/cored", filepath.Join(c.config.HomeDir, "cosmovisor", "genesis", "bin", "cored")); err != nil {
				return errors.WithStack(err)
			}
			return errors.WithStack(copyFile(filepath.Join(c.config.BinDir, ".cache", "docker", "cored-upgrade"),
				filepath.Join(c.config.HomeDir, "cosmovisor", "upgrades", "upgrade", "bin", "cored"), 0o755))
		},
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

func newBasicManager() module.BasicManager {
	return module.NewBasicManager(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
	)
}

func (c Cored) saveClientWrapper(wrapperDir string, hostname string) error {
	client := `#!/bin/bash
OPTS=""
if [ "$1" == "tx" ] || [ "$1" == "q" ] || [ "$1" == "query" ]; then
	OPTS="$OPTS --node ""` + infra.JoinNetAddr("tcp", hostname, c.config.Ports.RPC) + `"""
fi
if [ "$1" == "tx" ] || [ "$1" == "keys" ]; then
	OPTS="$OPTS --keyring-backend ""test"""
fi

exec "` + c.config.BinDir + `/cored" --chain-id "` + string(c.config.Network.ChainID()) + `" --home "` + filepath.Dir(c.config.HomeDir) + `" "$@" $OPTS
`
	return errors.WithStack(os.WriteFile(filepath.Join(wrapperDir, c.Name()), []byte(client), 0o700))
}

func copyFile(src, dst string, perm os.FileMode) error {
	fr, err := os.Open(src)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fr.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	//nolint:nosnakecase // Those constants are out of our control
	fw, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fw.Close()

	_, err = io.Copy(fw, fr)
	return errors.WithStack(err)
}
