package znet

import (
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/tests"
)

// DevMode is the environment for developer
func DevMode(appF *apps.Factory) infra.Mode {
	coredNodes := appF.CoredNetwork("coredev", 1, 0, appF.Network)
	node := coredNodes[0].(cored.Cored)

	var mode infra.Mode
	mode = append(mode, coredNodes...)
	mode = append(mode, appF.BlockExplorer("explorer", node)...)
	return mode
}

// TestMode returns environment used for testing
func TestMode(appF *apps.Factory) infra.Mode {
	mode, _ := tests.Tests(appF)
	return mode
}
