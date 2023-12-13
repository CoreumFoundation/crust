package rust

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/crust/build/tools"
)

// BuildSmartContract builds smart contract.
func BuildSmartContract(
	ctx context.Context,
	deps build.DepsFunc,
	path string,
) error {
	deps(EnsureRust, EnsureWASMOpt)

	if err := os.MkdirAll(filepath.Join(path, "artifacts"), 0o700); err != nil {
		return errors.WithStack(err)
	}

	targetPath := filepath.Join(tools.CacheDir(), "wasm", "target")
	if err := os.MkdirAll(targetPath, 0o700); err != nil {
		return errors.WithStack(err)
	}

	cmdCargo := exec.Command(tools.Path("bin/cargo", tools.TargetPlatformLocal),
		"build",
		"--release",
		"--target", "wasm32-unknown-unknown",
		"--target-dir", targetPath,
	)
	cmdCargo.Env = append(os.Environ(), `RUSTFLAGS=-C link-arg=-s`)
	cmdCargo.Dir = path

	contractFile := strings.ReplaceAll(filepath.Base(path), "-", "_") + ".wasm"
	cmdWASMOpt := exec.Command(tools.Path("bin/wasm-opt", tools.TargetPlatformLocal),
		"-Os", "--signext-lowering",
		"-o", filepath.Join("artifacts", contractFile),
		filepath.Join(targetPath, "wasm32-unknown-unknown", "release", contractFile),
	)
	cmdWASMOpt.Dir = path

	return libexec.Exec(ctx, cmdCargo, cmdWASMOpt)
}
