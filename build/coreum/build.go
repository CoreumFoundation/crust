package coreum

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	localBinaryPath  = "bin/cored"
	dockerBinaryPath = "bin/.cache/docker/cored/cored"
	testBinaryPath   = "bin/.cache/integration-tests/coreum"
)

// BuildCored builds all the versions of cored binary
func BuildCored(deps build.DepsFunc) {
	deps(BuildCoredLocally, BuildCoredInDocker)
}

// BuildCoredLocally builds cored locally
func BuildCoredLocally(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureCoreumRepo)

	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: localBinaryPath,
		CGOEnabled:    true,
	})
}

// BuildCoredInDocker builds cored in docker
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, git.EnsureCoreumRepo)

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  dockerBinaryPath,
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

// BuildIntegrationTests builds coreum integration tests
func BuildIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureCoreumRepo)

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../coreum/integration-tests",
		BinOutputPath: testBinaryPath,
		Tags:          []string{"integration"},
	})
}
