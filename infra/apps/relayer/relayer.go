package relayer

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	coreumconfig "github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/gaia"
	"github.com/CoreumFoundation/crust/infra/targets"
)

var (
	//go:embed run.tmpl
	tmpl              string
	runScriptTemplate = template.Must(template.New("").Parse(tmpl))
)

const (
	// AppType is the type of relayer application.
	AppType infra.AppType = "relayer"

	// DefaultDebugPort is the default port relayer listens for debug/metric requests.
	DefaultDebugPort = 7597

	dockerEntrypoint = "run.sh"
)

// Config stores relayer app config.
type Config struct {
	Name      string
	HomeDir   string
	AppInfo   *infra.AppInfo
	DebugPort int
	Cored     cored.Cored
	Gaia      gaia.Gaia
}

// New creates new relayer app.
func New(config Config) Relayer {
	return Relayer{
		config: config,
	}
}

// Relayer represents relayer.
type Relayer struct {
	config Config
}

// Type returns type of application.
func (r Relayer) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (r Relayer) Name() string {
	return r.config.Name
}

// Port returns port used by the application.
func (r Relayer) Port() int {
	return r.config.DebugPort
}

// Info returns deployment info.
func (r Relayer) Info() infra.DeploymentInfo {
	return r.config.AppInfo.Info()
}

// HealthCheck checks if relayer is operating.
func (r Relayer) HealthCheck(ctx context.Context) error {
	if r.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("realyer hasn't started yet"))
	}

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", r.Info().HostFromHost, r.config.DebugPort), Path: "/metrics"}
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

// Deployment returns deployment of relayer.
func (r Relayer) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     "relayer:znet",
		Name:      r.Name(),
		Info:      r.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      r.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports: map[string]int{
			"server": r.config.DebugPort,
		},
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				r.config.Cored,
				r.config.Gaia,
			},
		},
		PrepareFunc: r.prepare,
		Entrypoint:  fmt.Sprintf(".%s/%s", targets.AppHomeDir, dockerEntrypoint),
	}
}

type runScriptArgs struct {
	HomePath string

	CoreumChanID          string
	CoreumRPCUrl          string
	CoreumAccountPrefix   string
	CoreumRelayerMnemonic string
	CoreumRelayerCoinType uint32

	GaiaChanID          string
	GaiaRPCUrl          string
	GaiaAccountPrefix   string
	GaiaRelayerMnemonic string

	DebugPort int
}

func (r Relayer) prepare() error {
	args := runScriptArgs{
		HomePath: targets.AppHomeDir,

		CoreumChanID:          string(r.config.Cored.Config().Network.ChainID()),
		CoreumRPCUrl:          infra.JoinNetAddr("http", r.config.Cored.Info().HostFromContainer, r.config.Cored.Config().Ports.RPC),
		CoreumAccountPrefix:   r.config.Cored.Config().Network.AddressPrefix(),
		CoreumRelayerMnemonic: r.config.Cored.Config().RelayerMnemonic,
		CoreumRelayerCoinType: coreumconfig.CoinType,

		GaiaChanID:          r.config.Gaia.Config().ChainID,
		GaiaRPCUrl:          infra.JoinNetAddr("http", r.config.Gaia.Info().HostFromContainer, r.config.Gaia.Config().Ports.RPC),
		GaiaAccountPrefix:   r.config.Gaia.Config().AccountPrefix,
		GaiaRelayerMnemonic: r.config.Gaia.Config().RelayerMnemonic,

		DebugPort: r.config.DebugPort,
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, args); err != nil {
		return err
	}

	err := os.WriteFile(path.Join(r.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o700)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
