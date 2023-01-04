package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/git"
)

// AlpineImage contains tag of alpine image used to build dockerfiles
const AlpineImage = "alpine:3.17.0"

// BuildImageConfig contains the configuration required to build docker image
type BuildImageConfig struct {
	// RepoPath is the path to the repo where binary comes from
	RepoPath string

	// ContextDir
	ContextDir string

	// ImageName is the name of the image
	ImageName string

	// Dockerfile contains dockerfile for build
	Dockerfile []byte
}

// BuildImage builds docker image
func BuildImage(ctx context.Context, config BuildImageConfig) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}
	contextDir, err := filepath.Abs(config.ContextDir)
	if err != nil {
		return errors.WithStack(err)
	}

	tagFromCommit, err := git.DirtyHeadHash(ctx, config.RepoPath)
	if err != nil {
		return err
	}

	tagsFromGit, err := git.HeadTags(ctx, config.RepoPath)
	if err != nil {
		return err
	}

	buildParams := getDockerBuildParams(config.ImageName, contextDir, tagFromCommit, tagsFromGit, true)
	logger.Get(ctx).Info("Building docker images", zap.Any("build params", buildParams))
	buildCmd := exec.Command("docker", buildParams...)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}

// getTagsForDockerImage returns params for further use in "docker build" command
func getDockerBuildParams(imageName, contextDir, tagFromCommit string, tagsFromGit []string, tagForZnet bool) (tags []string) {
	tags = []string{"build", "-f", "-", contextDir}

	if tagForZnet {
		tags = append(tags, []string{"-t", fmt.Sprintf("%s:znet", imageName)}...)
	}

	if tagFromCommit != "" {
		tags = append(tags, []string{"-t", fmt.Sprintf("%s:%s", imageName, tagFromCommit[:7])}...)
	}

	for _, singleGitTag := range tagsFromGit {
		r := regexp.MustCompile(`^v(\d+\.)(\d+\.)(\*|\d+)(-rc(\d+)?)?$`) // v1.1.1 || v0.0.1-rc1 etc
		if r.MatchString(singleGitTag) {
			tags = append(tags, []string{"-t", fmt.Sprintf("%s:%s", imageName, singleGitTag)}...)
		}
	}

	return tags
}
