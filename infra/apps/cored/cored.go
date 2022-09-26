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

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/staking"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
	"github.com/CoreumFoundation/crust/pkg/rnd"
)

// AppType is the type of cored application
const AppType infra.AppType = "cored"

// Config stores cored app config
type Config struct {
	Name              string
	HomeDir           string
	BinDir            string
	WrapperDir        string
	Network           *app.Network
	AppInfo           *infra.AppInfo
	Ports             Ports
	IsValidator       bool
	ValidatorMnemonic string
	RootNode          *Cored
	Wallets           map[string]types.Secp256k1PrivateKey
}

// New creates new cored app
func New(config Config) Cored {
	nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)

	var validatorPrivateKey ed25519.PrivateKey
	if config.IsValidator {
		valPublicKey, valPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)

		stakerPrivKey, err := PrivateKeyFromMnemonic(config.ValidatorMnemonic)
		must.OK(err)

		stakerPubKey := stakerPrivKey.PubKey()
		validatorPrivateKey = valPrivateKey
		stake := "100000000" + config.Network.TokenSymbol()

		must.OK(config.Network.FundAccount(stakerPubKey, stake))

		clientCtx := app.NewDefaultClientContext().WithChainID(string(config.Network.ChainID()))

		// FIXME: make clientCtx as private field of the client type
		tx, err := staking.PrepareTxStakingCreateValidator(clientCtx, valPublicKey, stakerPrivKey, stake)
		must.OK(err)
		config.Network.AddGenesisTx(tx)
	}

	return Cored{
		config:              config,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: validatorPrivateKey,
		mu:                  &sync.RWMutex{},
		walletKeys:          config.Wallets,
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

// NodeID returns node ID
func (c Cored) NodeID() string {
	return c.nodeID
}

// Ports returns ports used by the application
func (c Cored) Ports() Ports {
	return c.config.Ports
}

// Network returns the network config used in the chain
func (c Cored) Network() *app.Network {
	return c.config.Network
}

// Info returns deployment info
func (c Cored) Info() infra.DeploymentInfo {
	return c.config.AppInfo.Info()
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
	return client.New(c.config.Network.ChainID(), infra.JoinNetAddr("", c.Info().HostFromHost, c.Ports().RPC))
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
	deployment := infra.Binary{
		BinPath: c.config.BinDir + "/.cache/docker/cored/cored",
		AppBase: infra.AppBase{
			Name: c.Name(),
			Info: c.config.AppInfo,
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
						"--p2p.persistent_peers", c.config.RootNode.NodeID()+"@"+infra.JoinNetAddr("", c.config.RootNode.Info().HostFromContainer, c.config.RootNode.Ports().P2P),
					)
				}

				return args
			},
			Ports: infra.PortsToMap(c.config.Ports),
			PrepareFunc: func() error {
				c.mu.RLock()
				defer c.mu.RUnlock()

				nodeConfig := app.NodeConfig{
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
