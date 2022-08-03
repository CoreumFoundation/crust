package tests

import (
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
	"github.com/CoreumFoundation/crust/tests/auth"
	"github.com/CoreumFoundation/crust/tests/bank"
	"github.com/CoreumFoundation/crust/tests/wasm"
)

// TODO (ysv): check if we can adapt our tests to run standard go testing framework

// Tests returns testing environment and tests
func Tests(appF *apps.Factory) (infra.Mode, []*testing.T) {
	mode := appF.CoredNetwork("coretest", 3, 0)
	node := mode[0].(cored.Cored)
	nodes := []cored.Cored{
		node,
		mode[1].(cored.Cored),
		mode[2].(cored.Cored),
	}

	tests := []*testing.T{
		testing.New(auth.TestUnexpectedSequenceNumber(node)),
		testing.New(bank.TestInitialBalance(node)),
		testing.New(bank.TestCoreTransfer(node)),
		testing.New(wasm.TestSimpleStateContract(node)),
		testing.New(wasm.TestBankSendContract(node)),
	}

	// The idea is to run 200 transfer transactions to be sure that none of them uses more gas than we assumed.
	// To make each faster the same test is started 10 times, each broadcasting 20 transactions, to make use of parallelism
	// implemented inside testing framework. Test itself is written serially to not fight for resources with other tests.
	// In the future, once we have more tests running in parallel, we will replace 10 tests running 20 transactions each
	// with a single one running 200 of them.
	for i := 0; i < 10; i++ {
		tests = append(tests, testing.New(bank.TestTransferMaximumGas(nodes[i%len(nodes)], 20)))
	}

	return mode, tests
}
