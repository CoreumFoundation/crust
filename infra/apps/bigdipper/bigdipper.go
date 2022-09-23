package bigdipper

import (
	"strconv"
	"time"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
)

const (
	// AppType is the type of big dipper application
	AppType infra.AppType = "bigdipper"

	// DefaultPort is the default port big dipper listens on for client connections
	DefaultPort = 3000
)

// Config stores big dipper app configuration
type Config struct {
	Name    string
	AppInfo *infra.AppInfo
	Port    int
	Cored   cored.Cored
	Hasura  hasura.Hasura
}

// New creates new big dipper app
func New(config Config) BigDipper {
	return BigDipper{
		config: config,
	}
}

// BigDipper represents big dipper
type BigDipper struct {
	config Config
}

// Type returns type of application
func (bd BigDipper) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (bd BigDipper) Name() string {
	return bd.config.Name
}

// Info returns deployment info
func (bd BigDipper) Info() infra.DeploymentInfo {
	return bd.config.AppInfo.Info()
}

// Deployment returns deployment of big dipper
func (bd BigDipper) Deployment() infra.Deployment {
	return infra.Deployment{
		// TODO: Get image from docker hub once it's there
		Image: "gcr.io/coreum-devnet-1/big-dipper-ui:latest-dev",
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "PORT",
					Value: strconv.Itoa(bd.config.Port),
				},
				{
					Name:  "NEXT_PUBLIC_URL",
					Value: infra.JoinNetAddr("http", "localhost", bd.config.Port),
				},
				{
					Name:  "NEXT_PUBLIC_RPC_WEBSOCKET",
					Value: infra.JoinNetAddr("ws", bd.config.Cored.Info().HostFromHost, bd.config.Cored.Ports().RPC) + "/websocket",
				},
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_URL",
					Value: infra.JoinNetAddr("http", bd.config.Hasura.Info().HostFromHost, bd.config.Hasura.Port()) + "/v1/graphql",
				},
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_WS",
					Value: infra.JoinNetAddr("ws", bd.config.Hasura.Info().HostFromHost, bd.config.Hasura.Port()) + "/v1/graphql",
				},
				{
					Name:  "NODE_ENV",
					Value: "development",
				},
				{
					Name:  "NEXT_PUBLIC_CHAIN_TYPE",
					Value: string(bd.config.Cored.Network().ChainID()),
				},
			}
		},
		Name: bd.Name(),
		Info: bd.config.AppInfo,
		Ports: map[string]int{
			"web": bd.config.Port,
		},
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				bd.config.Cored,
				infra.IsRunning(bd.config.Hasura),
			},
		},
	}
}
