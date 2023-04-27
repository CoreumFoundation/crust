package faucet

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	repoURL        = "https://github.com/CoreumFoundation/faucet.git"
	repoPath       = "../faucet"
	binaryName     = "faucet"
	binaryPath     = "bin/" + binaryName
	testBinaryPath = "bin/.cache/integration-tests/faucet"
)

// Build builds faucet in docker.
func Build(ctx context.Context, deps build.DepsFunc) error {
	return buildFaucet(ctx, deps, tools.PlatformDockerLocal)
}

func buildFaucet(ctx context.Context, deps build.DepsFunc, platform tools.Platform) error {
	deps(golang.EnsureGo, ensureRepo)

	config := golang.BinaryBuildConfig{
		PackagePath:   repoPath,
		BinOutputPath: filepath.Join("bin", ".cache", binaryName, platform.String(), "bin", binaryName),
	}

	switch {
	case platform == tools.PlatformDockerAMD64:
	//nolint:gocritic // condition is suspicious but fine
	// If we build on ARM64 for ARM64 no special config is required. But if we build on AMD64 for ARM64
	// then crosscompilation must be enabled.
	case platform == tools.PlatformDockerARM64 && platform == tools.PlatformDockerLocal:
	case platform == tools.PlatformDockerARM64:
		config.CrosscompileARM64 = true
	default:
		return errors.Errorf("releasing cored is not possible for platform %s", platform)
	}

	return golang.BuildInDocker(ctx, config, platform)
}

// BuildIntegrationTests builds faucet integration tests.
func BuildIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, ensureRepo)

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../faucet/integration-tests",
		BinOutputPath: testBinaryPath,
		Tags:          []string{"integrationtests"},
	})
}

// Tidy runs `go mod tidy` for faucet repo.
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints faucet repo.
func Lint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Lint(ctx, repoPath, deps)
}

// Test run unit tests in faucet repo.
func Test(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Test(ctx, repoPath, deps)
}

func ensureRepo(ctx context.Context, deps build.DepsFunc) error {
	return git.EnsureRepo(ctx, repoURL)
}
