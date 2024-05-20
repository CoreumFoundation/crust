package crust

import (
	"context"
	"os"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

// Remove removes all the resources used by crust.
func Remove(ctx context.Context, deps types.DepsFunc) error {
	if err := docker.Remove(ctx); err != nil {
		return err
	}

	return errors.WithStack(os.RemoveAll(tools.CacheDir()))
}
