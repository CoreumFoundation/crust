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
)

// AlpineImage contains tag of alpine image used to build dockerfiles
const AlpineImage = "alpine:3.16.0"

// BuildImageConfig contains the configuration required to build docker image
type BuildImageConfig struct {
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
	logger.Get(ctx).Info("Building docker image", zap.String("image", config.ImageName))

	buildCmd := exec.Command("docker", "build", "-t", config.ImageName, "-f", "-", contextDir)
	buildCmd.Stdin = bytes.NewReader(config.Dockerfile)

	return libexec.Exec(ctx, buildCmd)
}
