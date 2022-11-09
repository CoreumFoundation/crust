package gaia

import (
	"context"
	"path"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the gaia.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	gaiaLocalPath := tools.PathLocal(path.Join(".cache", "docker", "gaia"))

	deps(func(ctx context.Context, deps build.DepsFunc) error {
		return tools.EnsureDocker(ctx, tools.Gaia)
	})

	artifactNames := tools.CopyToolBinaries(tools.Gaia, gaiaLocalPath)
	if len(artifactNames) != 1 {
		return errors.New("Unexpected number of artifacts is returned")
	}

	dockerfile, err := basic.Execute(basic.Data{
		From:   docker.AlpineImage,
		Binary: artifactNames[0],
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: gaiaLocalPath,
		ImageName:  artifactNames[0],
		Dockerfile: dockerfile,
	})
}
