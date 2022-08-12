package bdjuno

import (
	"bytes"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/infra/targets"
)

const (
	// AppType is the type of bdjuno application
	AppType infra.AppType = "bdjuno"

	// DefaultPort is the default port bdjuno listens on for client connections
	DefaultPort = 3030
)

// Config storesbdjuno app configuration
type Config struct {
	Name           string
	HomeDir        string
	AppInfo        *infra.AppInfo
	Port           int
	ConfigTemplate string
	Cored          cored.Cored
	Postgres       postgres.Postgres
}

// New creates new bdjuno app
func New(config Config) BDJuno {
	return BDJuno{
		config: config,
	}
}

// BDJuno represents bdjuno
type BDJuno struct {
	config Config
}

// Type returns type of application
func (j BDJuno) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (j BDJuno) Name() string {
	return j.config.Name
}

// Port returns port used by hasura to accept client connections
func (j BDJuno) Port() int {
	return j.config.Port
}

// Info returns deployment info
func (j BDJuno) Info() infra.DeploymentInfo {
	return j.config.AppInfo.Info()
}

// Deployment returns deployment of bdjuno
func (j BDJuno) Deployment() infra.Deployment {
	return infra.Container{
		Image: "gcr.io/coreum-devnet-1/bdjuno:0.44.0",
		AppBase: infra.AppBase{
			Name: j.Name(),
			Info: j.config.AppInfo,
			ArgsFunc: func() []string {
				return []string{
					"bdjuno", "start",
					"--home", targets.AppHomeDir,
				}
			},
			Ports: map[string]int{
				"actions": j.config.Port,
			},
			Requires: infra.Prerequisites{
				Timeout: 20 * time.Second,
				Dependencies: []infra.HealthCheckCapable{
					j.config.Cored,
					j.config.Postgres,
				},
			},
			PrepareFunc: func() error {
				return ioutil.WriteFile(j.config.HomeDir+"/config.yaml", j.prepareConfig(), 0o644)
			},
		},
	}
}

func (j BDJuno) prepareConfig() []byte {
	configBuf := &bytes.Buffer{}
	must.OK(template.Must(template.New("config").Parse(j.config.ConfigTemplate)).Execute(configBuf, struct {
		Port  int
		Cored struct {
			Host          string
			PortRPC       int
			PortGRPC      int
			AddressPrefix string
		}
		Postgres struct {
			Host string
			Port int
			User string
			DB   string
		}
	}{
		Port: j.config.Port,
		Cored: struct {
			Host          string
			PortRPC       int
			PortGRPC      int
			AddressPrefix string
		}{
			Host:          j.config.Cored.Info().HostFromContainer,
			PortRPC:       j.config.Cored.Ports().RPC,
			PortGRPC:      j.config.Cored.Ports().GRPC,
			AddressPrefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		Postgres: struct {
			Host string
			Port int
			User string
			DB   string
		}{
			Host: j.config.Postgres.Info().HostFromContainer,
			Port: j.config.Postgres.Port(),
			User: postgres.User,
			DB:   postgres.DB,
		},
	}))
	return configBuf.Bytes()
}
