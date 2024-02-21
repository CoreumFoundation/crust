package coreum

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
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
	goCoverFlag          = "-cover"
	linkStaticallyLDFlag = "-ldflags=-extldflags=-static"
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
	deps(ensureRepo)

	versionFlags, err := coredVersionLDFlags(ctx, tagsLocal)
	if err != nil {
		return err
	}

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "../coreum/cmd/cored",
		Envs:           []string{"CGO_ENABLED=1"},
		Flags: []string{
			goCoverFlag,
			versionFlags,
			"-tags=" + strings.Join(tagsLocal, ","),
			"-o=" + binaryName,
		},
	})
}

// BuildCoredInDocker builds cored in docker.
func BuildCoredInDocker(ctx context.Context, deps build.DepsFunc) error {
	return buildCoredInDocker(ctx, deps, tools.TargetPlatformLinuxLocalArchInDocker, []string{goCoverFlag})
}

func buildCoredInDocker(
	ctx context.Context,
	deps build.DepsFunc,
	targetPlatform tools.TargetPlatform,
	extraFlags []string,
) error {
	deps(ensureRepo)

	versionFlags, err := coredVersionLDFlags(ctx, tagsDocker)
	if err != nil {
		return err
	}

	if tools.TargetPlatformLocal == tools.TargetPlatformLinuxAMD64 &&
		targetPlatform == tools.TargetPlatformLinuxARM64InDocker {
		if err := tools.Ensure(ctx, tools.Aarch64LinuxMuslCross, tools.TargetPlatformLinuxAMD64InDocker); err != nil {
			return err
		}
	}
	if err := tools.Ensure(ctx, tools.LibWASMMuslC, targetPlatform); err != nil {
		return err
	}

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: targetPlatform,
		PackagePath:    "../coreum/cmd/cored",
		Envs:           []string{"CGO_ENABLED=1"},
		Flags: append(
			extraFlags,
			versionFlags,
			linkStaticallyLDFlag,
			"-tags="+strings.Join(tagsDocker, ","),
			"-o="+filepath.Join("bin", ".cache", binaryName, targetPlatform.String(), "bin", binaryName),
		),
	})
}

// buildCoredClientInDocker builds cored binary without the wasm VM and with CGO disabled. The result binary might be
// used for the CLI on target platform, but can't be used to run the node.
func buildCoredClientInDocker(ctx context.Context, deps build.DepsFunc, targetPlatform tools.TargetPlatform) error {
	deps(ensureRepo)

	versionFlags, err := coredVersionLDFlags(ctx, tagsDocker)
	if err != nil {
		return err
	}

	binOutputPath := filepath.Join(
		"bin",
		".cache",
		binaryName,
		targetPlatform.String(),
		"bin",
		fmt.Sprintf("%s-client", binaryName),
	)
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: targetPlatform,
		PackagePath:    "../coreum/cmd/cored",
		Envs:           []string{"CGO_ENABLED=0"},
		Flags: []string{
			versionFlags,
			linkStaticallyLDFlag,
			"-tags=" + strings.Join(tagsDocker, ","),
			"-o=" + binOutputPath,
		},
	})
}

// Tidy runs `go mod tidy` for coreum repo.
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)
	return golang.Tidy(ctx, repoPath, deps)
}

// Lint lints coreum repo.
func Lint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo, Generate, CompileAllSmartContracts, formatProto, lintProto, breakingProto)
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

func coredVersionLDFlags(ctx context.Context, buildTags []string) (string, error) {
	hash, err := git.DirtyHeadHash(ctx, repoPath)
	if err != nil {
		return "", err
	}

	version, err := git.VersionFromTag(ctx, repoPath)
	if err != nil {
		return "", err
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

	var ldFlags []string
	for k, v := range ps {
		ldFlags = append(ldFlags, fmt.Sprintf("-X %s=%s", k, v))
	}

	return "-ldflags=" + strings.Join(ldFlags, " "), nil
}

func formatProto(ctx context.Context, deps build.DepsFunc) error {
	deps(tools.EnsureBuf)

	cmd := exec.Command(tools.Path("bin/buf", tools.TargetPlatformLocal), "format", "-w")
	cmd.Dir = filepath.Join(repoPath, "proto", "coreum")
	return libexec.Exec(ctx, cmd)
}
