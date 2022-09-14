package faucet

import (
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/targets"
)

const (
	// AppType is the type of faucet application
	AppType infra.AppType = "faucet"

	// DefaultPort is the default port faucet listens on for client connections
	DefaultPort = 8090
)

// Config stores faucet app config
type Config struct {
	Name       string
	HomeDir    string
	BinDir     string
	ChainID    app.ChainID
	AppInfo    *infra.AppInfo
	Port       int
	PrivateKey types.Secp256k1PrivateKey
	Cored      cored.Cored
}

// New creates new faucet app
func New(config Config) Faucet {
	return Faucet{
		config: config,
	}
}

// Faucet represents faucet
type Faucet struct {
	config Config
}

// Type returns type of application
func (f Faucet) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (f Faucet) Name() string {
	return f.config.Name
}

// Port returns port used by the application
func (f Faucet) Port() int {
	return f.config.Port
}

// Info returns deployment info
func (f Faucet) Info() infra.DeploymentInfo {
	return f.config.AppInfo.Info()
}

// HealthCheck checks if cored chain is ready to accept transactions
func (f Faucet) HealthCheck(ctx context.Context) error {
	if f.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("faucet hasn't started yet"))
	}

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", f.Info().HostFromHost, f.config.Port), Path: "/api/faucet/v1/status"}
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, statusURL.String(), nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return retry.Retryable(errors.Errorf("health check failed, status code: %d", resp.StatusCode))
	}
	return nil
}

// Deployment returns deployment of cored
func (f Faucet) Deployment() infra.Deployment {
	return infra.Binary{
		BinPath: f.config.BinDir + "/.cache/docker/faucet",
		AppBase: infra.AppBase{
			Name: f.Name(),
			Info: f.config.AppInfo,
			ArgsFunc: func() []string {
				return []string{
					"--address", infra.JoinNetAddrIP("", net.IPv4zero, f.config.Port),
					"--chain-id", string(f.config.ChainID),
					"--key-path-mnemonic", filepath.Join(targets.AppHomeDir, "mnemonic-key"),
					// TODO (milad): remove after faucet is updated to use mnemonic
					"--key-path", filepath.Join(targets.AppHomeDir, "key"),
					"--node", infra.JoinNetAddr("tcp", f.config.Cored.Info().HostFromContainer, f.config.Cored.Ports().RPC),
					"--log-format", "yaml",
				}
			},
			Ports: map[string]int{
				"server": f.config.Port,
			},
			Requires: infra.Prerequisites{
				Timeout: 20 * time.Second,
				Dependencies: []infra.HealthCheckCapable{
					f.config.Cored,
				},
			},
			PrepareFunc: func() error {
				// TODO (milad): remove after faucet is updated to use mnemonic
				err := errors.WithStack(os.WriteFile(filepath.Join(f.config.HomeDir, "key"), []byte(hex.EncodeToString(f.config.PrivateKey)), 0o400))
				if err != nil {
					return err
				}

				return errors.WithStack(os.WriteFile(filepath.Join(f.config.HomeDir, "mnemonic-key"), []byte(PrivateKeyMnemonic), 0o400))
			},
		},
	}
}
