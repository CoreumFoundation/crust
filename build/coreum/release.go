package coreum

import (
	"context"
	"runtime"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	releaseAMD64BinaryPath = "bin/release/" + binaryName + "-linux-amd64"
	releaseARM64BinaryPath = "bin/release/" + binaryName + "-linux-arm64"
)

// ReleaseCored releases cored binary for amd64 and arm64 to be published inside the release.
func ReleaseCored(ctx context.Context, deps build.DepsFunc) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return errors.New("this task can be executed on linux/amd64 machine only")
	}

	deps(golang.EnsureGo, golang.EnsureLibWASMVMMuslC, ensureRepo)

	parameters, err := coredVersionParams(ctx)
	if err != nil {
		return err
	}

	config := golang.BinaryBuildConfig{
		PackagePath:    "../coreum/cmd/cored",
		BinOutputPath:  releaseAMD64BinaryPath,
		Parameters:     parameters,
		CGOEnabled:     true,
		Tags:           []string{"muslc"},
		LinkStatically: true,
	}

	if err := golang.BuildInDocker(ctx, config); err != nil {
		return err
	}

	config.BinOutputPath = releaseARM64BinaryPath
	config.CrosscompileARM64 = true

	return golang.BuildInDocker(ctx, config)
}
