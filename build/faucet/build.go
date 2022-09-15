package faucet

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	dockerBinaryPath = "bin/.cache/docker/faucet/faucet"
	testBinaryPath   = "bin/.cache/integration-tests/faucet"
)

// Build builds faucet in docker
func Build(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureFaucetRepo)

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../faucet",
		BinOutputPath:  dockerBinaryPath,
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

// BuildIntegrationTests builds faucet integration tests
func BuildIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureFaucetRepo)

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../faucet/integration-tests",
		BinOutputPath: testBinaryPath,
		Tags:          []string{"integration"},
	})
}
