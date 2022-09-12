package faucet

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
)

// BuildDockerImage builds docker image of the faucet
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		BinaryPath: dockerBinaryPath,
	})
}
