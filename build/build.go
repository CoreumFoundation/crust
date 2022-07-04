package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	_ "embed"
)

const dockerGOOS = "linux"

const coreumRepoURL = "https://github.com/CoreumFoundation/coreum.git"

func buildAll(deps build.DepsFunc) {
	deps(buildCored, buildZNet, buildZStress)
}

func buildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo, ensureCoreumRepo)

	return buildNativeAndDocker(ctx, "../coreum/cmd/cored", "bin/cored", true)
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuildPkg(ctx, "build/cmd", runtime.GOOS, "bin/.cache/crust", false)
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)

	return goBuildPkg(ctx, "cmd/znet", runtime.GOOS, "bin/.cache/znet", false)
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)

	return buildNativeAndDocker(ctx, "cmd/zstress", "bin/.cache/zstress", false)
}

func ensureAllRepos(deps build.DepsFunc) {
	deps(ensureCoreumRepo)
}

func ensureCoreumRepo(ctx context.Context) error {
	return ensureRepo(ctx, coreumRepoURL)
}

func buildNativeAndDocker(ctx context.Context, pkg, out string, cgoEnabled bool) error {
	dir := filepath.Dir(out)
	binName := filepath.Base(out)
	outPath := filepath.Join(dir, runtime.GOOS, binName)

	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return errors.WithStack(err)
	}
	if err := goBuildPkg(ctx, pkg, runtime.GOOS, outPath, cgoEnabled); err != nil {
		return err
	}
	if err := os.Link(outPath, out); err != nil {
		return errors.WithStack(err)
	}

	if cgoEnabled {
		// docker-targeted binary must be built from within Docker environment
		return goBuildWithDocker(ctx, pkg, filepath.Join(dir, dockerGOOS), binName)
	} else if runtime.GOOS != dockerGOOS {
		return goBuildPkg(ctx, pkg, dockerGOOS, filepath.Join(dir, dockerGOOS, binName), false)
	}

	return nil
}

//go:embed docker/Dockerfile.cgo
var cgoDockerfile []byte

func goBuildWithDocker(ctx context.Context, pkgPath, outPath, binName string) error {
	absPkgPath, err := filepath.Abs(pkgPath)
	must.OK(err)

	// buildContext should be the module root containing go.sum and go.mod
	// with the actualy bin package located under ./cmd/{binName}
	buildContext := filepath.Dir(filepath.Dir(absPkgPath))

	log := logger.Get(ctx)
	log.With(zap.String("buildContext", buildContext)).Info("Building CGO-enabled bin inside Docker")

	absOutPath, err := filepath.Abs(outPath)
	must.OK(err)

	dockerCmd, err := exec.LookPath("docker")
	if err != nil {
		err = errors.Wrap(err, "docker command is not available in PATH")
		return err
	}

	buildCmd := &exec.Cmd{
		Path:  dockerCmd,
		Stdin: bytes.NewReader(cgoDockerfile),
		Args: []string{
			"docker", "build",
			"--build-arg", "BIN_NAME=" + binName,
			"--tag", binName + "-cgo-build",
			"-f", "-",
			buildContext,
		},
	}
	log.Debug(buildCmd.String())

	runCmd := &exec.Cmd{
		Path: dockerCmd,
		Args: []string{
			"docker", "run",
			"--rm",
			"-v", fmt.Sprintf("%s:%s", absOutPath, "/mnt"),
			"--env", "BIN_NAME=" + binName,
			binName + "-cgo-build",
		},
	}
	log.Debug(runCmd.String())

	if err := libexec.Exec(ctx, buildCmd, runCmd); err != nil {
		err = errors.Wrapf(err, "failed to build %s inside Docker", binName)
		return err
	}

	return nil
}
