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

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/config"
	coreumstaking "github.com/CoreumFoundation/coreum/pkg/staking"
	"github.com/CoreumFoundation/coreum/pkg/tx"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
	"github.com/CoreumFoundation/crust/pkg/rnd"
)

// AppType is the type of cored application
const AppType infra.AppType = "cored"

// Config stores cored app config
type Config struct {
	Name           string
	HomeDir        string
	BinDir         string
	WrapperDir     string
	Network        *config.Network
	AppInfo        *infra.AppInfo
	Ports          Ports
	IsValidator    bool
	StakerMnemonic string
	RootNode       *Cored
	Wallets        map[string]types.Secp256k1PrivateKey
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

		clientCtx := tx.NewClientContext(module.NewBasicManager(
			staking.AppModuleBasic{},
		)).WithChainID(string(cfg.Network.ChainID()))

		createValidatorTx, err := coreumstaking.PrepareTxStakingCreateValidator(clientCtx, valPublicKey, stakerPrivKey, stake.String())
		must.OK(err)
		cfg.Network.AddGenesisTx(createValidatorTx)
	}

	return Cored{
		config:              cfg,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: validatorPrivateKey,
		mu:                  &sync.RWMutex{},
		walletKeys:          cfg.Wallets,
	}
}

// Cored represents cored
type Cored struct {
	config              Config
	nodeID              string
	nodePrivateKey      ed25519.PrivateKey
	validatorPrivateKey ed25519.PrivateKey

	mu         *sync.RWMutex
	walletKeys map[string]types.Secp256k1PrivateKey
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

// AddWallet adds wallet to genesis block and local keystore
func (c Cored) AddWallet(balances string) types.Wallet {
	pubKey, privKey := types.GenerateSecp256k1Key()
	err := c.config.Network.FundAccount(pubKey, balances)
	must.OK(err)

	c.mu.Lock()
	defer c.mu.Unlock()

	var name string
	for {
		name = rnd.GetRandomName()
		if len(c.walletKeys[name]) == 0 {
			break
		}
	}

	c.walletKeys[name] = privKey
	return types.Wallet{Name: name, Key: privKey}
}

// Client creates new client for cored blockchain
func (c Cored) Client() client.Client {
	return client.New(c.config.Network.ChainID(), infra.JoinNetAddr("", c.Info().HostFromHost, c.Config().Ports.RPC))
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
		MountAppDir: true,
		Image:       "cored:znet",
		Name:        c.Name(),
		Info:        c.config.AppInfo,
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

			addKeysToStore(c.config.HomeDir, c.walletKeys)

			return c.config.Network.SaveGenesis(c.config.HomeDir)
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
	return errors.WithStack(os.WriteFile(wrapperDir+"/"+c.Name(), []byte(client), 0o700))
}
