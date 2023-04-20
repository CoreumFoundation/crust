package coreum

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/golang"
)

// Generate regenerates everything in coreum.
func Generate(ctx context.Context, deps build.DepsFunc) error {
	deps(GenerateDeterministicGasSpec)
	return nil
}

// GenerateDeterministicGasSpec regenerates spec of deterministic gas.
func GenerateDeterministicGasSpec(ctx context.Context, deps build.DepsFunc) error {
	return golang.Generate(ctx, filepath.Join(repoPath, "x", "deterministicgas", "spec"), deps)
}
