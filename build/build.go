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
	deps(ensureGo, ensureCoreumRepo)
	return goBuildNativeAndDocker(ctx, "../coreum/cmd/cored", "bin/cored", true)
}

func buildCrust(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuildOnHost(ctx, "build/cmd", "bin/.cache/crust", false)
}

func buildZNet(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuildOnHost(ctx, "cmd/znet", "bin/.cache/znet", false)
}

func buildZStress(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo)
	return goBuildNativeAndDocker(ctx, "cmd/zstress", "bin/.cache/zstress", false)
}

func ensureAllRepos(deps build.DepsFunc) {
	deps(ensureCoreumRepo)
}

func ensureCoreumRepo(ctx context.Context) error {
	return ensureRepo(ctx, coreumRepoURL)
}
