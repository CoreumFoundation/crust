package coreum

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	repoURL          = "https://github.com/CoreumFoundation/coreum.git"
	repoPath         = "../coreum"
	localBinaryPath  = "bin/cored"
	dockerBinaryPath = "bin/.cache/docker/cored/cored"
	testBinaryPath   = "bin/.cache/integration-tests/coreum"
)

// BuildCored builds all the versions of cored binary
func BuildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(BuildCoredLocally, BuildCoredInDocker)
	return nil
}

// BuildCoredLocally builds cored locally
func BuildCoredLocally(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, ensureRepo)

	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: localBinaryPath,
		CGOEnabled:    true,
	})
}

// BuildCoredInDocker builds cored in docker
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

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
	deps(golang.EnsureGo, ensureRepo)

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../coreum/integration-tests",
		BinOutputPath: testBinaryPath,
		Tags:          []string{"integration"},
	})
}

// Tidy runs `go mod tidy` for coreum repo
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints coreum repo
func Lint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Lint(ctx, repoPath, deps)
}

// Test run unit tests in coreum repo
func Test(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Test(ctx, repoPath, deps)
}

func ensureRepo(ctx context.Context, deps build.DepsFunc) error {
	return git.EnsureRepo(ctx, repoURL)
}
