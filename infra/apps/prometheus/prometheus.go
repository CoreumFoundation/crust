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
	"github.com/CoreumFoundation/crust/infra/apps/hermes"
	"github.com/CoreumFoundation/crust/infra/apps/relayercosmos"
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
	BDJuno     bdjuno.BDJuno
	Hermes     hermes.Hermes
	// FIXME (dzmitry): after replacing cosmos relayer with hermes for osmosis sth here probably stops working.
	// Instead of enumerating all the apps here, it would be nice to expect an interface like `MonitoredApps []MonitorableApp`
	// defining whatever methods are required to configure the app in the Prometheus. This way we may easily add/remove/replace
	// apps at any time, without coming back to this place.
	RelayerCosmos relayercosmos.Relayer
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

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", p.Info().HostFromHost, p.config.Port), Path: "/status"}
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
		Image:     "prom/prometheus:v2.45.0",
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
			Timeout: 20 * time.Second,
			Dependencies: func() []infra.HealthCheckCapable {
				containers := make([]infra.HealthCheckCapable, 0, len(p.config.CoredNodes))
				for _, node := range p.config.CoredNodes {
					containers = append(containers, node)
				}
				// determine whether the bdjuno is provided
				if p.config.BDJuno.Name() != "" {
					containers = append(containers, p.config.BDJuno)
				}
				// determine whether the hermes is provided
				if p.config.Hermes.Name() != "" {
					containers = append(containers, p.config.Hermes)
				}
				// determine whether the relayer cosmos is provided
				if p.config.RelayerCosmos.Name() != "" {
					containers = append(containers, p.config.RelayerCosmos)
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

func (p Prometheus) saveConfigFile() error {
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
		Nodes         []nodesConfigArgs
		BDJuno        hostPortConfig
		Hermes        hostPortConfig
		RelayerCosmos hostPortConfig
	}{
		Nodes: nodesConfig,
	}

	// determine whether the bdjuno is provided
	if p.config.BDJuno.Name() != "" {
		configArgs.BDJuno = hostPortConfig{
			Host: p.config.BDJuno.Info().HostFromContainer,
			Port: p.config.BDJuno.Config().TelemetryPort,
		}
	}

	// determine whether the hermes is provided
	if p.config.Hermes.Name() != "" {
		configArgs.Hermes = hostPortConfig{
			Host: p.config.Hermes.Info().HostFromContainer,
			Port: p.config.Hermes.Config().TelemetryPort,
		}
	}

	// determine whether the relayer cosmos is provided
	if p.config.RelayerCosmos.Name() != "" {
		configArgs.RelayerCosmos = hostPortConfig{
			Host: p.config.RelayerCosmos.Info().HostFromContainer,
			Port: p.config.RelayerCosmos.Config().DebugPort,
		}
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
		chainID = string(p.config.CoredNodes[0].Config().NetworkConfig.ChainID())
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
