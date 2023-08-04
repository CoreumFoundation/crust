package cosmoschain

import (
	"bytes"
	"context"
	_ "embed"
	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"net"
	"os"
	"path"
	"path/filepath"
	"text/template"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v2/pkg/client"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/pkg/errors"
)

const dockerEntrypoint = "run.sh"

var (
	//go:embed run.tmpl
	tmpl              string
	runScriptTemplate = template.Must(template.New("").Parse(tmpl))
)

// Ports defines ports used by application.
type Ports struct {
	RPC     int `json:"rpc"`
	P2P     int `json:"p2p"`
	GRPC    int `json:"grpc"`
	GRPCWeb int `json:"grpcWeb"`
	PProf   int `json:"pprof"`
}

// AppConfig defines configuration of the application.
type AppConfig struct {
	Name            string
	HomeDir         string
	HomeName        string
	ChainID         string
	AppInfo         *infra.AppInfo
	Ports           Ports
	RelayerMnemonic string
	FundingMnemonic string
	TimeoutCommit   time.Duration
}

// AppTypeConfig defines configuration of the application type.
type AppTypeConfig struct {
	AppType       infra.AppType
	DockerImage   string
	AccountPrefix string
	ExecName      string
}

// New creates new gaia app.
func New(appTypeConfig AppTypeConfig, appConfig AppConfig) BaseApp {
	return BaseApp{
		appTypeConfig: appTypeConfig,
		appConfig:     appConfig,
	}
}

// BaseApp represents base for cosmos chain apps.
type BaseApp struct {
	appTypeConfig AppTypeConfig
	appConfig     AppConfig
}

// AppConfig returns the app config.
func (ba BaseApp) AppConfig() AppConfig {
	return ba.appConfig
}

// AppTypeConfig returns the app type config.
func (ba BaseApp) AppTypeConfig() AppTypeConfig {
	return ba.appTypeConfig
}

// Type returns type of application.
func (ba BaseApp) Type() infra.AppType {
	return ba.appTypeConfig.AppType
}

// Name returns name of app.
func (ba BaseApp) Name() string {
	return ba.appConfig.Name
}

// Ports returns port used by the application.
func (ba BaseApp) Ports() Ports {
	return ba.appConfig.Ports
}

// Info returns deployment info.
func (ba BaseApp) Info() infra.DeploymentInfo {
	return ba.appConfig.AppInfo.Info()
}

// ClientContext creates new cored ClientContext.
func (ba BaseApp) ClientContext() client.Context {
	rpcClient, err := cosmosclient.NewClientFromNode(infra.JoinNetAddr("http", ba.Info().HostFromHost, ba.appConfig.Ports.RPC))
	must.OK(err)

	return client.NewContext(client.DefaultContextConfig(), newBasicManager()).
		WithChainID(ba.appConfig.ChainID).
		WithRPCClient(rpcClient)
}

// HealthCheck checks if chain is ready.
func (ba BaseApp) HealthCheck(ctx context.Context) error {
	return infra.CheckCosmosNodeHealth(ctx, ba.ClientContext(), ba.Info())
}

// Deployment returns deployment.
func (ba BaseApp) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     ba.appTypeConfig.DockerImage,
		Name:      ba.appConfig.Name,
		Info:      ba.appConfig.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      ba.appConfig.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports:       infra.PortsToMap(ba.appConfig.Ports),
		PrepareFunc: ba.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
	}
}

func (ba BaseApp) prepare() error {
	args := struct {
		ExecName        string
		HomePath        string
		HomeName        string
		ChainID         string
		RelayerMnemonic string
		FundingMnemonic string
		TimeoutCommit   string
		RPCLaddr        string
		P2PLaddr        string
		GRPCAddress     string
		GRPCWebAddress  string
		RPCPprofLaddr   string
	}{
		ExecName:        ba.appTypeConfig.ExecName,
		HomePath:        targets.AppHomeDir,
		HomeName:        ba.appConfig.HomeName,
		ChainID:         ba.appConfig.ChainID,
		RelayerMnemonic: ba.appConfig.RelayerMnemonic,
		FundingMnemonic: ba.appConfig.FundingMnemonic,
		TimeoutCommit:   ba.appConfig.TimeoutCommit.String(),
		RPCLaddr:        infra.JoinNetAddrIP("tcp", net.IPv4zero, ba.appConfig.Ports.RPC),
		P2PLaddr:        infra.JoinNetAddrIP("tcp", net.IPv4zero, ba.appConfig.Ports.P2P),
		GRPCAddress:     infra.JoinNetAddrIP("", net.IPv4zero, ba.appConfig.Ports.GRPC),
		GRPCWebAddress:  infra.JoinNetAddrIP("", net.IPv4zero, ba.appConfig.Ports.GRPCWeb),
		RPCPprofLaddr:   infra.JoinNetAddrIP("", net.IPv4zero, ba.appConfig.Ports.PProf),
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, args); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(ba.appConfig.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func newBasicManager() module.BasicManager {
	return module.NewBasicManager(
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		staking.AppModuleBasic{},
	)
}
