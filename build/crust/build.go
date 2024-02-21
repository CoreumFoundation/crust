package crust

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/bdjuno"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/hermes"
	"github.com/CoreumFoundation/crust/build/osmosis"
	"github.com/CoreumFoundation/crust/build/tools"
)

const repoPath = "."

// BuildBuilder builds building tool in the current repository.
func BuildBuilder(ctx context.Context, deps build.DepsFunc) error {
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd",
		Flags:          []string{"-o " + must.String(filepath.EvalSymlinks(must.String(os.Executable())))},
	})
}

// BuildZNet builds znet.
func BuildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(
		bdjuno.BuildDockerImage,
		coreum.BuildCoredDockerImage,
		gaia.BuildDockerImage,
		osmosis.BuildDockerImage,
		hermes.BuildDockerImage,
	)

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "cmd/znet",
		Flags:          []string{"-o " + "bin/.cache/znet"},
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
