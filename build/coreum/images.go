package coreum

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
)

// BuildCoredDockerImage builds cored docker image
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(BuildCoredInDocker)

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		BinaryPath: dockerBinaryPath,
	})
}
