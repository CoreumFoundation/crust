package build

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"

	"github.com/CoreumFoundation/coreum/build/git"
	"github.com/CoreumFoundation/coreum/build/golang"
)

func buildAll(deps build.DepsFunc) {
	deps(buildCored, buildZNet, buildZStress)
}

func buildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, git.EnsureCoreumRepo)

	if err := golang.BuildLocally(ctx, golang.BuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: "bin/cored",
		CGOEnabled:    true,
	}); err != nil {
		return err
	}

	return golang.BuildInDocker(ctx, golang.BuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  "bin/.cache/docker/cored",
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BuildConfig{
		PackagePath:   "build/cmd",
		BinOutputPath: must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BuildConfig{
		PackagePath:   "cmd/znet",
		BinOutputPath: "bin/.cache/znet",
	})
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	if err := golang.BuildLocally(ctx, golang.BuildConfig{
		PackagePath:   "cmd/zstress",
		BinOutputPath: "bin/.cache/zstress",
	}); err != nil {
		return err
	}

	return golang.BuildInDocker(ctx, golang.BuildConfig{
		PackagePath:   "cmd/zstress",
		BinOutputPath: "bin/.cache/zstress",
	})
}
