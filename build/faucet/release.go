package faucet

import (
	"context"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/config"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Release releases faucet binary for amd64 and arm64 to be published inside the release.
func Release(ctx context.Context, deps build.DepsFunc) error {
	clean, _, err := git.StatusClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return errors.New("released commit contains uncommitted changes")
	}

	version, err := git.VersionFromTag(ctx, repoPath)
	if err != nil {
		return err
	}
	if version == "" {
		return errors.New("no version present on released commit")
	}

	if err := buildFaucet(ctx, deps, tools.PlatformDockerAMD64); err != nil {
		return err
	}
	return buildFaucet(ctx, deps, tools.PlatformDockerARM64)
}

// ReleaseImage releases faucet docker images for amd64 and arm64.
func ReleaseImage(ctx context.Context, deps build.DepsFunc) error {
	deps(Release)

	return buildDockerImage(ctx, imageConfig{
		Platforms: []tools.Platform{tools.PlatformDockerAMD64, tools.PlatformDockerARM64},
		Action:    docker.ActionPush,
		Username:  config.DockerHubUsername,
	})
}
