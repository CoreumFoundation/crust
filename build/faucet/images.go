package faucet

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the faucet.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	dockerfile, err := dockerbasic.Execute(dockerbasic.Data{
		From:   docker.AlpineImage,
		Binary: binaryPath,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   "../faucet",
		ContextDir: filepath.Join("bin", ".cache", binaryName, tools.PlatformDockerLocal.String()),
		ImageName:  binaryName,
		Dockerfile: dockerfile,
	})
}
