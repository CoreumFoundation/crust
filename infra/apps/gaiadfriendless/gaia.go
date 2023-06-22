package gaiadfriendless

import (
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
)

const (
	dockerImage   = "gaiad:znet"
	accountPrefix = "cosmos"
	execName      = "gaiad"

	// AppType is the type of gaia application.
	AppType infra.AppType = "gaiad-friendless"

	// DefaultChainID is the gaia's default chain id.
	DefaultChainID = "gaia-friendless-1"
)

// DefaultPorts are the default ports listens on.
var DefaultPorts = cosmoschain.Ports{
	RPC:     26357,
	P2P:     26356,
	GRPC:    9060,
	GRPCWeb: 9061,
	PProf:   6030,
}

// New creates new gaia friendless blockchain.
func New(config cosmoschain.AppConfig) cosmoschain.BaseApp {
	return cosmoschain.New(cosmoschain.AppTypeConfig{
		AppType:       AppType,
		DockerImage:   dockerImage,
		AccountPrefix: accountPrefix,
		ExecName:      execName,
	}, config)
}
