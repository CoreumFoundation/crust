package rust

import (
	"context"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/crust/build/tools"
)

// EnsureRust ensures that rust is available.
func EnsureRust(ctx context.Context, _ build.DepsFunc) error {
	return tools.Ensure(ctx, tools.Rust, tools.TargetPlatformLocal)
}

// EnsureWASMOpt ensures that wasm-opt is available.
func EnsureWASMOpt(ctx context.Context, _ build.DepsFunc) error {
	return tools.Ensure(ctx, tools.WASMOpt, tools.TargetPlatformLocal)
}
