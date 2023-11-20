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

// Tags used to build our docker images.
const (
	AlpineImage = "alpine:3.18.4"
	UbuntuImage = "ubuntu:23.10"
)

// Label used to tag docker resources created by crust.
const (
	LabelKey   = "com.coreum.crust"
	LabelValue = "crust"
)

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

	// TargetPlatforms is the list of platforms to build the image for
	TargetPlatforms []tools.TargetPlatform

	// Dockerfile contains dockerfile for build
	Dockerfile []byte

	// Action is the action to take after building the image
	Action Action

	// Username to use in the tags
	Username string

	// Versions to add to the image tags, despite standard ones (commit hash, version)
	Versions []string
}

// dockerBuildParamsInput is used to omit telescope antipattern.
type dockerBuildParamsInput struct {
	imageName     string
	contextDir    string
	commitHash    string
	gitVersions   []string
	otherVersions []string
	username      string
	platforms     []tools.TargetPlatform
	action        Action
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

	versionsFromGit, err := git.HeadTags(ctx, config.RepoPath)
	if err != nil {
		return err
	}

	buildParams := getDockerBuildParams(ctx, dockerBuildParamsInput{
		imageName:     config.ImageName,
		contextDir:    contextDir,
		commitHash:    commitHash,
		gitVersions:   versionsFromGit,
		otherVersions: config.Versions,
		username:      config.Username,
		platforms:     config.TargetPlatforms,
		action:        config.Action,
	})

	logger.Get(ctx).Info("Building docker images", zap.Any("build params", buildParams))
	buildCmd := exec.Command("docker", buildParams...)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}

// getTagsForDockerImage returns params for further use in "docker build" command.
func getDockerBuildParams(ctx context.Context, input dockerBuildParamsInput) []string {
	params := []string{
		"buildx",
		"build",
		"--builder", "crust",
		"--label", LabelKey + "=" + LabelValue,
	}

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

	versions := append([]string{}, input.otherVersions...)
	if input.commitHash != "" {
		versions = append(versions, input.commitHash[:7])
	}

	log := logger.Get(ctx)
	r := regexp.MustCompile(`^v(\d+\.)(\d+\.)(\*|\d+)(-rc(\d+)?)?$`) // v1.1.1 || v0.0.1-rc1 etc
	for _, version := range input.gitVersions {
		if r.MatchString(version) {
			versions = append(versions, version)
		} else {
			log.Info("Skipped HEAD tag because it doesn't fit regex", zap.String("tag", version))
		}
	}
	for _, version := range versions {
		if input.username == "" {
			params = append(params, "-t", fmt.Sprintf("%s:%s", input.imageName, version))
		} else {
			params = append(params, "-t", fmt.Sprintf("%s/%s:%s", input.username, input.imageName, version))
		}
	}

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
