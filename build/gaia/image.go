package gaia

import (
	"context"
	"path"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the gaia.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	const binaryName = "gaiad"

	gaiaLocalPath := path.Join("bin", ".cache", "docker", "gaia")
	if err := tools.EnsureDocker(ctx, tools.Gaia); err != nil {
		return err
	}

	if err := tools.CopyToolBinaries(tools.Gaia, gaiaLocalPath, binaryName); err != nil {
		return err
	}

	dockerfile, err := dockerbasic.Execute(dockerbasic.Data{
		From:   docker.AlpineImage,
		Binary: binaryName,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: gaiaLocalPath,
		ImageName:  binaryName,
		Dockerfile: dockerfile,
	})
}
