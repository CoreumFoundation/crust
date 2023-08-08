//go:build tools

package tools

// https://github.com/golang/go/issues/25922
// FIXME (wojtek): Try to generate go project dynamically in tmp directory to install those dependencies
import (
	_ "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway"
	_ "github.com/regen-network/cosmos-proto/protoc-gen-gocosmos"
)
