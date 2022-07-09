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
		BinPath:        "bin/cored",
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
		Package:      "build/cmd",
		BinPath:      "bin/.cache/crust",
		BuildForHost: true,
	})
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuild(ctx, goBuildConfig{
		Package:      "cmd/znet",
		BinPath:      "bin/.cache/znet",
		BuildForHost: true,
	})
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuild(ctx, goBuildConfig{
		Package:        "cmd/zstress",
		BinPath:        "bin/.cache/zstress",
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
