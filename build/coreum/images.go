package coreum

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/coreum/image"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildCoredDockerImage builds cored docker image
func BuildCoredDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureCosmovisor, BuildCoredInDocker)

	dockerfile, err := image.Execute(image.Data{
		From:             docker.AlpineImage,
		CoredBinary:      filepath.Base(dockerBinaryPath),
		CosmovisorBinary: "cosmovisor",
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
	if err := tools.EnsureLocal(ctx, tools.Cosmovisor); err != nil {
		return err
	}

	absPath, err := filepath.EvalSymlinks("bin/.cache/cosmovisor")
	if err != nil {
		return errors.WithStack(err)
	}
	fr, err := os.Open(absPath)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fr.Close()

	if err := os.MkdirAll("bin/.cache/docker/cored", 0o700); err != nil {
		return err
	}
	fw, err := os.OpenFile("bin/.cache/docker/cored/cosmovisor", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return errors.WithStack(err)
	}
	defer fw.Close()

	_, err = io.Copy(fw, fr)
	return errors.WithStack(err)
}
