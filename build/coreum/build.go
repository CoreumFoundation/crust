package coreum

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	blockchainName = "coreum"
	binaryName     = "cored"
	repoURL        = "https://github.com/CoreumFoundation/coreum.git"
	repoName       = "coreum"
	repoPath       = "../" + repoName
	binaryPath     = "bin/" + binaryName
	testsDir       = repoPath + "/integration-tests"
	testsBinDir    = "bin/.cache/integration-tests"

	cosmovisorBinaryPath = "bin/cosmovisor"
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

	return golang.Build(ctx, golang.BinaryBuildConfig{
		Platform:      tools.PlatformLocal,
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: binaryPath,
		Parameters:    parameters,
		CGOEnabled:    true,
		Tags:          tagsLocal,
	})
}

// BuildCoredInDocker builds cored in docker.
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	return buildCoredInDocker(ctx, deps, tools.PlatformDockerLocal)
}

func buildCoredInDocker(ctx context.Context, deps build.DepsFunc, platform tools.Platform) error {
	deps(golang.EnsureGo, ensureRepo)

	parameters, err := coredVersionParams(ctx, tagsDocker)
	if err != nil {
		return err
	}

	if err := tools.EnsureBinaries(ctx, tools.LibWASMMuslC, platform); err != nil {
		return err
	}

	return golang.Build(ctx, golang.BinaryBuildConfig{
		Platform:       platform,
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  filepath.Join("bin", ".cache", binaryName, platform.String(), "bin", binaryName),
		Parameters:     parameters,
		CGOEnabled:     true,
		Tags:           tagsDocker,
		LinkStatically: true,
	})
}

// Tidy runs `go mod tidy` for coreum repo.
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints coreum repo.
func Lint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo, Generate, CompileAllSmartContracts)
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

func coredVersionParams(ctx context.Context, buildTags []string) (params, error) {
	hash, err := git.DirtyHeadHash(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	version, err := git.VersionFromTag(ctx, repoPath)
	if err != nil {
		return nil, err
	}
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
