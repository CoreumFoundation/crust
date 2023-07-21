package infra

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/CoreumFoundation/coreum/v2/pkg/client"
)

// HealthCheckCapable represents application exposing health check endpoint.
type HealthCheckCapable interface {
	// Name returns name of app
	Name() string

	// HealthCheck runs single health check
	HealthCheck(ctx context.Context) error
}

// WaitUntilHealthy waits until app is healthy or context expires.
func WaitUntilHealthy(ctx context.Context, apps ...HealthCheckCapable) error {
	for _, app := range apps {
		app := app
		ctx = logger.With(ctx, zap.String("app", app.Name()))
		if err := retry.Do(ctx, time.Second, func() error {
			return app.HealthCheck(ctx)
		}); err != nil {
			return err
		}
	}
	return nil
}

// AppWithInfo represents application which is able to return information about its deployment.
type AppWithInfo interface {
	// Name returns name of app
	Name() string

	// Info returns information about application's deployment
	Info() DeploymentInfo
}

// IsRunning returns a health check which succeeds if application is running.
func IsRunning(app AppWithInfo) HealthCheckCapable {
	return isRunningHealthCheck{app: app}
}

type isRunningHealthCheck struct {
	app AppWithInfo
}

// Name returns name of app.
func (hc isRunningHealthCheck) Name() string {
	return hc.app.Name()
}

// HealthCheck runs single health check.
func (hc isRunningHealthCheck) HealthCheck(ctx context.Context) error {
	if hc.app.Info().Status == AppStatusRunning {
		return nil
	}
	return retry.Retryable(errors.New("application hasn't been started yet"))
}

// JoinNetAddr joins protocol, hostname and port.
func JoinNetAddr(proto, hostname string, port int) string {
	if proto != "" {
		proto += "://"
	}
	return proto + net.JoinHostPort(hostname, strconv.Itoa(port))
}

// JoinNetAddrIP joins protocol, IP and port.
func JoinNetAddrIP(proto string, ip net.IP, port int) string {
	return JoinNetAddr(proto, ip.String(), port)
}

// PortsToMap converts structure containing port numbers to a map.
func PortsToMap(ports interface{}) map[string]int {
	unmarshaled := map[string]interface{}{}
	must.OK(json.Unmarshal(must.Bytes(json.Marshal(ports)), &unmarshaled))

	res := map[string]int{}
	for k, v := range unmarshaled {
		res[k] = int(v.(float64))
	}
	return res
}

// CheckCosmosNodeHealth check the health of the running cosmos based node.
func CheckCosmosNodeHealth(ctx context.Context, clientCtx client.Context, appInfo DeploymentInfo) error {
	if appInfo.Status != AppStatusRunning {
		return retry.Retryable(errors.New("chain hasn't started yet"))
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Test RPC endpoint
	status, err := clientCtx.RPCClient().Status(ctx)
	if err != nil {
		return retry.Retryable(errors.Wrap(err, "retrieving node status failed"))
	}

	if status.SyncInfo.LatestBlockHeight == 0 {
		return retry.Retryable(errors.New("genesis block hasn't been mined yet"))
	}

	// Test GRPC endpoint
	tmClient := tmservice.NewServiceClient(clientCtx)
	_, err = tmClient.GetNodeInfo(ctx, &tmservice.GetNodeInfoRequest{})
	if err != nil {
		return retry.Retryable(errors.Wrap(err, "grpc endpoint is not ready yet"))
	}

	return nil
}
