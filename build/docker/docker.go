package docker

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"

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

	hash, err := git.DirtyHeadHash(ctx, config.RepoPath)
	if err != nil {
		return err
	}
	tagZNet := config.ImageName + ":znet"
	tag := config.ImageName + ":" + hash[:7]

	logger.Get(ctx).Info("Building docker image", zap.String("image", tag))

	buildCmd := exec.Command("docker", "build", "-t", tag, "-t", tagZNet, "-f", "-", contextDir)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}
