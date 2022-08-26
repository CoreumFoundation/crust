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
	deps(buildCored, buildFaucet, buildZNet, buildAllIntegrationTests)
}

func buildAllIntegrationTests(deps build.DepsFunc) {
	deps(buildCoreumIntegrationTests)
}

func buildCoreumIntegrationTests(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureCoreumRepo)

	return golang.BuildTests(ctx, golang.TestBuildConfig{
		PackagePath:   "../coreum/integration-tests",
		BinOutputPath: "bin/.cache/integration-tests/coreum",
		Tags:          []string{"integration"},
	})
}

func buildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, git.EnsureCoreumRepo)

	if err := golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "../coreum/cmd/cored",
		BinOutputPath: "bin/cored",
		CGOEnabled:    true,
	}); err != nil {
		return err
	}

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  "bin/.cache/docker/cored",
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

func buildFaucet(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, git.EnsureFaucetRepo)

	return golang.BuildInDocker(ctx, golang.BinaryBuildConfig{
		PackagePath:    "../faucet",
		BinOutputPath:  "bin/.cache/docker/faucet",
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	})
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "build/cmd",
		BinOutputPath: must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.BuildLocally(ctx, golang.BinaryBuildConfig{
		PackagePath:   "cmd/znet",
		BinOutputPath: "bin/.cache/znet",
	})
}
