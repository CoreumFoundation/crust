package blockexplorer

import (
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
)

// DefaultPorts are the default ports applications building block explorer listen on.
var DefaultPorts = Ports{
	Postgres:  postgres.DefaultPort,
	Hasura:    hasura.DefaultPort,
	BDJuno:    bdjuno.DefaultPort,
	BigDipper: bigdipper.DefaultPort,
}
