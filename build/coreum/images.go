package coreum

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum/pkg/config/constant"
	"github.com/CoreumFoundation/crust/build/coreum/image"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildCoredDockerImage builds cored docker image.
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(BuildCoredInDocker, ensureReleasedBinaries)

	return buildCoredDockerImage(ctx, []tools.Platform{tools.PlatformDockerLocal}, docker.ActionLoad)
}

func buildCoredDockerImage(ctx context.Context, platforms []tools.Platform, action docker.Action) error {
	for _, platform := range platforms {
		if err := ensureCosmovisor(ctx, platform); err != nil {
			return err
		}
	}
	dockerfile, err := image.Execute(image.Data{
		From:             docker.AlpineImage,
		CoredBinary:      binaryPath,
		CosmovisorBinary: cosmovisorBinaryPath,
		Networks: []string{
			string(constant.ChainIDDev),
			string(constant.ChainIDTest),
		},
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   repoPath,
		ContextDir: filepath.Join("bin", ".cache", binaryName),
		ImageName:  binaryName,
		Platforms:  platforms,
		Action:     action,
		Dockerfile: dockerfile,
	})
}

// ensureReleasedBinaries ensures that all previous cored versions are installed.
func ensureReleasedBinaries(ctx context.Context, deps build.DepsFunc) error {
	for _, binaryTool := range []tools.Name{
		tools.CoredV011,
	} {
		if err := tools.EnsureBinaries(ctx, binaryTool, tools.PlatformDockerLocal); err != nil {
			return err
		}

		if err := tools.CopyToolBinaries(binaryTool, tools.PlatformDockerLocal, filepath.Join("bin", ".cache", binaryName, tools.PlatformDockerLocal.String()), fmt.Sprintf("bin/%s", binaryTool)); err != nil {
			return err
		}
	}

	return nil
}
