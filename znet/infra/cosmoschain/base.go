package cosmoschain

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"text/template"
	"time"

	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v6/pkg/client"
	"github.com/CoreumFoundation/coreum/v6/pkg/config"
	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/targets"
	"github.com/CoreumFoundation/crust/znet/pkg/tools"
)

const dockerEntrypoint = "run.sh"

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
	Name              string
	HomeDir           string
	HomeName          string
	ChainID           string
	AppInfo           *infra.AppInfo
	Ports             Ports
	RelayerMnemonic   string
	FundingMnemonic   string
	TimeoutCommit     time.Duration
	WrapperDir        string
	GasPriceStr       string
	RunScriptTemplate template.Template
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
	rpcClient, err := cosmosclient.NewClientFromNode(
		infra.JoinNetAddr("http", ba.Info().HostFromHost, ba.appConfig.Ports.RPC),
	)
	must.OK(err)

	grpcClient, err := GRPCClient(infra.JoinNetAddr("", ba.Info().HostFromHost, ba.appConfig.Ports.GRPC))
	must.OK(err)

	return client.NewContext(client.DefaultContextConfig(), moduleBasicList...).
		WithChainID(ba.appConfig.ChainID).
		WithRPCClient(rpcClient).
		WithGRPCClient(grpcClient)
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
		ConfigureFunc: func(ctx context.Context, deployment infra.DeploymentInfo) error {
			return ba.saveClientWrapper(deployment.HostFromHost)
		},
	}
}

func (ba BaseApp) prepare(_ context.Context) error {
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
	if err := ba.appConfig.RunScriptTemplate.Execute(buf, args); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(ba.appConfig.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// GRPCClient prepares GRPC client.
func GRPCClient(url string) (*grpc.ClientConn, error) {
	encodingConfig := config.NewEncodingConfig(moduleBasicList...)
	pc, ok := encodingConfig.Codec.(codec.GRPCCodecProvider)
	if !ok {
		return nil, errors.New("failed to cast codec to codec.GRPCCodecProvider)")
	}

	grpClient, err := grpc.NewClient(
		url,
		grpc.WithDefaultCallOptions(grpc.ForceCodec(pc.GRPCCodec())),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return grpClient, nil
}

var moduleBasicList = []module.AppModuleBasic{
	auth.AppModuleBasic{},
	bank.AppModuleBasic{},
	staking.AppModuleBasic{},
}

func (ba BaseApp) saveClientWrapper(hostname string) error {
	baClient := fmt.Sprintf(`#!/bin/bash
OPTS=""
if [ "$1" == "tx" ] || [ "$1" == "keys" ]; then
	OPTS="$OPTS --keyring-backend ""test"""
fi

if [ "$1" == "status" ] || [ "$1" == "tx" ] || [ "$1" == "q" ]; then
	OPTS="$OPTS --node ""%s"""
fi

exec %s --home %s "$@" $OPTS
`,
		infra.JoinNetAddr("tcp", hostname, ba.appConfig.Ports.RPC),                                   // rpc endpoint
		filepath.Join(tools.BinariesRootPath(tools.PlatformLocal), "bin", ba.appTypeConfig.ExecName), // client's path
		filepath.Dir(ba.appConfig.HomeDir),                                                           // home dir
	)

	return errors.WithStack(
		os.WriteFile(filepath.Join(ba.appConfig.WrapperDir, ba.appConfig.Name), []byte(baClient), 0o700),
	)
}
