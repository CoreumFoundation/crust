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

// AlpineImage contains tag of alpine image used to build dockerfiles.
const AlpineImage = "alpine:3.17.0"

// BuildImageConfig contains the configuration required to build docker image.
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

// dockerBuildParamsInput is used to omit telescope antipattern.
type dockerBuildParamsInput struct {
	imageName  string
	contextDir string
	commitHash string
	tags       []string
}

// BuildImage builds docker image.
func BuildImage(ctx context.Context, config BuildImageConfig) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}
	contextDir, err := filepath.Abs(config.ContextDir)
	if err != nil {
		return errors.WithStack(err)
	}

	commitHash, err := git.DirtyHeadHash(ctx, config.RepoPath)
	if err != nil {
		return err
	}

	tagsFromGit, err := git.HeadTags(ctx, config.RepoPath)
	if err != nil {
		return err
	}

	buildParams := getDockerBuildParams(ctx, dockerBuildParamsInput{
		imageName:  config.ImageName,
		contextDir: contextDir,
		commitHash: commitHash,
		tags:       tagsFromGit,
	})

	logger.Get(ctx).Info("Building docker images", zap.Any("build params", buildParams))
	buildCmd := exec.Command("docker", buildParams...)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}

// getTagsForDockerImage returns params for further use in "docker build" command.
func getDockerBuildParams(ctx context.Context, input dockerBuildParamsInput) []string {
	params := []string{"build", "-t", fmt.Sprintf("%s:znet", input.imageName)}

	if input.commitHash != "" {
		params = append(params, []string{"-t", fmt.Sprintf("%s:%s", input.imageName, input.commitHash[:7])}...)
	}

	r := regexp.MustCompile(`^v(\d+\.)(\d+\.)(\*|\d+)(-rc(\d+)?)?$`) // v1.1.1 || v0.0.1-rc1 etc
	for _, tag := range input.tags {
		if r.MatchString(tag) {
			params = append(params, []string{"-t", fmt.Sprintf("%s:%s", input.imageName, tag)}...)
		} else {
			logger.Get(ctx).Info("Skipped HEAD tag because it doesn't fit regex", zap.String("tag", tag))
		}
	}

	params = append(params, []string{"-f", "-", input.contextDir}...)

	return params
}
