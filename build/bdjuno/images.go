package bdjuno

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/bdjuno/image"
	"github.com/CoreumFoundation/crust/build/config"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildDockerImage builds docker image of the faucet.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	dockerfile, err := image.Execute(image.Data{
		From:   docker.AlpineImage,
		Binary: binaryPath,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: filepath.Join("bin", ".cache", binaryName, tools.TargetPlatformLinuxLocalArchInDocker.String()),
		ImageName:  binaryName,
		Dockerfile: dockerfile,
		Versions:   []string{config.ZNetVersion},
	})
}
