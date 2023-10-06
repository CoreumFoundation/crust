package coreum

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	cosmosSDKModule    = "github.com/cosmos/cosmos-sdk"
	cosmosIBCModule    = "github.com/cosmos/ibc-go/v7"
	cosmosProtoModule  = "github.com/cosmos/cosmos-proto"
	cosmWASMModule     = "github.com/CosmWasm/wasmd"
	gogoProtobufModule = "github.com/cosmos/gogoproto"
	grpcGatewayModule  = "github.com/grpc-ecosystem/grpc-gateway"
)

// Generate regenerates everything in coreum.
func Generate(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo, generateProtoDocs, generateProtoGo, generateProtoOpenAPI)

	return golang.Generate(ctx, repoPath, deps)
}

func moduleDirectories(ctx context.Context, deps build.DepsFunc) (map[string]string, error) {
	return golang.ModuleDirs(ctx, deps, repoPath,
		cosmosSDKModule,
		cosmosIBCModule,
		cosmWASMModule,
		cosmosProtoModule,
		gogoProtobufModule,
		grpcGatewayModule,
	)
}
