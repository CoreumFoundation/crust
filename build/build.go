package build

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"

	"github.com/CoreumFoundation/coreum/build/git"
	"github.com/CoreumFoundation/coreum/build/golang"
)

func buildAll(deps build.DepsFunc) {
	deps(buildCored, buildZNet, buildZStress)
}

func buildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, git.EnsureCoreumRepo)
	return golang.Build(ctx, golang.BuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  "bin/cored",
		DockerStatic:   true,
		DockerTags:     []string{"muslc"},
		CGOEnabled:     true,
		BuildForLocal:  true,
		BuildForDocker: true,
	})
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.Build(ctx, golang.BuildConfig{
		PackagePath:   "build/cmd",
		BinOutputPath: "bin/.cache/crust",
		BuildForLocal: true,
	})
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.Build(ctx, golang.BuildConfig{
		PackagePath:   "cmd/znet",
		BinOutputPath: "bin/.cache/znet",
		BuildForLocal: true,
	})
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(golang.EnsureGo)
	return golang.Build(ctx, golang.BuildConfig{
		PackagePath:    "cmd/zstress",
		BinOutputPath:  "bin/.cache/zstress",
		BuildForLocal:  true,
		BuildForDocker: true,
	})
}
