package gaia

import (
	"context"

	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

// EnsureBinary installs gaiad binary to crust cache.
func EnsureBinary(ctx context.Context, deps types.DepsFunc) error {
	return tools.Ensure(ctx, tools.Gaia, tools.TargetPlatformLocal)
}
