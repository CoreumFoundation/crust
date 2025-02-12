package callisto

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/apps/cored"
	"github.com/CoreumFoundation/crust/znet/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/znet/infra/targets"
)

var (
	//go:embed run.tmpl
	scriptTmpl        string
	runScriptTemplate = template.Must(template.New("").Parse(scriptTmpl))
)

const (
	// AppType is the type of callisto application.
	AppType infra.AppType = "callisto"

	// DefaultPort is the default port callisto listens on for client connections.
	DefaultPort = 3030

	// DefaultTelemetryPort is default port use for the callisto telemetry.
	DefaultTelemetryPort = 5001

	dockerEntrypoint = "run.sh"
)

// Config storescallisto app configuration.
type Config struct {
	Name           string
	HomeDir        string
	RepoDir        string
	AppInfo        *infra.AppInfo
	Port           int
	TelemetryPort  int
	ConfigTemplate string
	Cored          cored.Cored
	Postgres       postgres.Postgres
}

// New creates new callisto app.
func New(config Config) Callisto {
	return Callisto{
		config: config,
	}
}

// Callisto represents callisto.
type Callisto struct {
	config Config
}

// Type returns type of application.
func (j Callisto) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (j Callisto) Name() string {
	return j.config.Name
}

// Port returns port used by hasura to accept client connections.
func (j Callisto) Port() int {
	return j.config.Port
}

// Info returns deployment info.
func (j Callisto) Info() infra.DeploymentInfo {
	return j.config.AppInfo.Info()
}

// Config returns config.
func (j Callisto) Config() Config {
	return j.config
}

// Deployment returns deployment of callisto.
func (j Callisto) Deployment() infra.Deployment {
	return infra.Deployment{
		Image: "coreumfoundation/callisto:latest",
		Name:  j.Name(),
		Info:  j.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      j.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports: map[string]int{
			"actions":   j.config.Port,
			"telemetry": j.config.TelemetryPort,
		},
		Requires: infra.Prerequisites{
			Timeout: 40 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				j.config.Cored,
				j.config.Postgres,
			},
		},
		PrepareFunc: j.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
		DockerArgs: []string{
			// exec from root to call psql
			"--user", "root",
		},
	}
}

// HealthCheck checks if callisto is operating.
func (j Callisto) HealthCheck(ctx context.Context) error {
	if j.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("callisto hasn't started yet"))
	}

	statusURL := url.URL{
		Scheme: "http",
		Host:   infra.JoinNetAddr("", j.Info().HostFromHost, j.config.TelemetryPort),
		Path:   "/metrics",
	}
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

func (j Callisto) prepareConfig() []byte {
	configBuf := &bytes.Buffer{}
	must.OK(template.Must(template.New("config").Parse(j.config.ConfigTemplate)).Execute(configBuf, struct {
		Port  int
		Cored struct {
			Host            string
			PortRPC         int
			PortGRPC        int
			PortAPI         int
			AddressPrefix   string
			GenesisFilePath string
		}
		Postgres struct {
			Host string
			Port int
			User string
			DB   string
		}
	}{
		Port: j.config.Port,
		Cored: struct {
			Host            string
			PortRPC         int
			PortGRPC        int
			PortAPI         int
			AddressPrefix   string
			GenesisFilePath string
		}{
			Host:            j.config.Cored.Info().HostFromContainer,
			PortRPC:         j.config.Cored.Config().Ports.RPC,
			PortGRPC:        j.config.Cored.Config().Ports.GRPC,
			PortAPI:         j.config.Cored.Config().Ports.API,
			AddressPrefix:   sdk.GetConfig().GetBech32AccountAddrPrefix(),
			GenesisFilePath: targets.AppHomeDir + "/config/genesis.json",
		},
		Postgres: struct {
			Host string
			Port int
			User string
			DB   string
		}{
			Host: j.config.Postgres.Info().HostFromContainer,
			Port: j.config.Postgres.Port(),
			User: postgres.User,
			DB:   postgres.DB,
		},
	}))
	return configBuf.Bytes()
}

func (j Callisto) prepare(ctx context.Context) error {
	if err := j.config.Cored.SaveGenesis(ctx, j.config.HomeDir); err != nil {
		return err
	}

	if err := os.WriteFile(j.config.HomeDir+"/config.yaml", j.prepareConfig(), 0o644); err != nil {
		return err
	}

	return j.saveRunScriptFile()
}

func (j Callisto) saveRunScriptFile() error {
	scriptArgs := struct {
		HomePath    string
		PostgresURL string
	}{
		HomePath: targets.AppHomeDir,
		PostgresURL: fmt.Sprintf("postgres://%s@%s/%s",
			postgres.User,
			net.JoinHostPort(j.config.Postgres.Info().HostFromContainer, strconv.Itoa(j.config.Postgres.Port())),
			postgres.DB,
		),
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, scriptArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(j.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
