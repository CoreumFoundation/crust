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

// DefaultXrplContractAddress is the address of instantiated contract by the first relayer (leader).
// according to this link:
// https://github.com/CosmWasm/wasmd/blob/04cb6e5408cc54c27247b0b327dfa99769d5103c/x/wasm/keeper/addresses.go#L18
// the address of instantiated contract is calculated by CodeID and the sequence number of instance, so the generated address
// is always as follows (if the logic of relayer and leader does not change).
// nolint:lll
const DefaultXrplContractAddress = "devcore14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sd4f0ak"
