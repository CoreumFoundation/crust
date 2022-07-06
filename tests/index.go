package tests

import (
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
	"github.com/CoreumFoundation/crust/tests/auth"
	"github.com/CoreumFoundation/crust/tests/bank"
)

// TODO (ysv): check if we can adapt our tests to run standard go testing framework

// Tests returns testing environment and tests
func Tests(appF *apps.Factory) (infra.Mode, []*testing.T) {
	mode := appF.CoredNetwork("coretest", 3, 0)
	node0 := mode[0].(cored.Cored)
	node1 := mode[1].(cored.Cored)
	node2 := mode[2].(cored.Cored)
	return mode,
		[]*testing.T{
			testing.New(auth.TestUnexpectedSequenceNumber(node0)),
			testing.New(bank.TestInitialBalance(node0)),
			testing.New(bank.TestCoreTransfer(node0)),

			// Run the same test 9 times to reuse the parallelism implemented inside the testing framework to use CPUs fully.
			// Later on we will have more tests utilizing CPUs so this won't be needed. For now we take the benefit of running tests faster.
			testing.New(bank.TestTransferMaximumGas(node0)),
			testing.New(bank.TestTransferMaximumGas(node1)),
			testing.New(bank.TestTransferMaximumGas(node2)),
			testing.New(bank.TestTransferMaximumGas(node0)),
			testing.New(bank.TestTransferMaximumGas(node1)),
			testing.New(bank.TestTransferMaximumGas(node2)),
			testing.New(bank.TestTransferMaximumGas(node0)),
			testing.New(bank.TestTransferMaximumGas(node1)),
			testing.New(bank.TestTransferMaximumGas(node2)),
		}
}
