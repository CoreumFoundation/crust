package hermes

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"text/template"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	coreumconstant "github.com/CoreumFoundation/coreum/v3/pkg/config/constant"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
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
	// AppType is the type of hermes application.
	AppType infra.AppType = "hermes"

	// DefaultTelemetryPort is the default port hermes listens for debug/metric requests.
	DefaultTelemetryPort = 7698

	dockerEntrypoint = "run.sh"
)

// Config stores hermes app config.
type Config struct {
	Name                  string
	HomeDir               string
	AppInfo               *infra.AppInfo
	TelemetryPort         int
	Cored                 cored.Cored
	CoreumRelayerMnemonic string
	PeeredChain           cosmoschain.BaseApp
}

// New creates new hermes app.
func New(config Config) Hermes {
	return Hermes{
		config: config,
	}
}

// Hermes represents hermes.
type Hermes struct {
	config Config
}

// Type returns type of application.
func (h Hermes) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (h Hermes) Name() string {
	return h.config.Name
}

// Info returns deployment info.
func (h Hermes) Info() infra.DeploymentInfo {
	return h.config.AppInfo.Info()
}

// Config returns config.
func (h Hermes) Config() Config {
	return h.config
}

// HealthCheck checks if hermes is operating.
func (h Hermes) HealthCheck(ctx context.Context) error {
	const metric = "ws_events_total"

	if h.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("realyer hasn't started yet"))
	}

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", h.Info().HostFromHost, h.config.TelemetryPort), Path: "/metrics"}
	req := must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, statusURL.String(), nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return retry.Retryable(errors.Errorf("health check failed, status code: %d", resp.StatusCode))
	}

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return errors.Wrap(err, "unexpected metrics response in the health check")
	}
	metricFamily, ok := mf[metric]
	if !ok {
		return retry.Retryable(errors.Errorf("health check failed, no %q metric in the response", metric))
	}

	chainIDs := map[string]struct{}{
		h.config.PeeredChain.AppConfig().ChainID:                {},
		string(h.config.Cored.Config().NetworkConfig.ChainID()): {},
	}

	for _, metricItem := range metricFamily.Metric {
		for _, label := range metricItem.Label {
			if label.Value == nil {
				continue
			}
			if _, found := chainIDs[*label.Value]; found {
				metricCounter := metricItem.Counter
				if metricCounter.Value == nil {
					continue
				}
				if *metricCounter.Value > 0 {
					delete(chainIDs, *label.Value)
				}
			}
		}
	}

	if len(chainIDs) != 0 {
		return errors.Wrapf(err, "the relayer chains %v are still syncing", chainIDs)
	}

	return nil
}

// Deployment returns deployment of hermes.
func (h Hermes) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     "hermes:znet",
		Name:      h.Name(),
		Info:      h.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      h.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports: map[string]int{
			"debug": h.config.TelemetryPort,
		},
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				h.config.Cored,
				h.config.PeeredChain,
			},
		},
		PrepareFunc: h.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
		DockerArgs: []string{
			// Restart is needed to handle chain upgrade by Hermes.
			// Since v2.0.2 -> v3 upgrade changes IBC version we have to restart Hermes because it stops working.
			// https://github.com/informalsystems/hermes/issues/3579
			"--restart", "on-failure:1000",
		},
	}
}

func (h Hermes) prepare() error {
	if err := h.saveConfigFile(); err != nil {
		return err
	}

	return h.saveRunScriptFile()
}

func (h Hermes) saveConfigFile() error {
	configArgs := struct {
		TelemetryPort int

		CoreumChanID        string
		CoreumRPCURL        string
		CoreumGRPCURL       string
		CoreumWebsocketURL  string
		CoreumAccountPrefix string
		CoreumGasPrice      sdk.DecCoin

		PeerChanID        string
		PeerRPCURL        string
		PeerGRPCURL       string
		PeerWebsocketURL  string
		PeerAccountPrefix string
		PeerGasPrice      sdk.DecCoin
	}{
		TelemetryPort: h.config.TelemetryPort,

		CoreumChanID:        string(h.config.Cored.Config().NetworkConfig.ChainID()),
		CoreumRPCURL:        infra.JoinNetAddr("http", h.config.Cored.Info().HostFromContainer, h.config.Cored.Config().Ports.RPC),
		CoreumGRPCURL:       infra.JoinNetAddr("http", h.config.Cored.Info().HostFromContainer, h.config.Cored.Config().Ports.GRPC),
		CoreumWebsocketURL:  infra.JoinNetAddr("ws", h.config.Cored.Info().HostFromContainer, h.config.Cored.Config().Ports.RPC) + "/websocket",
		CoreumAccountPrefix: h.config.Cored.Config().NetworkConfig.Provider.GetAddressPrefix(),
		// TODO(dzmitryhil) move gas price for host and peer chains to prams
		CoreumGasPrice: sdk.NewDecCoinFromDec("udevcore", sdk.MustNewDecFromStr("0.0625")),

		PeerChanID:        h.config.PeeredChain.AppConfig().ChainID,
		PeerRPCURL:        infra.JoinNetAddr("http", h.config.PeeredChain.Info().HostFromContainer, h.config.PeeredChain.AppConfig().Ports.RPC),
		PeerGRPCURL:       infra.JoinNetAddr("http", h.config.PeeredChain.Info().HostFromContainer, h.config.PeeredChain.AppConfig().Ports.GRPC),
		PeerWebsocketURL:  infra.JoinNetAddr("ws", h.config.PeeredChain.Info().HostFromContainer, h.config.PeeredChain.AppConfig().Ports.RPC) + "/websocket",
		PeerAccountPrefix: h.config.PeeredChain.AppTypeConfig().AccountPrefix,
		PeerGasPrice:      sdk.NewDecCoinFromDec("stake", sdk.MustNewDecFromStr("0.1")),
	}

	buf := &bytes.Buffer{}
	if err := configTemplate.Execute(buf, configArgs); err != nil {
		return errors.WithStack(err)
	}

	configFolderPath := filepath.Join(h.config.HomeDir, ".hermes")
	err := os.MkdirAll(configFolderPath, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	err = os.WriteFile(filepath.Join(configFolderPath, "config.toml"), buf.Bytes(), 0o700)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (h Hermes) saveRunScriptFile() error {
	scriptArgs := struct {
		HomePath string

		CoreumChanID          string
		CoreumRelayerMnemonic string
		CoreumRPCURL          string
		CoreumRelayerCoinType uint32

		PeerChanID          string
		PeerRelayerMnemonic string
	}{
		HomePath: targets.AppHomeDir,

		CoreumChanID:          string(h.config.Cored.Config().NetworkConfig.ChainID()),
		CoreumRelayerMnemonic: h.config.CoreumRelayerMnemonic,
		CoreumRPCURL:          infra.JoinNetAddr("http", h.config.Cored.Info().HostFromContainer, h.config.Cored.Config().Ports.RPC),
		CoreumRelayerCoinType: coreumconstant.CoinType,

		PeerChanID:          h.config.PeeredChain.AppConfig().ChainID,
		PeerRelayerMnemonic: h.config.PeeredChain.AppConfig().RelayerMnemonic,
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, scriptArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(h.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
