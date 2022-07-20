package contracts

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// BuildConfig provides params for the building stage.
type BuildConfig struct {
	// NeedOptimizedBuild will enable running a Docker container that builds a very optimized and predictable build.
	// This option must be enabled for any deployment-oriented build.
	NeedOptimizedBuild bool
}

// Build implements logic for "contracts build" CLI command
func Build(ctx context.Context, workspaceDir string, config BuildConfig) error {
	log := logger.Get(ctx)

	if err := ensureBuildTools(ctx, config.NeedOptimizedBuild); err != nil {
		err = errors.Wrap(err, "not all build dependencies are installed")
		return err
	}

	crateName, err := readCrateMetadata(ctx, workspaceDir)
	if err != nil {
		err = errors.Wrap(err, "problem with ensuring the target crate workspace")
		return err
	}
	crateLog := log.With(zap.String("name", crateName), zap.String("dir", workspaceDir))

	crateLog.Info("Running cargo check on the workspace")
	if err := runCargoCheck(ctx, workspaceDir); err != nil {
		err = errors.Wrap(err, "problem with checking the target crate workspace")
		return err
	}

	var outDir string
	if config.NeedOptimizedBuild {
		outDir = filepath.Join(workspaceDir, "artifacts")
		crateLog.Info("Running build of the optimized WASM artefact",
			zap.String("outDir", outDir),
		)

		if err := runReleaseOptimizedBuild(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with docker run cargo build")
			return err
		}
	} else {
		outDir = filepath.Join(workspaceDir, "target", wasmTarget, "release")
		crateLog.Info("Running build of the release WASM artefact",
			zap.String("outDir", outDir),
		)

		if err := runReleaseBuild(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with cargo build")
			return err
		}
	}

	outDirAbs, err := filepath.Abs(outDir)
	if err != nil {
		crateLog.With(zap.Error(err)).Warn("Build completed, but no WASM artefact could be located")
	} else {
		artefactPath := filepath.Join(outDirAbs, fmt.Sprintf("%s.wasm", crateNameToArtefactName(crateName)))
		crateLog.Info("Build artefact created", zap.String("path", artefactPath))
	}

	return nil
}

// runCargoCheck checks workspace and all dependencies for errors
func runCargoCheck(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"check", "--workspace", "--quiet", "--tests",
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo check run")
		return err
	}

	return nil
}

// runDebugBuild builds the debug variant of the contract for the wasm target
func runDebugBuild(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"build", "--debug", "--target", wasmTarget,
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo wasm build debug")
		return err
	}

	return nil
}

// runReleaseBuild builds the release variant of the contract for the wasm target
func runReleaseBuild(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"build", "--release", "--target", wasmTarget,
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo wasm build release")
		return err
	}

	return nil
}

// runReleaseOptimizedBuild builds the release with all optimizations and a deterministic result
// is obtained by leveraging a special Docker image.
func runReleaseOptimizedBuild(ctx context.Context, workspaceDir string) error {
	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		err = errors.Wrapf(err, "failed to resolve abs path for the dir: %s", workspaceDir)
		return err
	}

	crateName, err := readCrateMetadata(ctx, workspaceDir)
	if err != nil {
		err = errors.Wrap(err, "problem with ensuring the target crate workspace")
		return err
	}

	cmdArgs := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/code", absWorkspaceDir),
		"--mount", "type=volume,source=crust_contracts_registry_cache,target=/usr/local/cargo/registry",
		"--mount", fmt.Sprintf("type=volume,source=crust_contracts_sccache_%s,target=/root/.cache/sccache", crateNameToArtefactName(crateName)),
		"--mount", fmt.Sprintf("type=volume,source=crust_contracts_target_%s,target=/code/target", crateNameToArtefactName(crateName)),
		optimizedBuildDockerImage,
	}
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = workspaceDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo wasm build release")
		return err
	}

	return nil
}

func crateNameToArtefactName(crateName string) string {
	return strings.ReplaceAll(crateName, "-", "_")
}
