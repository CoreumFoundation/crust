package hermes

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
	binaryName = "hermes"
	binaryPath = "bin/" + binaryName
)

// BuildDockerImage builds docker image of the ibc relayer.
func BuildDockerImage(ctx context.Context, deps types.DepsFunc) error {
	if err := tools.Ensure(ctx, tools.Hermes, tools.TargetPlatformLinuxLocalArchInDocker); err != nil {
		return err
	}

	hermesLocalPath := filepath.Join("bin", ".cache", binaryName, tools.TargetPlatformLinuxLocalArchInDocker.String())
	if err := tools.CopyToolBinaries(
		tools.Hermes,
		tools.TargetPlatformLinuxLocalArchInDocker,
		hermesLocalPath,
		binaryPath,
	); err != nil {
		return err
	}

	dockerfile, err := dockerbasic.Execute(dockerbasic.Data{
		From:   docker.UbuntuImage,
		Binary: binaryPath,
		Run:    "apt update && apt install curl jq -y",
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: hermesLocalPath,
		ImageName:  binaryName,
		Dockerfile: dockerfile,
		Versions:   []string{config.ZNetVersion},
	})
}
