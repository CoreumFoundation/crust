package znet

import (
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
)

// DevMode is the environment for developer
func DevMode(appF *apps.Factory) infra.Mode {
	node, coredNodes, err := appF.CoredNetwork("coredev", 1, 0)
	must.OK(err)

	faucet, err := appF.Faucet("faucet", node)
	must.OK(err)

	var mode infra.Mode
	mode = append(mode, coredNodes...)
	mode = append(mode, faucet)
	mode = append(mode, appF.BlockExplorer("explorer", node)...)
	return mode
}

// TestMode returns environment used for testing
func TestMode(appF *apps.Factory) infra.Mode {
	node, coredNodes, err := appF.CoredNetwork("coretest", 3, 0)
	must.OK(err)

	faucet, err := appF.Faucet("faucet", node)
	must.OK(err)

	var mode infra.Mode
	mode = append(mode, coredNodes...)
	mode = append(mode, faucet)
	return mode
}
