package rust

import (
	"context"

	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

// EnsureRust ensures that rust is available.
func EnsureRust(ctx context.Context, _ types.DepsFunc) error {
	return tools.Ensure(ctx, tools.Rust, tools.TargetPlatformLocal)
}

// EnsureWASMOpt ensures that wasm-opt is available.
func EnsureWASMOpt(ctx context.Context, _ types.DepsFunc) error {
	return tools.Ensure(ctx, tools.WASMOpt, tools.TargetPlatformLocal)
}
