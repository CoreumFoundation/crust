package bdjuno

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	repoURL    = "https://github.com/CoreumFoundation/bdjuno.git"
	repoPath   = "../bdjuno"
	binaryName = "bdjuno"
	binaryPath = "bin/" + binaryName
)

// Build builds faucet in docker.
func Build(ctx context.Context, deps build.DepsFunc) error {
	return buildBDJuno(ctx, deps, tools.TargetPlatformLinuxLocalArchInDocker)
}

func buildBDJuno(ctx context.Context, deps build.DepsFunc, targetPlatform tools.TargetPlatform) error {
	deps(ensureRepo)

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: targetPlatform,
		PackagePath:    filepath.Join(repoPath, "cmd", "bdjuno"),
		BinOutputPath:  filepath.Join("bin", ".cache", binaryName, targetPlatform.String(), "bin", binaryName),
	})
}

func ensureRepo(ctx context.Context, deps build.DepsFunc) error {
	return git.EnsureRepo(ctx, repoURL)
}
