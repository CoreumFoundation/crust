package build

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/faucet"
	"github.com/CoreumFoundation/crust/build/golang"
)

// FIXME (wojtek): rearrange functions here and in `git` in product-specific subpackages

func buildAll(deps build.DepsFunc) {
	deps(coreum.BuildCored, faucet.Build, buildZNet, buildAllIntegrationTests)
}

func buildAllIntegrationTests(deps build.DepsFunc) {
	deps(coreum.BuildIntegrationTests, faucet.BuildIntegrationTests)
}

func buildAllDockerImages(deps build.DepsFunc) {
	deps(coreum.BuildCoredDockerImage, faucet.BuildDockerImage)
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
		CGOEnabled:    true,
	})
}
