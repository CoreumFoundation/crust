package coreum

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum/pkg/config/constant"
	"github.com/CoreumFoundation/crust/build/coreum/image"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildCoredDockerImage builds cored docker image.
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureCosmovisor, BuildCoredInDocker, ensureReleasedBinaries(allReleases()))

	dockerfile, err := image.Execute(image.Data{
		From:             docker.AlpineImage,
		CoredBinary:      binaryName,
		CosmovisorBinary: cosmovisorBinaryName,
		Networks:         []string{string(constant.ChainIDDev), string(constant.ChainIDTest)},
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   "../coreum",
		ContextDir: dockerRootPath,
		ImageName:  dockerImageName,
		Dockerfile: dockerfile,
	})
}

func ensureCosmovisor(ctx context.Context, deps build.DepsFunc) error {
	if err := tools.EnsureDocker(ctx, tools.Cosmovisor); err != nil {
		return err
	}
	cosmovisorLocalPath := filepath.Join("bin", ".cache", "docker", "cored")

	return tools.CopyToolBinaries(tools.Cosmovisor, cosmovisorLocalPath, cosmovisorBinaryName)
}

// ensureReleasedBinaries ensures that all previous cored versions are installed.
func ensureReleasedBinaries(binaries []tools.Name) func(ctx context.Context, deps build.DepsFunc) error {
	return func(ctx context.Context, deps build.DepsFunc) error {
		for _, binaryTool := range binaries {
			if err := tools.EnsureDocker(ctx, binaryTool); err != nil {
				return err
			}

			if err := tools.CopyToolBinaries(binaryTool, dockerRootPath, string(binaryTool)); err != nil {
				return err
			}
		}

		return nil
	}
}
