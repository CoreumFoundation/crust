package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/parallel"
)

// Remove removes all the docker components used by crust.
func Remove(ctx context.Context) error {
	if err := removeContainers(ctx); err != nil {
		return err
	}
	if err := removeImages(ctx); err != nil {
		return err
	}
	return removeBuilder(ctx)
}

func removeContainers(ctx context.Context) error {
	return forContainer(ctx, func(ctx context.Context, containerID string, isRunning bool) error {
		log := logger.Get(ctx).With(zap.String("id", containerID))
		log.Info("Deleting container")

		if err := removeContainer(ctx, containerID, isRunning); err != nil {
			return err
		}

		log.Info("Container deleted")
		return nil
	})
}

func removeImages(ctx context.Context) error {
	return forImage(ctx, func(ctx context.Context, imageID string) error {
		log := logger.Get(ctx).With(zap.String("id", imageID))
		log.Info("Deleting image")

		if err := removeImage(ctx, imageID); err != nil {
			return err
		}

		log.Info("Image deleted")
		return nil
	})
}

func removeBuilder(ctx context.Context) error {
	inspectCmd := exec.Command("docker", "buildx", "inspect", "crust")
	inspectCmd.Stderr = io.Discard
	err := libexec.Exec(ctx, inspectCmd)

	if err == nil {
		return libexec.Exec(ctx, noStdout(exec.Command("docker", "buildx", "rm", "crust")))
	}

	return nil
}

func forContainer(ctx context.Context, fn func(ctx context.Context, containerID string, isRunning bool) error) error {
	listBuf := &bytes.Buffer{}
	listCmd := exec.Command("docker", "ps", "-aq", "--no-trunc", "--filter", "label="+LabelKey+"="+LabelValue)
	listCmd.Stdout = listBuf
	if err := libexec.Exec(ctx, listCmd); err != nil {
		return err
	}

	listStr := strings.TrimSuffix(listBuf.String(), "\n")
	if listStr == "" {
		return nil
	}

	inspectBuf := &bytes.Buffer{}
	inspectCmd := exec.Command("docker", append([]string{"inspect"}, strings.Split(listStr, "\n")...)...)
	inspectCmd.Stdout = inspectBuf

	if err := libexec.Exec(ctx, inspectCmd); err != nil {
		return err
	}

	var info []struct {
		ID    string `json:"Id"` //nolint:tagliatelle // `Id` is defined by docker
		Name  string
		State struct {
			Running bool
		}
		Config struct {
			Labels map[string]string
		}
	}

	if err := json.Unmarshal(inspectBuf.Bytes(), &info); err != nil {
		return errors.Wrap(err, "unmarshalling container properties failed")
	}

	return parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		for _, cInfo := range info {
			spawn("container."+cInfo.ID, parallel.Continue, func(ctx context.Context) error {
				return fn(ctx, cInfo.ID, cInfo.State.Running)
			})
		}
		return nil
	})
}

func forImage(ctx context.Context, fn func(ctx context.Context, imageID string) error) error {
	listBuf := &bytes.Buffer{}
	listCmd := exec.Command("docker", "image", "list", "-aq", "--no-trunc", "--filter", "label="+LabelKey+"="+LabelValue)
	listCmd.Stdout = listBuf
	if err := libexec.Exec(ctx, listCmd); err != nil {
		return err
	}

	listStr := strings.TrimSuffix(listBuf.String(), "\n")
	if listStr == "" {
		return nil
	}

	return parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		for _, imageID := range strings.Split(listStr, "\n") {
			spawn("image."+imageID, parallel.Continue, func(ctx context.Context) error {
				return fn(ctx, imageID)
			})
		}
		return nil
	})
}

func removeContainer(ctx context.Context, containerID string, isRunning bool) error {
	cmds := []*exec.Cmd{}
	if isRunning {
		// Everything will be removed, so we don't care about graceful shutdown
		cmds = append(cmds, noStdout(exec.Command("docker", "kill", containerID)))
	}
	if err := libexec.Exec(ctx, append(cmds, noStdout(exec.Command("docker", "rm", containerID)))...); err != nil {
		return errors.Wrapf(err, "deleting container `%s` failed", containerID)
	}
	return nil
}

func removeImage(ctx context.Context, imageID string) error {
	if err := libexec.Exec(ctx, noStdout(exec.Command("docker", "rmi", "-f", imageID))); err != nil {
		return errors.Wrapf(err, "deleting image `%s` failed", imageID)
	}
	return nil
}

func noStdout(cmd *exec.Cmd) *exec.Cmd {
	cmd.Stdout = io.Discard
	return cmd
}
