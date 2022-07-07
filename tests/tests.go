package tests

import (
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/testing"

	testscoreum "github.com/CoreumFoundation/coreum/tests"
)

// TODO (ysv): check if we can adapt our tests to run standard go testing framework

// Tests returns testing environment and tests
func Tests(appF *apps.Factory) (infra.Mode, []*testing.T) {
	mode := appF.CoredNetwork("coretest", 3, 0)
	return mode,
		integrateTests(testscoreum.Tests(mode))
}

func integrateTests(testSets ...[]*testing.T) []*testing.T {
	var tests []*testing.T

	for _, ts := range testSets {
		tests = append(tests, ts...)
	}

	return tests
}
