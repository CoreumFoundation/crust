package crust

import (
	"context"
	"os"
	"path/filepath"

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
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "build/cmd/znet",
		BinOutputPath:  "bin/.cache/znet",
		CGOEnabled:     true,
	})
}

// BuildCrustZNet builds znet.
func BuildCrustZNet(ctx context.Context, deps types.DepsFunc) error {
	return golang.Build(ctx, deps, golang.BinaryBuildConfig{
		TargetPlatform: tools.TargetPlatformLocal,
		PackagePath:    "znet/cmd/znet",
		BinOutputPath:  "bin/.cache/znet",
		CGOEnabled:     true,
	})
}
