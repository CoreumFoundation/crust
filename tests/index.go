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

			// The idea is to run 200 transfer transactions to be sure that none of them uses more gas than we assumed.
			// To make each faster the same test is started 10 times, each broadcasting 20 transactions, to make use of parallelism
			// implemented inside testing framework. Test itself is written serially to not fight for resources with other tests.
			// In the future, once we have more tests running in parallel, we will replace 10 tests running 20 transactions each
			// with a single one running 200 of them.
			testing.New(bank.TestTransferMaximumGas(node0, 20)),
			testing.New(bank.TestTransferMaximumGas(node1, 20)),
			testing.New(bank.TestTransferMaximumGas(node2, 20)),
			testing.New(bank.TestTransferMaximumGas(node0, 20)),
			testing.New(bank.TestTransferMaximumGas(node1, 20)),
			testing.New(bank.TestTransferMaximumGas(node2, 20)),
			testing.New(bank.TestTransferMaximumGas(node0, 20)),
			testing.New(bank.TestTransferMaximumGas(node1, 20)),
			testing.New(bank.TestTransferMaximumGas(node2, 20)),
			testing.New(bank.TestTransferMaximumGas(node2, 20)),
		}
}
