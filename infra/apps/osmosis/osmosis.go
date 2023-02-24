package osmosis

import (
	_ "embed"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/cosmos"
)

const (
	dockerImage   = "osmolabs/osmosis:14.0.0-alpine"
	accountPrefix = "osmo"
	execName      = "osmosisd"

	// AppType is the type of osmosis application.
	AppType infra.AppType = "osmosis"

	// DefaultChainID is the osmosis default chain id.
	DefaultChainID = "osmosis-localnet-1"
)

// DefaultPorts are the default ports listens on.
var DefaultPorts = cosmos.Ports{
	RPC:     26457,
	P2P:     26456,
	GRPC:    9070,
	GRPCWeb: 9071,
	PProf:   6040,
}

// New creates new osmosis blockchain
func New(config cosmos.AppConfig) cosmos.BaseApp {
	return cosmos.New(cosmos.AppTypeConfig{
		AppType:       AppType,
		DockerImage:   dockerImage,
		AccountPrefix: accountPrefix,
		ExecName:      execName,
	}, config)
}
