package relayer

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/config"
	"github.com/CoreumFoundation/crust/build/docker"
	dockerbasic "github.com/CoreumFoundation/crust/build/docker/basic"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	binaryName = "relayercosmos"
	binaryPath = "bin/" + binaryName
)

// BuildDockerImage builds docker image of the ibc relayer.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	tRelayer, err := tools.Get(tools.RelayerCosmos)
	if err != nil {
		return err
	}
	if err := tRelayer.Ensure(ctx, tools.PlatformDockerLocal); err != nil {
		return err
	}

	relayerLocalPath := filepath.Join("bin", ".cache", binaryName, tools.PlatformDockerLocal.String())
	if err := tools.CopyToolBinaries(tools.RelayerCosmos, tools.PlatformDockerLocal, relayerLocalPath, binaryPath); err != nil {
		return err
	}

	dockerfile, err := dockerbasic.Execute(dockerbasic.Data{
		From:   docker.UbuntuImage,
		Binary: binaryPath,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: relayerLocalPath,
		ImageName:  binaryName,
		Dockerfile: dockerfile,
		Versions:   []string{config.ZNetVersion},
	})
}
