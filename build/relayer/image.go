package relayer

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the ibc relayer.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	const binaryName = "relayer"

	relayerLocalPath := filepath.Join("bin", ".cache", "docker", "relayer")

	if err := tools.EnsureDocker(ctx, tools.Relayer); err != nil {
		return err
	}

	if err := tools.CopyToolBinaries(tools.Relayer, relayerLocalPath, binaryName); err != nil {
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
		ContextDir: relayerLocalPath,
		ImageName:  binaryName,
		Dockerfile: dockerfile,
	})
}
