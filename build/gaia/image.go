package gaia

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	binaryName = "gaiad"
	binaryPath = "bin/" + binaryName
)

// BuildDockerImage builds docker image of the gaia.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	if err := tools.EnsureBinaries(ctx, tools.Gaia, tools.PlatformDockerLocal); err != nil {
		return err
	}

	gaiaLocalPath := filepath.Join("bin", ".cache", binaryName, tools.PlatformDockerLocal.String())
	if err := tools.CopyToolBinaries(tools.Gaia, tools.PlatformDockerLocal, gaiaLocalPath, binaryPath); err != nil {
		return err
	}

	dockerfile, err := dockerbasic.Execute(dockerbasic.Data{
		From:   docker.AlpineImage,
		Binary: binaryPath,
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
