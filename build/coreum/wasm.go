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

// Smart contract names.
const (
	WASMBankSend    = "bank-send"
	WASMFT          = "ft"
	WASMNFT         = "nft"
	WASMSimpleState = "simple-state"

	WASMIBCTransfer = "ibc-transfer"
	WASMIBCCall     = "ibc-call"

	WasmModulesDir = repoPath + "/integration-tests/modules/testdata/wasm"
	WasmIBCDir     = repoPath + "/integration-tests/ibc/testdata/wasm"
)

// CompileAllSmartContracts compiles all th smart contracts.
func CompileAllSmartContracts(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureRepo)

	if err := compileWasmDir(WasmModulesDir, deps); err != nil {
		return err
	}

	return compileWasmDir(WasmIBCDir, deps)
}

func compileWasmDir(dirPath string, deps build.DepsFunc) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return errors.WithStack(err)
	}

	actions := make([]build.CommandFunc, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		actions = append(actions, CompileSmartContract(dirPath, e.Name()))
	}
	deps(actions...)

	return nil
}

// CompileSmartContract returns function compiling smart contract.
func CompileSmartContract(dirPath, name string) build.CommandFunc {
	return func(ctx context.Context, deps build.DepsFunc) error {
		deps(ensureRepo)

		path := filepath.Join(dirPath, name)

		log := logger.Get(ctx)
		log.Info("Compiling WASM smart contract", zap.String("path", path))

		path, err := filepath.Abs(path)
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
			"-v", path+":/code",
			"-v", registryCachePath+":/usr/local/cargo/registry",
			"-v", targetCachePath+":/code/target",
			"-e", "HOME=/tmp",
			"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			"cosmwasm/rust-optimizer:0.12.13")

		return libexec.Exec(ctx, cmd)
	}
}
