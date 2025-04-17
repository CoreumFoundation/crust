package lint

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

var (
	//go:embed "typos.toml"
	lintConfig []byte
)

// EnsureTypos ensures that typos linter is available.
func EnsureTypos(ctx context.Context, deps types.DepsFunc) error {
	deps(storeLintConfig)
	return tools.Ensure(ctx, tools.TyposLint, tools.TargetPlatformLocal)
}

func typosLintConfigPath() string {
	return filepath.Join(tools.VersionedRootPath(tools.TargetPlatformLocal), "typos.toml")
}

func storeLintConfig(_ context.Context, _ types.DepsFunc) error {
	return errors.WithStack(os.WriteFile(typosLintConfigPath(), lintConfig, 0o600))
}
