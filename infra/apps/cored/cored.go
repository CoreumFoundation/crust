package cored

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"

	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
	"github.com/CoreumFoundation/crust/pkg/retry"
	"github.com/CoreumFoundation/crust/pkg/rnd"
)

// AppType is the type of cored application
const AppType infra.AppType = "cored"

// New creates new cored app
func New(name string, cfg infra.Config, network *app.Network, appInfo *infra.AppInfo, ports Ports, rootNode *Cored) Cored {
	nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)
	validatorPublicKey, validatorPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	must.OK(err)

	stakerPubKey, stakerPrivKey := types.GenerateSecp256k1Key()

	err = network.FundAccount(stakerPubKey, "100000000000000000000000"+network.TokenSymbol())
	must.OK(err)
	clientCtx := app.NewDefaultClientContext().WithChainID(string(network.ChainID()))

	tx, err := client.PrepareTxStakingCreateValidator(clientCtx, validatorPublicKey, stakerPrivKey, "100000000"+network.TokenSymbol())
	must.OK(err)
	network.AddGenesisTx(tx)

	cored := Cored{
		name:                name,
		homeDir:             cfg.AppDir + "/" + name,
		config:              cfg,
		nodeID:              NodeID(nodePublicKey),
		nodePrivateKey:      nodePrivateKey,
		validatorPrivateKey: validatorPrivateKey,
		network:             network,
		appInfo:             appInfo,
		ports:               ports,
		rootNode:            rootNode,
		mu:                  &sync.RWMutex{},
		walletKeys: map[string]types.Secp256k1PrivateKey{
			"staker":  stakerPrivKey,
			"alice":   AlicePrivKey,
			"bob":     BobPrivKey,
			"charlie": CharliePrivKey,
		},
	}
	return cored
}

// Cored represents cored
type Cored struct {
	name                string
	homeDir             string
	config              infra.Config
	nodeID              string
	nodePrivateKey      ed25519.PrivateKey
	validatorPrivateKey ed25519.PrivateKey
	network             *app.Network
	appInfo             *infra.AppInfo
	ports               Ports
	rootNode            *Cored

	mu         *sync.RWMutex
	walletKeys map[string]types.Secp256k1PrivateKey
}

// Type returns type of application
func (c Cored) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (c Cored) Name() string {
	return c.name
}

// NodeID returns node ID
func (c Cored) NodeID() string {
	return c.nodeID
}

// Ports returns ports used by the application
func (c Cored) Ports() Ports {
	return c.ports
}

// TokenSymbol returns the governance token symbol. This is different
// for each network(i.e mainnet, testnet, etc)
func (c Cored) TokenSymbol() string {
	return c.network.TokenSymbol()
}

// ChainID returns ID of the chain
func (c Cored) ChainID() string {
	return string(c.network.ChainID())
}

// Info returns deployment info
func (c Cored) Info() infra.DeploymentInfo {
	return c.appInfo.Info()
}

// AddWallet adds wallet to genesis block and local keystore
func (c Cored) AddWallet(balances string) types.Wallet {
	pubKey, privKey := types.GenerateSecp256k1Key()
	err := c.network.FundAccount(pubKey, balances)
	must.OK(err)

	c.mu.Lock()
	defer c.mu.Unlock()

	var name string
	for {
		name = rnd.GetRandomName()
		if c.walletKeys[name] == nil {
			break
		}
	}

	c.walletKeys[name] = privKey
	return types.Wallet{Name: name, Key: privKey}
}

// Client creates new client for cored blockchain
func (c Cored) Client() Client {
	return NewClient(string(c.network.ChainID()), infra.JoinNetAddr("", c.Info().HostFromHost, c.Ports().RPC))
}

// HealthCheck checks if cored chain is ready to accept transactions
func (c Cored) HealthCheck(ctx context.Context) error {
	if c.appInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("cored chain hasn't started yet"))
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", c.Info().HostFromHost, c.ports.RPC), Path: "/status"}
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, statusURL.String(), nil))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retry.Retryable(err)
	}

	if resp.StatusCode != http.StatusOK {
		return retry.Retryable(errors.Errorf("health check failed, status code: %d, response: %s", resp.StatusCode, body))
	}

	data := struct {
		Result struct {
			SyncInfo struct {
				LatestBlockHash string `json:"latest_block_hash"` // nolint: tagliatelle
			} `json:"sync_info"` // nolint: tagliatelle
		} `json:"result"`
	}{}

	if err := json.Unmarshal(body, &data); err != nil {
		return retry.Retryable(err)
	}

	if data.Result.SyncInfo.LatestBlockHash == "" {
		return retry.Retryable(errors.New("genesis block hasn't been mined yet"))
	}

	return nil
}

// Deployment returns deployment of cored
func (c Cored) Deployment() infra.Deployment {
	deployment := infra.Binary{
		BinPath: c.config.BinDir + "/.cache/docker/cored",
		AppBase: infra.AppBase{
			Name: c.Name(),
			Info: c.appInfo,
			ArgsFunc: func() []string {
				args := []string{
					"start",
					"--home", targets.AppHomeDir,
					"--rpc.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.ports.RPC),
					"--p2p.laddr", infra.JoinNetAddrIP("tcp", net.IPv4zero, c.ports.P2P),
					"--grpc.address", infra.JoinNetAddrIP("", net.IPv4zero, c.ports.GRPC),
					"--grpc-web.address", infra.JoinNetAddrIP("", net.IPv4zero, c.ports.GRPCWeb),
					"--rpc.pprof_laddr", infra.JoinNetAddrIP("", net.IPv4zero, c.ports.PProf),
				}
				if c.rootNode != nil {
					args = append(args,
						"--p2p.persistent_peers", c.rootNode.NodeID()+"@"+infra.JoinNetAddr("", c.rootNode.Info().HostFromContainer, c.rootNode.Ports().P2P),
					)
				}

				return args
			},
			Ports: infra.PortsToMap(c.ports),
			PrepareFunc: func() error {
				c.mu.RLock()
				defer c.mu.RUnlock()

				nodeConfig := app.NodeConfig{
					Name:           c.name,
					PrometheusPort: c.ports.Prometheus,
					NodeKey:        c.nodePrivateKey,
					ValidatorKey:   c.validatorPrivateKey,
				}
				SaveConfig(nodeConfig, c.homeDir)

				AddKeysToStore(c.homeDir, c.walletKeys)

				return c.network.SaveGenesis(c.homeDir)
			},
			ConfigureFunc: func(ctx context.Context, deployment infra.DeploymentInfo) error {
				return c.saveClientWrapper(c.config.WrapperDir, deployment.HostFromHost)
			},
		},
	}
	if c.rootNode != nil {
		deployment.Requires = infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				infra.IsRunning(*c.rootNode),
			},
		}
	}
	return deployment
}

func (c Cored) saveClientWrapper(wrapperDir string, hostname string) error {
	client := `#!/bin/bash
OPTS=""
if [ "$1" == "tx" ] || [ "$1" == "q" ] || [ "$1" == "query" ]; then
	OPTS="$OPTS --chain-id ""` + string(c.network.ChainID()) + `"" --node ""` + infra.JoinNetAddr("tcp", hostname, c.ports.RPC) + `"""
fi
if [ "$1" == "tx" ] || [ "$1" == "keys" ]; then
	OPTS="$OPTS --keyring-backend ""test"""
fi

exec "` + c.config.BinDir + `/cored" --home "` + c.homeDir + `" "$@" $OPTS
`
	return ioutil.WriteFile(wrapperDir+"/"+c.Name(), []byte(client), 0o700)
}
