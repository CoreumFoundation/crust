package gaia

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/tools"
)

// EnsureBinary installs gaiad binary to crust cache.
func EnsureBinary(ctx context.Context, deps build.DepsFunc) error {
	return tools.Ensure(ctx, tools.Gaia, tools.PlatformLocal)
}
