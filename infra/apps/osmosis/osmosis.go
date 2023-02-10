package osmosis

import (
	"bytes"
	"context"
	_ "embed"
	"net"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/targets"
)

var (
	//go:embed run.tmpl
	tmpl              string
	runScriptTemplate = template.Must(template.New("").Parse(tmpl))
)

const (
	// AppType is the type of osmosis application.
	AppType infra.AppType = "osmosis"
	// DefaultChainID is the osmosis default chain id.
	DefaultChainID = "osmosis-localnet-1"
	// DefaultAccountPrefix is the account prefix used in the osmosis.
	DefaultAccountPrefix = "osmo"

	dockerEntrypoint = "run.sh"
)

// Ports defines ports used by application.
type Ports struct {
	RPC     int `json:"rpc"`
	P2P     int `json:"p2p"`
	GRPC    int `json:"grpc"`
	GRPCWeb int `json:"grpcWeb"`
	PProf   int `json:"pprof"`
}

// DefaultPorts are the default ports listens on.
var DefaultPorts = Ports{
	RPC:     26457,
	P2P:     26456,
	GRPC:    9070,
	GRPCWeb: 9071,
	PProf:   6040,
}

// Config stores osmosis app config.
type Config struct {
	Name            string
	HomeDir         string
	ChainID         string
	AccountPrefix   string
	AppInfo         *infra.AppInfo
	Ports           Ports
	RelayerMnemonic string
}

// New creates new osmosis app.
func New(config Config) Osmosis {
	return Osmosis{
		config: config,
	}
}

// Osmosis represents osmosis.
type Osmosis struct {
	config Config
}

// Type returns type of application.
func (o Osmosis) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (o Osmosis) Name() string {
	return o.config.Name
}

// Ports returns port used by the application.
func (o Osmosis) Ports() Ports {
	return o.config.Ports
}

// Info returns deployment info.
func (o Osmosis) Info() infra.DeploymentInfo {
	return o.config.AppInfo.Info()
}

// HealthCheck checks if chain is ready.
func (o Osmosis) HealthCheck(ctx context.Context) error {
	return infra.CheckCosmosNodeHealth(ctx, o.Info(), o.config.Ports.RPC)
}

// Deployment returns deployment.
func (o Osmosis) Deployment() infra.Deployment {
	return infra.Deployment{
		RunAsUser: true,
		Image:     "osmolabs/osmosis:14.0.0-alpine",
		Name:      o.Name(),
		Info:      o.config.AppInfo,
		Volumes: []infra.Volume{
			{
				Source:      o.config.HomeDir,
				Destination: targets.AppHomeDir,
			},
		},
		Ports:       infra.PortsToMap(o.config.Ports),
		PrepareFunc: o.prepare,
		Entrypoint:  filepath.Join(targets.AppHomeDir, dockerEntrypoint),
	}
}

// Config returns the config.
func (o Osmosis) Config() Config {
	return o.config
}

func (o Osmosis) prepare() error {
	args := struct {
		HomePath        string
		ChainID         string
		RelayerMnemonic string
		RPCLaddr        string
		P2PLaddr        string
		GRPCAddress     string
		GRPCWebAddress  string
		RPCPprofLaddr   string
	}{
		HomePath:        targets.AppHomeDir,
		ChainID:         o.config.ChainID,
		RelayerMnemonic: RelayerMnemonic,
		RPCLaddr:        infra.JoinNetAddrIP("tcp", net.IPv4zero, o.config.Ports.RPC),
		P2PLaddr:        infra.JoinNetAddrIP("tcp", net.IPv4zero, o.config.Ports.P2P),
		GRPCAddress:     infra.JoinNetAddrIP("", net.IPv4zero, o.config.Ports.GRPC),
		GRPCWebAddress:  infra.JoinNetAddrIP("", net.IPv4zero, o.config.Ports.GRPCWeb),
		RPCPprofLaddr:   infra.JoinNetAddrIP("", net.IPv4zero, o.config.Ports.PProf),
	}

	buf := &bytes.Buffer{}
	if err := runScriptTemplate.Execute(buf, args); err != nil {
		return errors.WithStack(err)
	}

	err := os.WriteFile(path.Join(o.config.HomeDir, dockerEntrypoint), buf.Bytes(), 0o777)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
