package crust

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/hermes"
	"github.com/CoreumFoundation/crust/build/osmosis"
	"github.com/CoreumFoundation/crust/build/tools"
)

const repoPath = "."

// BuildCrust builds crust.
func BuildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.Build(ctx, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd",
		BinOutputPath:  must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

// BuildZNet builds znet.
func BuildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(
		golang.EnsureGo,
		coreum.BuildCored,
		gaia.EnsureBinary,
		gaia.BuildDockerImage,
		osmosis.BuildDockerImage,
		hermes.BuildDockerImage,
	)

	return golang.Build(ctx, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "cmd/znet",
		BinOutputPath:  "bin/.cache/znet",
		CGOEnabled:     true,
	})
}

// Tidy runs `go mod tidy` for crust repo.
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints crust repo.
func Lint(ctx context.Context, deps build.DepsFunc) error {
	return golang.Lint(ctx, repoPath, deps)
}

// LintCurrentDir lints current dir.
func LintCurrentDir(ctx context.Context, deps build.DepsFunc) error {
	path := os.Getenv("SOURCE_DIR")
	if path == "" {
		return errors.New("can't get current dir for linting")
	}

	return golang.Lint(ctx, path, deps)
}

// Test run unit tests in crust repo.
func Test(ctx context.Context, deps build.DepsFunc) error {
	return golang.Test(ctx, repoPath, deps)
}
