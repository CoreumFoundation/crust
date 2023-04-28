package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

// AlpineImage contains tag of alpine image used to build dockerfiles.
const AlpineImage = "alpine:3.17.0"

// Action is the action to take after building the image.
type Action int

const (
	// ActionLoad causes image to be loaded into local daemon.
	ActionLoad Action = iota

	// ActionPush causes image to be pushed to repository.
	ActionPush
)

// BuildImageConfig contains the configuration required to build docker image.
type BuildImageConfig struct {
	// RepoPath is the path to the repo where binary comes from
	RepoPath string

	// ContextDir
	ContextDir string

	// ImageName is the name of the image
	ImageName string

	// Platforms is the list of platforms to build the image for
	Platforms []tools.Platform

	// Dockerfile contains dockerfile for build
	Dockerfile []byte

	// Action is the action to take after building the image
	Action Action

	// Username to use in the tags
	Username string

	// Tags to add to the image, despite standard ones (commit hash, version)
	Tags []string
}

// dockerBuildParamsInput is used to omit telescope antipattern.
type dockerBuildParamsInput struct {
	imageName  string
	contextDir string
	commitHash string
	gitTags    []string
	otherTags  []string
	username   string
	platforms  []tools.Platform
	action     Action
}

// BuildImage builds docker image.
func BuildImage(ctx context.Context, config BuildImageConfig) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}

	if err := ensureBuilder(ctx); err != nil {
		return err
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
		gitTags:    tagsFromGit,
		otherTags:  config.Tags,
		username:   config.Username,
		platforms:  config.Platforms,
		action:     config.Action,
	})

	logger.Get(ctx).Info("Building docker images", zap.Any("build params", buildParams))
	buildCmd := exec.Command("docker", buildParams...)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}

// getTagsForDockerImage returns params for further use in "docker build" command.
func getDockerBuildParams(ctx context.Context, input dockerBuildParamsInput) []string {
	params := []string{"buildx", "build", "--builder", "crust"}

	switch input.action {
	case ActionLoad:
		params = append(params, "--load")
	case ActionPush:
		params = append(params, "--push")
	default:
		panic("unknown action")
	}

	for _, platform := range input.platforms {
		params = append(params, "--platform", fmt.Sprintf("linux/%s", platform.Arch))
	}

	tags := append([]string{}, input.otherTags...)
	if input.commitHash != "" {
		tags = append(tags, input.commitHash[:7])
	}

	log := logger.Get(ctx)
	r := regexp.MustCompile(`^v(\d+\.)(\d+\.)(\*|\d+)(-rc(\d+)?)?$`) // v1.1.1 || v0.0.1-rc1 etc
	for _, tag := range input.gitTags {
		if r.MatchString(tag) {
			tags = append(tags, tag)
		} else {
			log.Info("Skipped HEAD tag because it doesn't fit regex", zap.String("tag", tag))
		}
	}
	for _, tag := range tags {
		if input.username == "" {
			params = append(params, "-t", fmt.Sprintf("%s:%s", input.imageName, tag))
		} else {
			params = append(params, "-t", fmt.Sprintf("%s/%s:%s", input.username, input.imageName, tag))
		}
	}
	fmt.Println(params)

	params = append(params, []string{"-f", "-", input.contextDir}...)

	return params
}

func ensureBuilder(ctx context.Context) error {
	inspectCmd := exec.Command("docker", "buildx", "inspect", "crust")
	inspectCmd.Stderr = io.Discard
	err := libexec.Exec(ctx, inspectCmd)

	if err == nil {
		return nil
	}

	return libexec.Exec(ctx, exec.Command("docker",
		"buildx", "create", "--name", "crust", "--driver", "docker-container", "--bootstrap"))
}
