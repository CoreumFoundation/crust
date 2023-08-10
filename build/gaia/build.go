package gaia

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Build installs gaiad binary.
func Build(ctx context.Context, deps build.DepsFunc) error {
	deps(tools.EnsureGaiad)
	return nil
}
