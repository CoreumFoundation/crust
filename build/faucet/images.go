package faucet

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/docker/basic"
)

// BuildDockerImage builds docker image of the faucet
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	dockerfile, err := basic.Execute(basic.Data{
		From:   docker.AlpineImage,
		Binary: filepath.Base(dockerBinaryPath),
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   "../faucet",
		ContextDir: filepath.Dir(dockerBinaryPath),
		ImageName:  filepath.Base(dockerBinaryPath),
		Dockerfile: dockerfile,
	})
}
