package coreum

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/docker/basic"
)

// BuildCoredDockerImage builds cored docker image
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(BuildCoredInDocker)

	dockerfile, err := basic.Execute(basic.Data{
		From:   docker.AlpineImage,
		Binary: filepath.Base(dockerBinaryPath),
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		ContextDir: filepath.Dir(dockerBinaryPath),
		ImageName:  filepath.Base(dockerBinaryPath),
		Dockerfile: dockerfile,
	})
}
