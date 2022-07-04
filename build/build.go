package build

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/build/exec"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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

	if !cgoEnabled && runtime.GOOS != dockerGOOS {
		return goBuildPkg(ctx, pkg, dockerGOOS, filepath.Join(dir, dockerGOOS, binName), false)
	} else if cgoEnabled {
		// docker-targeted binary must be built from within Docker environment
		return goBuildWithDocker(ctx, pkg, filepath.Join(dir, dockerGOOS), binName)
	}

	return nil
}

//go:embed docker/Dockerfile.cgo
var cgoDockerfile []byte

func goBuildWithDocker(ctx context.Context, pkgPath, outPath, binName string) error {
	tmpDir := filepath.Join(os.TempDir(), "crust")
	must.OK(os.MkdirAll(tmpDir, 0o700))

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile.cgo")
	must.OK(ioutil.WriteFile(dockerfilePath, cgoDockerfile, 0o600))

	absPkgPath, err := filepath.Abs(pkgPath)
	must.OK(err)

	// buildContext should be the module root containing go.sum and go.mod
	// with the actualy bin package located under ./cmd/{binName}
	buildContext := filepath.Base(filepath.Base(absPkgPath))

	log := logger.Get(ctx)
	log.With(zap.String("build_ctx", buildContext)).Info("Building CGO-enabled bin inside Docker")

	if out, err := exec.Docker(
		"build",
		"--build-arg", "BIN_NAME="+binName,
		"--tag", binName+"-cgo-build",
		"-f", dockerfilePath,
		buildContext,
	).CombinedOutput(); err != nil {
		_, _ = io.Copy(os.Stderr, bytes.NewReader(out))
		err = errors.Wrapf(err, "failed to build %s inside Docker", binName)
		return err
	}

	absOutPath, err := filepath.Abs(outPath)
	must.OK(err)

	if out, err := exec.Docker(
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:%s", absOutPath, "/mnt"),
		"--env", "BIN_NAME="+binName,
		binName+"-cgo-build",
	).CombinedOutput(); err != nil {
		_, _ = io.Copy(os.Stderr, bytes.NewReader(out))
		err = errors.Wrapf(err, "failed to copy %s outside of builder image", binName)
		return err
	}

	return nil
}
