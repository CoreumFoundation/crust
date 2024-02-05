package grafana

import (
	"bytes"
	"context"
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/prometheus"
)

var (
	//go:embed config/datasource.tmpl
	datasourceTmpl     string
	datasourceTemplate = template.Must(template.New("").Parse(datasourceTmpl))

	//go:embed config/dashboards
	dashboards embed.FS
)

const (
	// AppType is the type of grafana application.
	AppType infra.AppType = "grafana"

	datasourceFileName = "datasource.yml"
	dashboardsFolder   = "dashboards"

	// DefaultPort is default grafana port.
	DefaultPort = 3001
)

// Config stores grafana app config.
type Config struct {
	Name       string
	HomeDir    string
	Port       int
	AppInfo    *infra.AppInfo
	CoredNodes []cored.Cored
	Prometheus prometheus.Prometheus
}

// New creates new grafana app.
func New(config Config) Grafana {
	return Grafana{
		config: config,
	}
}

// Grafana represents grafana.
type Grafana struct {
	config Config
}

// Type returns type of application.
func (g Grafana) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (g Grafana) Name() string {
	return g.config.Name
}

// Info returns deployment info.
func (g Grafana) Info() infra.DeploymentInfo {
	return g.config.AppInfo.Info()
}

// Deployment returns deployment of grafana.
func (g Grafana) Deployment() infra.Deployment {
	return infra.Deployment{
		Image:     "grafana/grafana:10.2.1",
		RunAsUser: true,
		Name:      g.Name(),
		Info:      g.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      filepath.Join(g.config.HomeDir, datasourceFileName),
				Destination: "/etc/grafana/provisioning/datasources/datasource.yml",
			},
			{
				Source:      filepath.Join(g.config.HomeDir, dashboardsFolder),
				Destination: "/etc/grafana/provisioning/dashboards",
			},
		},
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "GF_USERS_ALLOW_SIGN_UP",
					Value: "false",
				},
				{
					Name:  "GF_SERVER_HTTP_PORT",
					Value: strconv.Itoa(g.config.Port),
				},
			}
		},
		Ports: map[string]int{
			"web": g.config.Port,
		},
		Requires: infra.Prerequisites{
			Timeout: 40 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				g.config.Prometheus,
			},
		},
		PrepareFunc: g.saveConfigFiles,
	}
}

func (g Grafana) saveConfigFiles(_ context.Context) error {
	dataSourceConfigArgs := struct {
		PrometheusHost string
		PrometheusPort int
	}{
		PrometheusHost: g.config.Prometheus.Info().HostFromContainer,
		PrometheusPort: g.config.Prometheus.DataSourcePort(),
	}

	buf := &bytes.Buffer{}
	if err := datasourceTemplate.Execute(buf, dataSourceConfigArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(filepath.Join(g.config.HomeDir, datasourceFileName), buf.Bytes(), 0o600)
	if err != nil {
		return errors.WithStack(err)
	}

	// create dashboards dir
	err = os.MkdirAll(filepath.Join(g.config.HomeDir, dashboardsFolder), 0o700)
	if err != nil {
		return errors.WithMessage(err, "can't create Home dashboards dir")
	}

	if err = fs.WalkDir(dashboards, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.WithMessage(err, "can't open dashboards FS")
		}
		if d.IsDir() {
			return nil
		}

		source, err := dashboards.Open(path)
		if err != nil {
			return errors.WithMessagef(err, "can't open file %q from dashboards", path)
		}
		defer source.Close()

		destination, err := os.OpenFile(
			filepath.Join(g.config.HomeDir, dashboardsFolder, d.Name()),
			os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			0o600,
		)
		if err != nil {
			return errors.WithMessagef(err, "can't create file %q file", d.Name())
		}
		defer destination.Close()

		if _, err = io.Copy(destination, source); err != nil {
			return errors.WithMessagef(err, "can't copy file %q from dashboards to %q folder", path, dashboardsFolder)
		}

		return nil
	}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
