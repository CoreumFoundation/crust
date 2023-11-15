package faucet

import (
	"context"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/config"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/faucet/image"
	"github.com/CoreumFoundation/crust/build/tools"
)

type imageConfig struct {
	Platforms []tools.Platform
	Action    docker.Action
	Username  string
	Versions  []string
}

// BuildDockerImage builds docker image of the faucet.
func BuildDockerImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Build)

	return buildDockerImage(ctx, imageConfig{
		Platforms: []tools.Platform{tools.PlatformLinuxLocalArchInDocker},
		Action:    docker.ActionLoad,
		Versions:  []string{config.ZNetVersion},
	})
}

func buildDockerImage(ctx context.Context, cfg imageConfig) error {
	dockerfile, err := image.Execute(image.Data{
		From:   docker.AlpineImage,
		Binary: binaryPath,
	})
	if err != nil {
		return err
	}

	return docker.BuildImage(ctx, docker.BuildImageConfig{
		RepoPath:   repoPath,
		ContextDir: filepath.Join("bin", ".cache", binaryName),
		ImageName:  binaryName,
		Platforms:  cfg.Platforms,
		Action:     cfg.Action,
		Versions:   cfg.Versions,
		Username:   cfg.Username,
		Dockerfile: dockerfile,
	})
}
