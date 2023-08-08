package coreum

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/crust/build/tools"
)

func ensureCosmovisor(ctx context.Context, platform tools.Platform) error {
	t, err := tools.Get(tools.Cosmovisor)
	if err != nil {
		return err
	}
	if err := t.Ensure(ctx, platform); err != nil {
		return err
	}

	return tools.CopyToolBinaries(tools.Cosmovisor,
		platform,
		filepath.Join("bin", ".cache", "cored", platform.String()),
		cosmovisorBinaryPath)
}
