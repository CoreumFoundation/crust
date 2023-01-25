package crust

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/golang"
)

const repoPath = "."

// BuildCrust builds crust.
func BuildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "build/cmd",
		BinOutputPath: must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

// BuildZNet builds znet.
func BuildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "cmd/znet",
		BinOutputPath: "bin/.cache/znet",
		CGOEnabled:    true,
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

// Test run unit tests in crust repo.
func Test(ctx context.Context, deps build.DepsFunc) error {
	return golang.Test(ctx, repoPath, deps)
}
