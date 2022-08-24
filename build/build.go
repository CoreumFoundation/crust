package build

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/golang"
)

// FIXME (wojtek): rearrange functions here and in `git` in product-specific subpackages

func buildAll(deps build.DepsFunc) {
	deps(buildCored, buildZNet)
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
