package xrpl

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
)

var (
	//go:embed run.tmpl
	scriptTmpl        string
	runScriptTemplate = template.Must(template.New("").Parse(scriptTmpl))

	//go:embed config.tmpl
	configTmpl     string
	configTemplate = template.Must(template.New("").Parse(configTmpl))
)

const (
	// AppType is the type of xrpl application.
	AppType infra.AppType = "xrpl"

	// DefaultRPCPort is the default xrp RPC port.
	DefaultRPCPort = 5005

	// DefaultWSPort is the default xrp WS port.
	DefaultWSPort = 6006

	// DefaultFaucetSeed is default faucet seed used in Stand-Alone Mode.
	DefaultFaucetSeed = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"

	dockerEntrypoint = "run.sh"
)

// Config stores xrpl app config.
type Config struct {
	Name       string
	HomeDir    string
	AppInfo    *infra.AppInfo
	RPCPort    int
	WSPort     int
	FaucetSeed string
}

// New creates new xrpl app.
func New(config Config) XRPL {
	return XRPL{
		config: config,
	}
}

// XRPL represents xrpl.
type XRPL struct {
	config Config
}

// Type returns type of application.
func (x XRPL) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (x XRPL) Name() string {
	return x.config.Name
}

// Info returns deployment info.
func (x XRPL) Info() infra.DeploymentInfo {
	return x.config.AppInfo.Info()
}

// Config returns config.
func (x XRPL) Config() Config {
	return x.config
}

// HealthCheck checks if xrpl is operating.
func (x XRPL) HealthCheck(ctx context.Context) error {
	if x.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("xrpl hasn't started yet"))
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", x.Info().HostFromHost, x.config.RPCPort)}
	statusBody := `{"method":"server_info","params":[{"api_version": 1}]}`
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodPost, statusURL.String(), strings.NewReader(statusBody)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return retry.Retryable(errors.Errorf("health check failed, status code: %d", resp.StatusCode))
	}

	res := struct {
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Errorf("failed to read the response body, err: %v", err)
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}

	if res.Result.Status != "success" {
		return retry.Retryable(errors.Errorf("health check failed, result status: %s", res.Result.Status))
	}

	return nil
}

// Deployment returns deployment of xrpl.
func (x XRPL) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     "xrpllabsofficial/xrpld:1.11.0",
		Name:      x.Name(),
		Info:      x.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      x.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports: map[string]int{
			"rpc": x.config.RPCPort,
			"ws":  x.config.WSPort,
		},
		PrepareFunc: x.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
	}
}

func (x XRPL) prepare() error {
	if err := x.saveConfigFile(); err != nil {
		return err
	}

	return x.saveRunScriptFile()
}

func (x XRPL) saveConfigFile() error {
	configArgs := struct {
		HomePath string
		RPCPort  int
		WSPort   int
	}{
		HomePath: targets.AppHomeDir,
		RPCPort:  x.config.RPCPort,
		WSPort:   x.config.WSPort,
	}

	configFile, err := os.OpenFile(filepath.Join(x.config.HomeDir, "rippled.cfg"), os.O_CREATE|os.O_RDWR, 0o700)
	if err != nil {
		return errors.WithStack(err)
	}
	defer configFile.Close()

	if err := configTemplate.Execute(configFile, configArgs); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (x XRPL) saveRunScriptFile() error {
	scriptArgs := struct {
		HomePath string
	}{
		HomePath: targets.AppHomeDir,
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, scriptArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(x.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
