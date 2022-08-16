package hasura

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
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
	Name             string
	AppInfo          *infra.AppInfo
	Port             int
	MetadataTemplate string
	Postgres         postgres.Postgres
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
	return infra.Container{
		Image: "hasura/graphql-engine:v2.8.0",
		AppBase: infra.AppBase{
			Name: h.Name(),
			Info: h.config.AppInfo,
			ArgsFunc: func() []string {
				return []string{
					"graphql-engine",
					"--host", h.config.Postgres.Info().HostFromContainer,
					"--port", strconv.Itoa(h.config.Postgres.Port()),
					"--user", postgres.User,
					"--dbname", postgres.DB,
					"serve",
					"--server-host", net.IPv4zero.String(),
					"--server-port", strconv.Itoa(h.config.Port),
					"--enable-console",
					"--dev-mode",
					"--enabled-log-types", "startup,http-log,webhook-log,websocket-log,query-log",
				}
			},
			Ports: map[string]int{
				"server": h.config.Port,
			},
			Requires: infra.Prerequisites{
				Timeout: 20 * time.Second,
				Dependencies: []infra.HealthCheckCapable{
					h.config.Postgres,
				},
			},
			ConfigureFunc: func(ctx context.Context, deployment infra.DeploymentInfo) error {
				metadata := h.prepareMetadata()
				metaURL := url.URL{Scheme: "http", Host: infra.JoinNetAddr("", deployment.HostFromHost, h.config.Port), Path: "/v1/metadata"}

				log := logger.Get(ctx)
				log.Info("Loading metadata")

				if err := postMetadata(ctx, metadata, metaURL.String()); err != nil {
					return err
				}

				log.Info("Metadata loaded")
				return nil
			},
		},
	}
}

func (h Hasura) prepareMetadata() []byte {
	metadataBuf := &bytes.Buffer{}
	must.OK(template.Must(template.New("metadata").Parse(h.config.MetadataTemplate)).Execute(metadataBuf, struct {
		DatabaseURL string
	}{
		DatabaseURL: "postgresql://" + postgres.User + "@" + infra.JoinNetAddr("", h.config.Postgres.Info().HostFromContainer, h.config.Postgres.Port()) + "/" + postgres.DB,
	}))
	reqData := struct {
		Type    string          `json:"type"`
		Version uint            `json:"version"`
		Args    json.RawMessage `json:"args"`
	}{
		Type:    "replace_metadata",
		Version: 2,
		Args:    metadataBuf.Bytes(),
	}

	return must.Bytes(json.Marshal(reqData))
}

func postMetadata(ctx context.Context, metadata []byte, url string) error {
	retryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return retry.Do(retryCtx, 2*time.Second, func() error {
		requestCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, url, bytes.NewReader(metadata))
		must.OK(err)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("X-Hasura-Role", "admin")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return retry.Retryable(errors.Wrap(err, "request to store hasura metadata failed"))
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return retry.Retryable(errors.Wrapf(err, "reading body failed"))
		}
		return errors.Errorf("request to store hasura metadata failed with status code %d, body: %s", resp.StatusCode, body)
	})
}
