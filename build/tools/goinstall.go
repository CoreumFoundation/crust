//go:build tools

package tools

// https://github.com/golang/go/issues/25922
// FIXME (wojtek): Try to generate go project dynamically in tmp directory to install those dependencies
import (
	_ "github.com/cosmos/gogoproto/protoc-gen-gocosmos"
	_ "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway"
)
