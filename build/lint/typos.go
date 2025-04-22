package lint

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

var (
	//go:embed "typos.toml"
	lintConfig []byte
)

func typosLint(ctx context.Context, deps types.DepsFunc) error {
	deps(ensureTypos)
	log := logger.Get(ctx)
	config := typosLintConfigPath()

	log.Info("Running typos linter", zap.String("path", repoPath))
	cmd := exec.Command(tools.Path("bin/typos", tools.TargetPlatformLocal), "--config", config, ".")
	cmd.Dir = repoPath
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "linter errors found in module '%s'", repoPath)
	}
	return nil
}

// ensureTypos ensures that typos linter is available.
func ensureTypos(ctx context.Context, deps types.DepsFunc) error {
	deps(storeLintConfig)
	return tools.Ensure(ctx, tools.TyposLint, tools.TargetPlatformLocal)
}

func typosLintConfigPath() string {
	return filepath.Join(tools.VersionedRootPath(tools.TargetPlatformLocal), "typos.toml")
}

func storeLintConfig(_ context.Context, _ types.DepsFunc) error {
	return errors.WithStack(os.WriteFile(typosLintConfigPath(), lintConfig, 0o600))
}
