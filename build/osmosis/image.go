package osmosis

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/crust/build/config"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

const (
	binaryName = "osmosisd"
	binaryPath = "bin/" + binaryName
)

// BuildDockerImage builds docker image of the osmosis.
func BuildDockerImage(ctx context.Context, deps types.DepsFunc) error {
	if err := tools.Ensure(ctx, tools.Osmosis, tools.TargetPlatformLinuxLocalArchInDocker); err != nil {
		return err
	}

	binaryLocalPath := filepath.Join("bin", ".cache", binaryName, tools.TargetPlatformLinuxLocalArchInDocker.String())
	if err := tools.CopyToolBinaries(
		tools.Osmosis,
		tools.TargetPlatformLinuxLocalArchInDocker,
		binaryLocalPath,
		binaryPath,
	); err != nil {
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
		ContextDir: binaryLocalPath,
		ImageName:  binaryName,
		Dockerfile: dockerfile,
		Versions:   []string{config.ZNetVersion},
	})
}
