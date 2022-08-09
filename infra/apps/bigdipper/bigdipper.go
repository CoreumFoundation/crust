package bigdipper

import (
	"bytes"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"

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
func New(name string, config infra.Config, appInfo *infra.AppInfo, port int, envTemplate string, cored cored.Cored, hasura hasura.Hasura) BigDipper {
	return BigDipper{
		name:        name,
		homeDir:     config.AppDir + "/" + name,
		appInfo:     appInfo,
		envTemplate: envTemplate,
		port:        port,
		cored:       cored,
		hasura:      hasura,
	}
}

// BigDipper represents big dipper
type BigDipper struct {
	name        string
	homeDir     string
	appInfo     *infra.AppInfo
	envTemplate string
	port        int
	cored       cored.Cored
	hasura      hasura.Hasura
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
		Volumes: []infra.Volume{
			{
				From: bd.homeDir + "/env",
				To:   "/.env",
			},
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
			PrepareFunc: func() error {
				return ioutil.WriteFile(bd.homeDir+"/env", bd.prepareEnv(), 0o644)
			},
		},
	}
}

func (bd BigDipper) prepareEnv() []byte {
	envBuf := &bytes.Buffer{}
	must.OK(template.Must(template.New("env").Parse(bd.envTemplate)).Execute(envBuf, struct {
		Port  int
		URL   string
		Cored struct {
			Host    string
			PortRPC int
		}
		Hasura struct {
			Host string
			Port int
		}
	}{
		Port: bd.port,
		URL:  infra.JoinNetAddr("http", "localhost", bd.port),
		Cored: struct {
			Host    string
			PortRPC int
		}{
			Host:    bd.cored.Info().HostFromHost,
			PortRPC: bd.cored.Ports().RPC,
		},
		Hasura: struct {
			Host string
			Port int
		}{
			Host: bd.hasura.Info().HostFromHost,
			Port: bd.hasura.Port(),
		},
	}))
	return envBuf.Bytes()
}
