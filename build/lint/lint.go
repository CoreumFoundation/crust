package lint

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

const (
	repoPath = "."
)

// Lint runs linters and check that git status is clean.
func Lint(ctx context.Context, deps types.DepsFunc) error {
	if err := typos(ctx, deps); err != nil {
		return err
	}

	if err := golang.Lint(ctx, deps); err != nil {
		return err
	}

	isClean, dirtyContent, err := git.StatusClean(ctx)
	if err != nil {
		return err
	}
	if !isClean {
		// fmt.Println is used intentionally here, because logger escapes special characters producing unreadable output
		fmt.Println("git status:")
		fmt.Println(dirtyContent)
		return errors.New("git status is not empty")
	}
	return nil
}

func typos(ctx context.Context, deps types.DepsFunc) error {
	deps(EnsureTypos)
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
