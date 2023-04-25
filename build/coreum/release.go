package coreum

import (
	"context"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

// ReleaseCored releases cored binary for amd64 and arm64 to be published inside the release.
func ReleaseCored(ctx context.Context, deps build.DepsFunc) error {
	if tools.PlatformLocal != tools.PlatformLinuxAMD64 {
		return errors.New("this task can be executed on linux/amd64 machine only")
	}

	clean, _, err := git.StatusClean(ctx, repoPath)
	if err != nil {
		return err
	}
	if !clean {
		return errors.New("released commit contains uncommitted changes")
	}

	tags, err := git.HeadTags(ctx, repoPath)
	if err != nil {
		return err
	}
	if firstVersionTag(tags) == "" {
		return errors.New("no version present on released commit")
	}

	if err := buildCoredInDocker(ctx, deps, tools.PlatformDockerAMD64); err != nil {
		return err
	}
	return buildCoredInDocker(ctx, deps, tools.PlatformDockerARM64)
}

// ReleaseCoredImage releases cored docker images for amd64 and arm64.
func ReleaseCoredImage(ctx context.Context, deps build.DepsFunc) error {
	deps(ReleaseCored)

	return buildCoredDockerImage(ctx,
		[]tools.Platform{tools.PlatformDockerAMD64, tools.PlatformDockerARM64},
		docker.ActionPush,
	)
}
