package coreum

import (
	"context"
	"path/filepath"

	"golang.org/x/mod/semver"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	blockchainName   = "coreum"
	binaryName       = "cored"
	repoURL          = "https://github.com/CoreumFoundation/coreum.git"
	repoPath         = "../coreum"
	localBinaryPath  = "bin/" + binaryName
	dockerBinaryPath = "bin/.cache/docker/cored/" + binaryName
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

	parameters, err := coredVersionParams(ctx)
	if err != nil {
		return err
	}

	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: localBinaryPath,
		Parameters:    parameters,
		CGOEnabled:    true,
	})
}

// BuildCoredInDocker builds cored in docker
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

	parameters, err := coredVersionParams(ctx)
	if err != nil {
		return err
	}

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  dockerBinaryPath,
		Parameters:     parameters,
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

func buildCoredWithFakeUpgrade(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

	parameters, err := coredVersionParams(ctx)
	if err != nil {
		return err
	}

	// This enables fake upgrade handler used to test upgrade procedure in CI before we have real one
	parameters["github.com/CoreumFoundation/coreum/pkg/config.EnableFakeUpgradeHandler"] = "true"
	// We do this to be able to verify that upgraded version was started by cosmovisor.
	// I would prefer to modify Commit instead of Version but only Version is exposed by ABCI's Info call.
	parameters["github.com/cosmos/cosmos-sdk/version.Version"] += "-upgrade"

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  filepath.Join("bin/.cache/docker", binaryName+"-upgrade"),
		Parameters:     parameters,
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

func coredVersionParams(ctx context.Context) (map[string]string, error) {
	hash, err := git.DirtyHeadHash(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	tags, err := git.HeadTags(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"github.com/cosmos/cosmos-sdk/version.Name":    blockchainName,
		"github.com/cosmos/cosmos-sdk/version.AppName": binaryName,
		"github.com/cosmos/cosmos-sdk/version.Version": firstVersionTag(tags),
		"github.com/cosmos/cosmos-sdk/version.Commit":  hash,
	}, nil
}

func firstVersionTag(tags []string) string {
	for _, tag := range tags {
		if semver.IsValid(tag) {
			return tag
		}
	}
	return ""
}
