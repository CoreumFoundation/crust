package postgres

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/crust/infra"
)

const (
	// AppType is the type of postgres application.
	AppType infra.AppType = "postgres"

	// DefaultPort is the default port postgres listens on for client connections.
	DefaultPort = 5432

	// User contains the login of superuser.
	User = "postgres"

	// DB is the name of database.
	DB = "db"
)

// SchemaLoaderFunc is the function receiving sql client and loading schema there.
type SchemaLoaderFunc func(ctx context.Context, db *pgx.Conn) error

// Config stores configuration of postgres app.
type Config struct {
	Name    string
	AppInfo *infra.AppInfo
	Port    int
}

// New creates new postgres app.
func New(config Config) Postgres {
	return Postgres{
		config: config,
	}
}

// Postgres represents postgres.
type Postgres struct {
	config Config
}

// Type returns type of application.
func (p Postgres) Type() infra.AppType {
	return AppType
}

// Name returns name of app.
func (p Postgres) Name() string {
	return p.config.Name
}

// Port returns port used by postgres to accept client connections.
func (p Postgres) Port() int {
	return p.config.Port
}

// Info returns deployment info.
func (p Postgres) Info() infra.DeploymentInfo {
	return p.config.AppInfo.Info()
}

// HealthCheck checks if postgres is ready to accept connections.
func (p Postgres) HealthCheck(ctx context.Context) error {
	if p.config.AppInfo.Info().Status != infra.AppStatusRunning {
		return retry.Retryable(errors.Errorf("postgres hasn't started yet"))
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	connStr := "postgres://" +
		User +
		"@" +
		infra.JoinNetAddr("", p.config.AppInfo.Info().HostFromHost, p.config.Port) +
		"/" +
		DB
	db, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return retry.Retryable(errors.WithStack(err))
	}

	if err := db.Ping(ctx); err != nil {
		return errors.WithStack(err)
	}

	time.Sleep(10 * time.Second)

	return retry.Retryable(errors.WithStack(db.Close(ctx)))
}

// Deployment returns deployment of postgres.
func (p Postgres) Deployment() infra.Deployment {
	return infra.Deployment{
		Image: "postgres:16.1-alpine3.18",
		EnvVarsFunc: func() []infra.EnvVar {
			return []infra.EnvVar{
				{
					Name:  "POSTGRES_USER",
					Value: User,
				},
				{
					Name:  "POSTGRES_DB",
					Value: DB,
				},

				// This allows to log in using any existing user (even superuser) without providing a password.
				// This is local, temporary development setup so security doesn't matter.
				{
					Name:  "POSTGRES_HOST_AUTH_METHOD",
					Value: "trust",
				},
			}
		},
		Name: p.Name(),
		Info: p.config.AppInfo,
		ArgsFunc: func() []string {
			return []string{
				"-h", net.IPv4zero.String(),
				"-p", strconv.Itoa(p.config.Port),
			}
		},
		Ports: map[string]int{
			"sql": p.config.Port,
		},
	}
}
