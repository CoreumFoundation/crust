package coreum

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/coreum/image"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildCoredDockerImage builds cored docker image.
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureCosmovisor, BuildCoredInDocker, buildCoredWithFakeUpgrade)

	dockerfile, err := image.Execute(image.Data{
		From:             docker.AlpineImage,
		CoredBinary:      filepath.Base(dockerBinaryPath),
		CosmovisorBinary: "cosmovisor",
		Networks:         []string{"coreum-devnet-1"},
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   "../coreum",
		ContextDir: filepath.Dir(dockerBinaryPath),
		ImageName:  filepath.Base(dockerBinaryPath),
		Dockerfile: dockerfile,
	})
}

func ensureCosmovisor(ctx context.Context, deps build.DepsFunc) error {
	if err := tools.EnsureDocker(ctx, tools.Cosmovisor); err != nil {
		return err
	}
	cosmovisorLocalPath := filepath.Join("bin", ".cache", "docker", "cored")

	return tools.CopyToolBinaries(tools.Cosmovisor, cosmovisorLocalPath, "cosmovisor")
}
