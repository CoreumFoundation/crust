package blockexplorer

import (
	"github.com/CoreumFoundation/crust/znet/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/znet/infra/apps/callisto"
	"github.com/CoreumFoundation/crust/znet/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/znet/infra/apps/postgres"
)

// DefaultPorts are the default ports applications building block explorer listen on.
var DefaultPorts = Ports{
	Postgres:          postgres.DefaultPort,
	Hasura:            hasura.DefaultPort,
	Callisto:          callisto.DefaultPort,
	CallistoTelemetry: callisto.DefaultTelemetryPort,
	BigDipper:         bigdipper.DefaultPort,
}
