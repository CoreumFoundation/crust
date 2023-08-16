package osmosis

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/tools"
)

// EnsureBinary installs osmosis binary to crust cache.
func EnsureBinary(ctx context.Context, deps build.DepsFunc) error {
	return tools.EnsureBinaries(ctx, tools.Osmosis, tools.PlatformLocal)
}
