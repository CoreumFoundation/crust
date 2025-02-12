package blockexplorer

import (
	"github.com/CoreumFoundation/crust/znet/infra"
	"github.com/CoreumFoundation/crust/znet/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/znet/infra/apps/callisto"
	"github.com/CoreumFoundation/crust/znet/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/znet/infra/apps/postgres"
)

// Ports defines ports used by applications required to run block explorer.
type Ports struct {
	Postgres          int
	Hasura            int
	Callisto          int
	CallistoTelemetry int
	BigDipper         int
}

// Explorer defines the struct of the aggregated block explorer components.
type Explorer struct {
	Postgres  postgres.Postgres
	Callisto  callisto.Callisto
	Hasura    hasura.Hasura
	BigDipper bigdipper.BigDipper
}

// ToAppSet build the AppSet from all explorer components.
func (e Explorer) ToAppSet() infra.AppSet {
	return infra.AppSet{
		e.Postgres,
		e.Callisto,
		e.Hasura,
		e.BigDipper,
	}
}
