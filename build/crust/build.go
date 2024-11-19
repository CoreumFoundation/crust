package crust

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

// BuildBuilder builds building tool in the current repository.
func BuildBuilder(ctx context.Context, deps types.DepsFunc) error {
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd/builder",
		BinOutputPath:  must.String(filepath.EvalSymlinks(must.String(os.Executable()))),
	})
}

// BuildZNet builds znet.
func BuildZNet(ctx context.Context, deps types.DepsFunc) error {
	outDir := "bin/.cache"
	items, err := os.ReadDir(outDir)
	if err != nil {
		return errors.WithStack(err)
	}
	for _, item := range items {
		if !item.Type().IsDir() && strings.HasPrefix(item.Name(), "znet") {
			if err := os.Remove(filepath.Join(outDir, item.Name())); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd/znet",
		BinOutputPath:  filepath.Join(outDir, "znet-"+tools.Version()),
		CGOEnabled:     true,
	})
}
