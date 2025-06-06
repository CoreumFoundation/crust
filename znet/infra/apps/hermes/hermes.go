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
	"github.com/samber/lo"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	coreumconstant "github.com/CoreumFoundation/coreum/v6/pkg/config/constant"
	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/apps/cored"
	"github.com/CoreumFoundation/crust/znet/infra/cosmoschain"
	"github.com/CoreumFoundation/crust/znet/infra/targets"
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
	PeeredChains          []cosmoschain.BaseApp
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

	statusURL := url.URL{
		Scheme: "http",
		Host:   infra.JoinNetAddr("", h.Info().HostFromHost, h.config.TelemetryPort),
		Path:   "/metrics",
	}
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
		string(h.config.Cored.Config().GenesisInitConfig.ChainID): {},
	}
	for _, chain := range h.config.PeeredChains {
		chainIDs[chain.AppConfig().ChainID] = struct{}{}
	}

	for _, metricItem := range metricFamily.GetMetric() {
		for _, label := range metricItem.GetLabel() {
			if _, found := chainIDs[label.GetValue()]; found {
				if metricItem.GetCounter().GetValue() > 0 {
					delete(chainIDs, label.GetValue())
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
	dependencies := []infra.HealthCheckCapable{
		h.config.Cored,
	}
	for _, chain := range h.config.PeeredChains {
		dependencies = append(dependencies, chain)
	}

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
			Timeout:      40 * time.Second,
			Dependencies: dependencies,
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

func (h Hermes) prepare(_ context.Context) error {
	if err := h.saveConfigFile(); err != nil {
		return err
	}

	return h.saveRunScriptFile()
}

func (h Hermes) saveConfigFile() error {
	type chainConfig struct {
		ChanID        string
		RPCURL        string
		GRPCURL       string
		WebsocketURL  string
		AccountPrefix string
		GasPrice      sdk.DecCoin
	}

	configArgs := struct {
		TelemetryPort int
		Chains        []chainConfig
	}{
		TelemetryPort: h.config.TelemetryPort,
		Chains: []chainConfig{
			{
				ChanID: string(h.config.Cored.Config().GenesisInitConfig.ChainID),
				RPCURL: infra.JoinNetAddr("http", h.config.Cored.Info().HostFromContainer, h.config.Cored.Config().Ports.RPC),
				GRPCURL: infra.JoinNetAddr(
					"http",
					h.config.Cored.Info().HostFromContainer,
					h.config.Cored.Config().Ports.GRPC,
				),
				AccountPrefix: h.config.Cored.Config().GenesisInitConfig.AddressPrefix,
				GasPrice:      lo.Must1(sdk.ParseDecCoin(h.config.Cored.Config().GasPriceStr)),
			},
		},
	}

	for _, chain := range h.config.PeeredChains {
		configArgs.Chains = append(configArgs.Chains, chainConfig{
			ChanID: chain.AppConfig().ChainID,
			RPCURL: infra.JoinNetAddr(
				"http",
				chain.Info().HostFromContainer,
				chain.AppConfig().Ports.RPC,
			),
			GRPCURL: infra.JoinNetAddr(
				"http",
				chain.Info().HostFromContainer,
				chain.AppConfig().Ports.GRPC,
			),
			AccountPrefix: chain.AppTypeConfig().AccountPrefix,
			GasPrice:      lo.Must1(sdk.ParseDecCoin(chain.AppConfig().GasPriceStr)),
		})
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
	type peersConfig struct {
		ChanID          string
		RelayerMnemonic string
	}

	peers := make([]peersConfig, 0)
	for _, chain := range h.config.PeeredChains {
		peers = append(peers, peersConfig{
			ChanID:          chain.AppConfig().ChainID,
			RelayerMnemonic: chain.AppConfig().RelayerMnemonic,
		})
	}

	scriptArgs := struct {
		HomePath string

		CoreumChanID          string
		CoreumRelayerMnemonic string
		CoreumRPCURL          string
		CoreumRelayerCoinType uint32

		Peers []peersConfig
	}{
		HomePath: targets.AppHomeDir,

		CoreumChanID:          string(h.config.Cored.Config().GenesisInitConfig.ChainID),
		CoreumRelayerMnemonic: h.config.CoreumRelayerMnemonic,
		CoreumRPCURL: infra.JoinNetAddr(
			"http",
			h.config.Cored.Info().HostFromContainer,
			h.config.Cored.Config().Ports.RPC,
		),
		CoreumRelayerCoinType: coreumconstant.CoinType,

		Peers: peers,
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
