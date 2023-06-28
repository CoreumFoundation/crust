package crust

import (
	"context"
	"os"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
)

// Remove removes all the resources used by crust.
func Remove(ctx context.Context, deps build.DepsFunc) error {
	if err := docker.Remove(ctx); err != nil {
		return err
	}

	return errors.WithStack(os.RemoveAll(tools.CacheDir()))
}
