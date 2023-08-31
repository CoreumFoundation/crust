package osmosis

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
	binaryName = "osmosisd"
	binaryPath = "bin/" + binaryName
)

// BuildDockerImage builds docker image of the osmosis.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	if err := tools.Ensure(ctx, tools.Osmosis, tools.PlatformDockerLocal); err != nil {
		return err
	}

	binaryLocalPath := filepath.Join("bin", ".cache", binaryName, tools.PlatformDockerLocal.String())
	if err := tools.CopyToolBinaries(tools.Osmosis, tools.PlatformDockerLocal, binaryLocalPath, binaryPath); err != nil {
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
