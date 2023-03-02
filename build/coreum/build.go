package coreum

import (
	"context"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	blockchainName        = "coreum"
	binaryName            = "cored"
	repoURL               = "https://github.com/CoreumFoundation/coreum.git"
	repoPath              = "../coreum"
	localBinaryPath       = "bin/" + binaryName
	dockerBinaryPath      = "bin/.cache/docker/cored/" + binaryName
	testBinaryModulePath  = "bin/.cache/integration-tests/coreum-modules"
	testBinaryUpgradePath = "bin/.cache/integration-tests/coreum-upgrade"
)

var (
	tagsLocal  = []string{"netgo", "ledger"}
	tagsDocker = append([]string{"muslc"}, tagsLocal...)
)

// BuildCored builds all the versions of cored binary.
func BuildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(BuildCoredLocally, BuildCoredInDocker)
	return nil
}

// BuildCoredLocally builds cored locally.
func BuildCoredLocally(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, ensureRepo)

	parameters, err := coredVersionParams(ctx, tagsLocal)
	if err != nil {
		return err
	}

	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: localBinaryPath,
		Parameters:    parameters,
		CGOEnabled:    true,
		Tags:          tagsLocal,
	})
}

// BuildCoredInDocker builds cored in docker.
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

	parameters, err := coredVersionParams(ctx, tagsDocker)
	if err != nil {
		return err
	}

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  dockerBinaryPath,
		Parameters:     parameters,
		CGOEnabled:     true,
		Tags:           tagsDocker,
		LinkStatically: true,
	})
}

func buildCoredWithFakeUpgrade(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

	parameters, err := coredVersionParams(ctx, tagsDocker)
	if err != nil {
		return err
	}

	// This enables fake upgrade handler used to test upgrade procedure in CI before we have real one
	parameters["github.com/CoreumFoundation/coreum/pkg/config.EnableFakeUpgradeHandler"] = "true"
	// We do this to be able to verify that upgraded version was started by cosmovisor.
	// I would prefer to modify Commit instead of Version but only Version is exposed by ABCI's Info call.
	parameters["github.com/cosmos/cosmos-sdk/version.Version"] += "-upgrade"

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath: "../coreum/cmd/cored",
		// we build the cored-upgrade to its own directory since the cored will be used for the release but upgrade for tests only.
		BinOutputPath:  filepath.Join("bin/.cache/docker/cored-upgrade", binaryName+"-upgrade"),
		Parameters:     parameters,
		CGOEnabled:     true,
		Tags:           tagsDocker,
		LinkStatically: true,
	})
}

// BuildIntegrationTests builds coreum integration tests.
func BuildIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, ensureRepo)

	err := golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../coreum/integration-tests/modules",
		BinOutputPath: testBinaryModulePath,
		Tags:          []string{"integrationtests"},
	})
	if err != nil {
		return err
	}

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../coreum/integration-tests/upgrade",
		BinOutputPath: testBinaryUpgradePath,
		Tags:          []string{"integrationtests"},
	})
}

// Tidy runs `go mod tidy` for coreum repo.
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints coreum repo.
func Lint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Lint(ctx, repoPath, deps)
}

// Test run unit tests in coreum repo.
func Test(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Test(ctx, repoPath, deps)
}

func ensureRepo(ctx context.Context, deps build.DepsFunc) error {
	return git.EnsureRepo(ctx, repoURL)
}

type params map[string]string

func (p params) Version() string {
	return p["github.com/cosmos/cosmos-sdk/version.Version"]
}

func (p params) Commit() string {
	return p["github.com/cosmos/cosmos-sdk/version.Commit"]
}

func (p params) IsDirty() bool {
	return strings.HasSuffix(p["github.com/cosmos/cosmos-sdk/version.Commit"], "-dirty")
}

func coredVersionParams(ctx context.Context, buildTags []string) (params, error) {
	hash, err := git.DirtyHeadHash(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	tags, err := git.HeadTags(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	version := firstVersionTag(tags)
	if version == "" {
		version = hash
	}
	ps := params{
		"github.com/cosmos/cosmos-sdk/version.Name":    blockchainName,
		"github.com/cosmos/cosmos-sdk/version.AppName": binaryName,
		"github.com/cosmos/cosmos-sdk/version.Version": version,
		"github.com/cosmos/cosmos-sdk/version.Commit":  hash,
	}

	if len(buildTags) > 0 {
		ps["github.com/cosmos/cosmos-sdk/version.BuildTags"] = strings.Join(buildTags, ",")
	}

	return ps, nil
}

func firstVersionTag(tags []string) string {
	for _, tag := range tags {
		if semver.IsValid(tag) {
			return tag
		}
	}
	return ""
}
