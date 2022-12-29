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
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

var (
	//go:embed config/prometheus.tmpl
	configTmpl     string
	configTemplate = template.Must(template.New("").Parse(configTmpl))

	//go:embed config/alert.rules
	alertRules string
)

const (
	// AppType is the type of prometheus application
	AppType infra.AppType = "prometheus"

	configFileName     = "prometheus.yml"
	alertRulesFileName = "alert.rules"

	// DefaultPort is default prometheus UI port.
	DefaultPort = 9092
)

// Config stores prometheus app config
type Config struct {
	Name       string
	HomeDir    string
	Port       int
	AppInfo    *infra.AppInfo
	CoredNodes []cored.Cored
}

// New creates new prometheus app
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

// Prometheus represents prometheus
type Prometheus struct {
	config Config
}

// Type returns type of application
func (p Prometheus) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (p Prometheus) Name() string {
	return p.config.Name
}

// Info returns deployment info
func (p Prometheus) Info() infra.DeploymentInfo {
	return p.config.AppInfo.Info()
}

// DataSourcePort returns the data source port of the Prometheus.
func (p Prometheus) DataSourcePort() int {
	return DefaultPort
}

// Deployment returns deployment of prometheus
func (p Prometheus) Deployment() infra.Deployment {
	return infra.Deployment{
		Image: "prom/prometheus:v2.41.0",
		Name:  p.Name(),
		Info:  p.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      p.config.HomeDir,
				Destination: "/etc/prometheus",
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

				return containers
			}(),
		},
		PrepareFunc: p.saveConfigFile,
		ArgsFunc: func() []string {
			return []string{
				"--web.listen-address",
				fmt.Sprintf("0.0.0.0:%d", p.config.Port),
				"--config.file",
				"/etc/prometheus/prometheus.yml",
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

	nodesConfig := make([]nodesConfigArgs, 0, len(p.config.CoredNodes))
	for _, node := range p.config.CoredNodes {
		nodesConfig = append(nodesConfig, nodesConfigArgs{
			Host: node.Info().HostFromContainer,
			Port: node.Config().Ports.Prometheus,
			Name: node.Name(),
		})
	}

	configArgs := struct {
		Nodes []nodesConfigArgs
	}{
		Nodes: nodesConfig,
	}

	buf := &bytes.Buffer{}
	if err := configTemplate.Execute(buf, configArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(filepath.Join(p.config.HomeDir, configFileName), buf.Bytes(), 0o700)
	if err != nil {
		return errors.Wrapf(err, "can't write prometheus %s file", configFileName)
	}

	err = os.WriteFile(filepath.Join(p.config.HomeDir, alertRulesFileName), []byte(alertRules), 0o700)
	if err != nil {
		return errors.Wrapf(err, "can't write prometheus %s file", alertRulesFileName)
	}

	return nil
}
