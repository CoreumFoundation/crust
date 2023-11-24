package bigdipper

import (
	"strconv"
	"time"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
)

const (
	// AppType is the type of big dipper application.
	AppType infra.AppType = "bigdipper"

	// DefaultPort is the default port big dipper listens on for client connections.
	DefaultPort = 3000
)

// Config stores big dipper app configuration.
type Config struct {
	Name    string
	AppInfo *infra.AppInfo
	Port    int
	Cored   cored.Cored
	Hasura  hasura.Hasura
}

// New creates new big dipper app.
func New(config Config) BigDipper {
	return BigDipper{
		config: config,
	}
}

// BigDipper represents big dipper.
type BigDipper struct {
	config Config
}

// Type returns type of application.
func (bd BigDipper) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (bd BigDipper) Name() string {
	return bd.config.Name
}

// Info returns deployment info.
func (bd BigDipper) Info() infra.DeploymentInfo {
	return bd.config.AppInfo.Info()
}

// Deployment returns deployment of big dipper.
func (bd BigDipper) Deployment() infra.Deployment {
	return infra.Deployment{
		Image: "coreumfoundation/big-dipper-ui:latest",
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_URL",
					Value: "http://localhost:8080/v1/graphql",
				},
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_WS",
					Value: "ws://localhost:8080/v1/graphql",
				},
				{
					Name:  "NEXT_PUBLIC_RPC_WEBSOCKET",
					Value: "ws://localhost:26657/websocket",
				},
				{
					Name:  "NEXT_PUBLIC_CHAIN_TYPE",
					Value: "devnet",
				},
				{
					Name:  "PORT",
					Value: strconv.Itoa(bd.config.Port),
				},
			}
		},

		Name: bd.Name(),
		Info: bd.config.AppInfo,
		Ports: map[string]int{
			"web": bd.config.Port,
		},
		Requires: infra.Prerequisites{
			Timeout: 40 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				bd.config.Cored,
				infra.IsRunning(bd.config.Hasura),
			},
		},
	}
}
