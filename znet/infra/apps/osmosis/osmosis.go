package osmosis

import (
	_ "embed"
	"text/template"

	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/cosmoschain"
)

const (
	dockerImage   = "osmosisd:znet"
	accountPrefix = "osmo"
	execName      = "osmosisd"

	// AppType is the type of osmosis application.
	AppType infra.AppType = "osmosis"

	// DefaultChainID is the osmosis default chain id.
	DefaultChainID = "osmosis-localnet-1"

	// DefaultHomeName is the gaia's default home name.
	DefaultHomeName = ".osmosisd"

	// DefaultGasPriceStr defines default gas price to be used inside IBC relayer.
	DefaultGasPriceStr = "0.1uosmo"
)

var (
	//go:embed run.tmpl
	tmpl string
	// RunScriptTemplate is osmosis run.sh template.
	RunScriptTemplate = *template.Must(template.New("").Parse(tmpl))
)

// DefaultPorts are the default ports listens on.
var DefaultPorts = cosmoschain.Ports{
	RPC:     26457,
	P2P:     26456,
	GRPC:    9070,
	GRPCWeb: 9071,
	PProf:   6040,
}

// New creates new osmosis blockchain.
func New(config cosmoschain.AppConfig) cosmoschain.BaseApp {
	return cosmoschain.New(cosmoschain.AppTypeConfig{
		AppType:       AppType,
		DockerImage:   dockerImage,
		AccountPrefix: accountPrefix,
		ExecName:      execName,
	}, config)
}
