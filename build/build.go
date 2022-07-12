package build

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"
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
	return goBuildPkg(ctx, "build/cmd", runtime.GOOS, must.String(filepath.EvalSymlinks(must.String(os.Executable()))), false)
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
		// docker-targeted cgo-enabled binary must be built from within Docker environment
		return goBuildWithDocker(ctx, pkg, filepath.Join(dir, "alpine-cgo", binName))
	} else if runtime.GOOS != dockerGOOS {
		return goBuildPkg(ctx, pkg, dockerGOOS, filepath.Join(dir, dockerGOOS, binName), false)
	}

	return nil
}
