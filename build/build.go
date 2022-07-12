package build

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
)

const coreumRepoURL = "https://github.com/CoreumFoundation/coreum.git"

func buildAll(deps build.DepsFunc) {
	deps(buildCored, buildZNet, buildZStress)
}

func buildCored(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo, ensureLibWASMVMMuslC, ensureCoreumRepo)
	return goBuild(ctx, goBuildConfig{
		Package:        "../coreum/cmd/cored",
		BinOutputPath:  "bin/cored",
		DockerStatic:   true,
		DockerTags:     []string{"muslc"},
		CGOEnabled:     true,
		BuildForHost:   true,
		BuildForDocker: true,
	})
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuild(ctx, goBuildConfig{
		Package:       "build/cmd",
		BinOutputPath: "bin/.cache/crust",
		BuildForHost:  true,
	})
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuild(ctx, goBuildConfig{
		Package:       "cmd/znet",
		BinOutputPath: "bin/.cache/znet",
		BuildForHost:  true,
	})
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuild(ctx, goBuildConfig{
		Package:        "cmd/zstress",
		BinOutputPath:  "bin/.cache/zstress",
		BuildForHost:   true,
		BuildForDocker: true,
	})
}

func ensureAllRepos(deps build.DepsFunc) {
	deps(ensureCoreumRepo)
}

func ensureCoreumRepo(ctx context.Context) error {
	return ensureRepo(ctx, coreumRepoURL)
}
