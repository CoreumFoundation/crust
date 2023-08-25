package coreum

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/golang"
)

// Generate regenerates everything in coreum.
func Generate(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo, generateProtoDocs, generateProtoGo, generateProtoOpenAPI)

	return golang.Generate(ctx, repoPath, deps)
}
