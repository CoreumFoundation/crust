package relayercosmos

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

	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	coreumconstant "github.com/CoreumFoundation/coreum/pkg/config/constant"
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
	// AppType is the type of relayer application.
	AppType infra.AppType = "relayer"

	// DefaultDebugPort is the default port relayer listens for debug/metric requests.
	DefaultDebugPort = 7597

	dockerEntrypoint = "run.sh"
)

// Config stores relayer app config.
type Config struct {
	Name                  string
	HomeDir               string
	AppInfo               *infra.AppInfo
	DebugPort             int
	Cored                 cored.Cored
	CoreumRelayerMnemonic string
	PeeredChain           cosmoschain.BaseApp
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

// Info returns deployment info.
func (r Relayer) Info() infra.DeploymentInfo {
	return r.config.AppInfo.Info()
}

// HealthCheck checks if relayer is operating.
func (r Relayer) HealthCheck(ctx context.Context) error {
	const cosmosHeightMetricName = "cosmos_relayer_chain_latest_height"

	if r.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("realyer hasn't started yet"))
	}

	statusURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", r.Info().HostFromHost, r.config.DebugPort), Path: "/relayer/metrics"}
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
	cosmosHeightMF, ok := mf[cosmosHeightMetricName]
	if !ok {
		return retry.Retryable(errors.Errorf("health check failed, no %q metric in the response", cosmosHeightMetricName))
	}

	chainIDs := map[string]struct{}{
		r.config.PeeredChain.AppConfig().ChainID:          {},
		string(r.config.Cored.Config().Network.ChainID()): {},
	}

	for _, metricItem := range cosmosHeightMF.Metric {
		for _, label := range metricItem.Label {
			if label.Value == nil {
				continue
			}
			if _, found := chainIDs[*label.Value]; found {
				metricGauge := metricItem.Gauge
				if metricGauge.Value == nil {
					continue
				}
				if *metricGauge.Value > 0 {
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
			"debug": r.config.DebugPort,
		},
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				r.config.Cored,
				r.config.PeeredChain,
			},
		},
		PrepareFunc: r.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
	}
}

func (r Relayer) prepare() error {
	if err := r.saveConfigFile(); err != nil {
		return err
	}

	return r.saveRunScriptFile()
}

func (r Relayer) saveConfigFile() error {
	configArgs := struct {
		CoreumChanID        string
		CoreumRPCURL        string
		CoreumAccountPrefix string

		PeerChanID        string
		PeerRPCURL        string
		PeerAccountPrefix string
	}{
		CoreumChanID:        string(r.config.Cored.Config().Network.ChainID()),
		CoreumRPCURL:        infra.JoinNetAddr("http", r.config.Cored.Info().HostFromContainer, r.config.Cored.Config().Ports.RPC),
		CoreumAccountPrefix: r.config.Cored.Config().Network.AddressPrefix(),

		PeerChanID:        r.config.PeeredChain.AppConfig().ChainID,
		PeerRPCURL:        infra.JoinNetAddr("http", r.config.PeeredChain.Info().HostFromContainer, r.config.PeeredChain.AppConfig().Ports.RPC),
		PeerAccountPrefix: r.config.PeeredChain.AppTypeConfig().AccountPrefix,
	}

	buf := &bytes.Buffer{}
	if err := configTemplate.Execute(buf, configArgs); err != nil {
		return errors.WithStack(err)
	}

	configFolderPath := filepath.Join(r.config.HomeDir, ".relayer", "config")
	err := os.MkdirAll(configFolderPath, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	err = os.WriteFile(filepath.Join(configFolderPath, "config.yaml"), buf.Bytes(), 0o700)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r Relayer) saveRunScriptFile() error {
	scriptArgs := struct {
		HomePath string

		CoreumChanID          string
		CoreumRelayerMnemonic string
		CoreumRelayerCoinType uint32

		PeerChanID          string
		PeerRelayerMnemonic string

		DebugPort int
	}{
		HomePath: targets.AppHomeDir,

		CoreumChanID:          string(r.config.Cored.Config().Network.ChainID()),
		CoreumRelayerMnemonic: r.config.CoreumRelayerMnemonic,
		CoreumRelayerCoinType: coreumconstant.CoinType,

		PeerChanID:          r.config.PeeredChain.AppConfig().ChainID,
		PeerRelayerMnemonic: r.config.PeeredChain.AppConfig().RelayerMnemonic,

		DebugPort: r.config.DebugPort,
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, scriptArgs); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(r.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
