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

// New creates new big dipper app
func New(name string, config infra.Config, appInfo *infra.AppInfo, port int, cored cored.Cored, hasura hasura.Hasura) BigDipper {
	return BigDipper{
		name:    name,
		appInfo: appInfo,
		port:    port,
		cored:   cored,
		hasura:  hasura,
	}
}

// BigDipper represents big dipper
type BigDipper struct {
	name    string
	appInfo *infra.AppInfo
	port    int
	cored   cored.Cored
	hasura  hasura.Hasura
}

// Type returns type of application
func (bd BigDipper) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (bd BigDipper) Name() string {
	return bd.name
}

// Info returns deployment info
func (bd BigDipper) Info() infra.DeploymentInfo {
	return bd.appInfo.Info()
}

// Deployment returns deployment of big dipper
func (bd BigDipper) Deployment() infra.Deployment {
	return infra.Container{
		Image: "gcr.io/coreum-devnet-1/big-dipper-ui:latest-dev",
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "PORT",
					Value: strconv.Itoa(bd.port),
				},
				{
					Name:  "NEXT_PUBLIC_URL",
					Value: infra.JoinNetAddr("http", "localhost", bd.port),
				},
				{
					Name:  "NEXT_PUBLIC_RPC_WEBSOCKET",
					Value: infra.JoinNetAddr("ws", bd.cored.Info().HostFromHost, bd.cored.Ports().RPC) + "/websocket",
				},
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_URL",
					Value: infra.JoinNetAddr("http", bd.hasura.Info().HostFromHost, bd.hasura.Port()) + "/v1/graphql",
				},
				{
					Name:  "NEXT_PUBLIC_GRAPHQL_WS",
					Value: infra.JoinNetAddr("ws", bd.hasura.Info().HostFromHost, bd.hasura.Port()) + "/v1/graphql",
				},
				{
					Name:  "NODE_ENV",
					Value: "development",
				},
				{
					Name:  "NEXT_PUBLIC_CHAIN_TYPE",
					Value: string(bd.cored.Network().ChainID()),
				},
			}
		},
		AppBase: infra.AppBase{
			Name: bd.Name(),
			Info: bd.appInfo,
			Ports: map[string]int{
				"web": bd.port,
			},
			Requires: infra.Prerequisites{
				Timeout: 20 * time.Second,
				Dependencies: []infra.HealthCheckCapable{
					bd.cored,
					infra.IsRunning(bd.hasura),
				},
			},
		},
	}
}
