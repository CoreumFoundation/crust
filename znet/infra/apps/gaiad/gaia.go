package gaiad

import (
	_ "embed"
	"text/template"

	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/cosmoschain"
)

const (
	dockerImage   = "gaiad:znet"
	accountPrefix = "cosmos"
	execName      = "gaiad"

	// AppType is the type of gaia application.
	AppType infra.AppType = "gaiad"

	// DefaultChainID is the gaia's default chain id.
	DefaultChainID = "gaia-localnet-1"

	// DefaultHomeName is the gaia's default home name.
	DefaultHomeName = ".gaia"

	// DefaultGasPriceStr defines default gas price to be used inside IBC relayer.
	DefaultGasPriceStr = "1.0uatom"
)

var (
	//go:embed run.tmpl
	tmpl string
	// RunScriptTemplate is gaia run.sh template.
	RunScriptTemplate = *template.Must(template.New("").Parse(tmpl))
)

// DefaultPorts are the default ports listens on.
var DefaultPorts = cosmoschain.Ports{
	RPC:     26557,
	P2P:     26556,
	GRPC:    9080,
	GRPCWeb: 9081,
	PProf:   6050,
}

// New creates new gaia blockchain.
func New(config cosmoschain.AppConfig) cosmoschain.BaseApp {
	return cosmoschain.New(cosmoschain.AppTypeConfig{
		AppType:       AppType,
		DockerImage:   dockerImage,
		AccountPrefix: accountPrefix,
		ExecName:      execName,
	}, config)
}
