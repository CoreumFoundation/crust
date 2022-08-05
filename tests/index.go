package tests

import (
	"context"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// Tests returns testing environment and tests
func Tests(appF *apps.Factory) (infra.Mode, []*testing.T) {
	mode := appF.CoredNetwork("coretest", 3, 0)
	node := mode[0].(cored.Cored)

	tests := []*testing.T{
		testing.New(dummyTest(node)),
	}
	return mode, tests
}

// FIXME (wojtek): Leaving a dummy test until WASM is merged. After that it will be removed.
func dummyTest(_ cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	return func(ctx context.Context) error { return nil },
		func(ctx context.Context, t *testing.T) {}
}
