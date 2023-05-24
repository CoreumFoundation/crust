package relayercosmos

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"path"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pkg/errors"

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
	AppType infra.AppType = "relayercosmos"

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
	// TODO: bring back health check once we don't have any upgrade test which introduces ibc.
	// In upgrade tests, we are start from older versions that don't have ibc module. so
	// the health check will fail. we need to introduce back health check once all the binaries used
	// have ibc module enabled.
	return nil
}

// Deployment returns deployment of relayer.
func (r Relayer) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     "relayercosmos:znet",
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
		DockerArgs: []string{
			"--restart", "on-failure:1000", // TODO: remove after we enable health check
		},
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
		CoreumChanID:        string(r.config.Cored.Config().NetworkConfig.ChainID()),
		CoreumRPCURL:        infra.JoinNetAddr("http", r.config.Cored.Info().HostFromContainer, r.config.Cored.Config().Ports.RPC),
		CoreumAccountPrefix: r.config.Cored.Config().NetworkConfig.AddressPrefix,

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

		CoreumChanID:          string(r.config.Cored.Config().NetworkConfig.ChainID()),
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
