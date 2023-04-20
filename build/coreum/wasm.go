package coreum

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/tools"
)

// WASM compiles all the smart contracts.
func WASM(ctx context.Context, deps build.DepsFunc) error {
	deps(WASMBankSend, WASMFT, WASMNFT, WASMSimpleState)
	return nil
}

// WASMBankSend compiles bank-send smart contract.
func WASMBankSend(ctx context.Context, deps build.DepsFunc) error {
	return compileWASMContract(ctx, filepath.Join(repoPath, "integration-tests", "modules", "testdata", "wasm", "bank-send"))
}

// WASMFT compiles ft smart contract.
func WASMFT(ctx context.Context, deps build.DepsFunc) error {
	return compileWASMContract(ctx, filepath.Join(repoPath, "integration-tests", "modules", "testdata", "wasm", "ft"))
}

// WASMNFT compiles nft smart contract.
func WASMNFT(ctx context.Context, deps build.DepsFunc) error {
	return compileWASMContract(ctx, filepath.Join(repoPath, "integration-tests", "modules", "testdata", "wasm", "nft"))
}

// WASMSimpleState compiles simple-state smart contract.
func WASMSimpleState(ctx context.Context, deps build.DepsFunc) error {
	return compileWASMContract(ctx, filepath.Join(repoPath, "integration-tests", "modules", "testdata", "wasm", "simple-state"))
}

func compileWASMContract(ctx context.Context, path string) error {
	log := logger.Get(ctx)
	log.Info("Compiling WASM smart contract", zap.String("path", path))

	path, err := filepath.Abs(path)
	if err != nil {
		return errors.WithStack(err)
	}

	// FIXME (wojtek): Remove this once we use official sdk crate
	sdkPath, err := filepath.Abs(filepath.Join(repoPath, "integration-tests", "modules", "testdata", "wasm", "sdk"))
	if err != nil {
		return errors.WithStack(err)
	}

	targetCachePath := filepath.Join(tools.CacheDir(), "wasm", "targets", fmt.Sprintf("%x", sha256.Sum256([]byte(path))))
	if err := os.MkdirAll(targetCachePath, 0o700); err != nil {
		return errors.WithStack(err)
	}

	registryCachePath := filepath.Join(tools.CacheDir(), "wasm", "registry")
	if err := os.MkdirAll(registryCachePath, 0o700); err != nil {
		return errors.WithStack(err)
	}

	cmd := exec.Command("docker", "run", "--rm",
		"-v", sdkPath+":/sdk", "-v", path+":/code",
		"-v", registryCachePath+":/usr/local/cargo/registry",
		"-v", targetCachePath+":/code/target",
		"-e", "HOME=/tmp",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"cosmwasm/rust-optimizer:0.12.13")

	return libexec.Exec(ctx, cmd)
}
