package rust

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
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

	cargo := struct {
		Package struct {
			Name string `toml:"name"`
		} `toml:"package"`
	}{}

	if _, err := toml.DecodeFile(filepath.Join(path, "Cargo.toml"), &cargo); err != nil {
		return errors.WithStack(err)
	}

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

	cmdCargo.Env = append(os.Environ(),
		"RUSTFLAGS=-C link-arg=-s",
		fmt.Sprintf("RUSTC=%s", tools.Path("bin/rustc", tools.TargetPlatformLocal)),
	)
	cmdCargo.Dir = path

	contractFile := strings.ReplaceAll(cargo.Package.Name, "-", "_") + ".wasm"
	cmdWASMOpt := exec.Command(tools.Path("bin/wasm-opt", tools.TargetPlatformLocal),
		"-Os", "--signext-lowering",
		"-o", filepath.Join("artifacts", contractFile),
		filepath.Join(targetPath, "wasm32-unknown-unknown", "release", contractFile),
	)
	cmdWASMOpt.Dir = path

	return libexec.Exec(ctx, cmdCargo, cmdWASMOpt)
}
