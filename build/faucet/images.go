package faucet

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/faucet/image"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the faucet.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	return buildDockerImage(ctx, []tools.Platform{tools.PlatformDockerLocal}, docker.ActionLoad)
}

func buildDockerImage(ctx context.Context, platforms []tools.Platform, action docker.Action) error {
	dockerfile, err := image.Execute(image.Data{
		From:   docker.AlpineImage,
		Binary: binaryPath,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   repoPath,
		ContextDir: filepath.Join("bin", ".cache", binaryName),
		ImageName:  binaryName,
		Platforms:  platforms,
		Action:     action,
		Dockerfile: dockerfile,
	})
}
