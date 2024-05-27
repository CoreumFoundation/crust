package crust

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/gaia"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/hermes"
	"github.com/CoreumFoundation/crust/build/osmosis"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

const repoPath = "."

// BuildBuilder builds building tool in the current repository.
func BuildBuilder(ctx context.Context, deps types.DepsFunc) error {
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd",
		BinOutputPath:  must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

// BuildZNet builds znet.
func BuildZNet(ctx context.Context, deps types.DepsFunc) error {
	// FIXME (wojciech): Remove these deps once all the repos use znet programmatically
	deps(
		gaia.BuildDockerImage,
		osmosis.BuildDockerImage,
		hermes.BuildDockerImage,
	)

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "cmd/znet",
		BinOutputPath:  "bin/.cache/znet",
		CGOEnabled:     true,
	})
}

// Tidy runs `go mod tidy` for crust repo.
func Tidy(ctx context.Context, deps types.DepsFunc) error {
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints crust repo.
func Lint(ctx context.Context, deps types.DepsFunc) error {
	return golang.Lint(ctx, repoPath, deps)
}

// Test run unit tests in crust repo.
func Test(ctx context.Context, deps types.DepsFunc) error {
	return golang.Test(ctx, repoPath, deps)
}

// DownloadDependencies downloads go dependencies.
func DownloadDependencies(ctx context.Context, deps types.DepsFunc) error {
	return golang.DownloadDependencies(ctx, repoPath, deps)
}
