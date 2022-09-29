package hasura

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
)

const (
	// AppType is the type of hasura application
	AppType infra.AppType = "hasura"

	// DefaultPort is the default port hasura listens on for client connections
	DefaultPort = 8080
)

// Config stores hasura app config
type Config struct {
	Name     string
	AppInfo  *infra.AppInfo
	Port     int
	Postgres postgres.Postgres
}

// New creates new hasura app
func New(config Config) Hasura {
	return Hasura{
		config: config,
	}
}

// Hasura represents hasura
type Hasura struct {
	config Config
}

// Type returns type of application
func (h Hasura) Type() infra.AppType {
	return AppType
}

// Name returns name of app
func (h Hasura) Name() string {
	return h.config.Name
}

// Port returns port used by hasura to accept client connections
func (h Hasura) Port() int {
	return h.config.Port
}

// Info returns deployment info
func (h Hasura) Info() infra.DeploymentInfo {
	return h.config.AppInfo.Info()
}

// Deployment returns deployment of hasura
func (h Hasura) Deployment() infra.Deployment {
	return infra.Deployment{
		Image: "gcr.io/coreum-devnet-1/hasura:v2.1.1.cli-migrations-v3",
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name: "HASURA_GRAPHQL_DATABASE_URL",
					Value: fmt.Sprintf("postgres://%s@%s/%s", postgres.User,
						infra.JoinNetAddr("", h.config.Postgres.Info().HostFromContainer, h.config.Postgres.Port()),
						postgres.DB),
				},
				{
					Name:  "HASURA_GRAPHQL_SERVER_PORT",
					Value: strconv.Itoa(h.config.Port),
				},
			}
		},
		Name: h.Name(),
		Info: h.config.AppInfo,
		Ports: map[string]int{
			"server": h.config.Port,
		},
		Requires: infra.Prerequisites{
			Timeout: 20 * time.Second,
			Dependencies: []infra.HealthCheckCapable{
				h.config.Postgres,
			},
		},
	}
}
