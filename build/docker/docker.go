package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

// BuildImageConfig contains the configuration required to build docker image
type BuildImageConfig struct {
	// BinaryPath is the path to binary file to include in the image
	BinaryPath string
}

// BuildImage builds docker image
func BuildImage(ctx context.Context, config BuildImageConfig) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}
	binPath, err := filepath.Abs(config.BinaryPath)
	if err != nil {
		return errors.WithStack(err)
	}
	contextDir := filepath.Dir(binPath)
	binName := filepath.Base(binPath)
	imageName := "coreum-" + binName

	logger.Get(ctx).Info("Building docker image", zap.String("binary", config.BinaryPath), zap.String("image", imageName))

	dockerFile := fmt.Sprintf(`FROM alpine:3.16.0
COPY %[1]s /bin/%[1]s
ENTRYPOINT ["%[1]s"]
`, binName)

	buildCmd := exec.Command("docker", "build", "-t", imageName, "-f", "-", contextDir)
	buildCmd.Stdin = bytes.NewReader([]byte(dockerFile))

	return libexec.Exec(ctx, buildCmd)
}
