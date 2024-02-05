package prometheus

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/hermes"
)

var (
	//go:embed config/prometheus.tmpl
	configTmpl     string
	configTemplate = template.Must(template.New("").Parse(configTmpl))

	//go:embed config/alert.tmpl
	alertRules         string
	alertRulesTemplate = template.Must(template.New("").Parse(alertRules))
)

const (
	// AppType is the type of prometheus application.
	AppType infra.AppType = "prometheus"

	configFileName     = "prometheus.yml"
	alertRulesFileName = "alert.rules"

	// DefaultPort is default prometheus UI port.
	DefaultPort = 9092
)

// Config stores prometheus app config.
type Config struct {
	Name       string
	HomeDir    string
	Port       int
	AppInfo    *infra.AppInfo
	CoredNodes []cored.Cored
	Faucet     faucet.Faucet
	BDJuno     bdjuno.BDJuno
	HermesApps []hermes.Hermes
}

// New creates new prometheus app.
func New(config Config) Prometheus {
	return Prometheus{
		config: config,
	}
}

// HealthCheck checks if relayer is operating.
func (p Prometheus) HealthCheck(ctx context.Context) error {
	if p.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("prometheus hasn't started yet"))
	}

	statusURL := url.URL{
		Scheme: "http",
		Host:   infra.JoinNetAddr("", p.Info().HostFromHost, p.config.Port),
		Path:   "/status",
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

// Prometheus represents prometheus.
type Prometheus struct {
	config Config
}

// Type returns type of application.
func (p Prometheus) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (p Prometheus) Name() string {
	return p.config.Name
}

// Info returns deployment info.
func (p Prometheus) Info() infra.DeploymentInfo {
	return p.config.AppInfo.Info()
}

// DataSourcePort returns the data source port of the Prometheus.
func (p Prometheus) DataSourcePort() int {
	return p.config.Port
}

// Deployment returns deployment of prometheus.
func (p Prometheus) Deployment() infra.Deployment {
	return infra.Deployment{
		Image:     "prom/prometheus:v2.47.2",
		RunAsUser: true,
		Name:      p.Name(),
		Info:      p.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      p.config.HomeDir,
				Destination: "/etc/prometheus",
			},
			{
				Source:      filepath.Join(p.config.HomeDir, "data"),
				Destination: "/prometheus",
			},
		},
		Ports: map[string]int{
			"metrics": p.config.Port,
		},
		Requires: infra.Prerequisites{
			Timeout: time.Minute, // relayers are slow
			Dependencies: func() []infra.HealthCheckCapable {
				containers := make([]infra.HealthCheckCapable, 0, len(p.config.CoredNodes))
				for _, node := range p.config.CoredNodes {
					containers = append(containers, node)
				}
				// determine whether the bdjuno is provided
				if p.config.BDJuno.Name() != "" {
					containers = append(containers, p.config.BDJuno)
				}
				// append hermes apps
				for _, h := range p.config.HermesApps {
					containers = append(containers, h)
				}

				return containers
			}(),
		},
		PrepareFunc: p.saveConfigFile,
		ArgsFunc: func() []string {
			return []string{
				"--web.listen-address", fmt.Sprintf("0.0.0.0:%d", p.config.Port),
				"--config.file", "/etc/prometheus/prometheus.yml",
				"--storage.tsdb.path", "prometheus",
			}
		},
	}
}

func (p Prometheus) saveConfigFile(_ context.Context) error {
	type nodesConfigArgs struct {
		Host string
		Port int
		Name string
	}

	type hostPortConfig struct {
		Host string
		Port int
	}

	if err := os.MkdirAll(filepath.Join(p.config.HomeDir, "data"), 0o700); err != nil {
		return errors.WithStack(err)
	}

	nodesConfig := make([]nodesConfigArgs, 0, len(p.config.CoredNodes))
	for _, node := range p.config.CoredNodes {
		nodesConfig = append(nodesConfig, nodesConfigArgs{
			Host: node.Info().HostFromContainer,
			Port: node.Config().Ports.Prometheus,
			Name: node.Name(),
		})
	}

	configArgs := struct {
		Nodes      []nodesConfigArgs
		Faucet     hostPortConfig
		BDJuno     hostPortConfig
		HermesApps []hostPortConfig
	}{
		Nodes: nodesConfig,
	}

	// determine whether the faucet is provided
	if p.config.Faucet.Name() != "" {
		configArgs.Faucet = hostPortConfig{
			Host: p.config.Faucet.Info().HostFromContainer,
			Port: p.config.Faucet.Config().MonitoringPort,
		}
	}

	// determine whether the bdjuno is provided
	if p.config.BDJuno.Name() != "" {
		configArgs.BDJuno = hostPortConfig{
			Host: p.config.BDJuno.Info().HostFromContainer,
			Port: p.config.BDJuno.Config().TelemetryPort,
		}
	}

	for _, h := range p.config.HermesApps {
		configArgs.HermesApps = append(configArgs.HermesApps, hostPortConfig{
			Host: h.Info().HostFromContainer,
			Port: h.Config().TelemetryPort,
		})
	}

	buf := &bytes.Buffer{}
	if err := configTemplate.Execute(buf, configArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(filepath.Join(p.config.HomeDir, configFileName), buf.Bytes(), 0o700)
	if err != nil {
		return errors.Wrapf(err, "can't write prometheus %s file", configFileName)
	}

	chainID := ""
	if len(p.config.CoredNodes) > 0 {
		chainID = string(p.config.CoredNodes[0].Config().NetworkConfig.ChainID()) //nolint:staticcheck
	}
	rulesArgs := struct {
		ChainID string
	}{
		ChainID: chainID,
	}

	buf = &bytes.Buffer{}
	if err := alertRulesTemplate.Execute(buf, rulesArgs); err != nil {
		return errors.WithStack(err)
	}

	err = os.WriteFile(filepath.Join(p.config.HomeDir, alertRulesFileName), buf.Bytes(), 0o700)
	if err != nil {
		return errors.Wrapf(err, "can't write prometheus %s file", alertRulesFileName)
	}

	return nil
}
