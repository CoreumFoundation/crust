package znet

import (
	"sort"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

var modeMap = map[string]func(appF *apps.Factory) infra.Mode{
	"dev":  DevMode,
	"test": TestMode,
}

var modes = func() []string {
	modes := make([]string, 0, len(modeMap))
	for m := range modeMap {
		modes = append(modes, m)
	}
	sort.Strings(modes)
	return modes
}()

// Modes returns list of available modes
func Modes() []string {
	return modes
}

// Mode creates mode
func Mode(appF *apps.Factory, mode string) (infra.Mode, error) {
	modeF, exists := modeMap[mode]
	if !exists {
		return nil, errors.Errorf("unknown mode %s", mode)
	}
	return modeF(appF), nil
}

// DevMode is the environment for developer
func DevMode(appF *apps.Factory) infra.Mode {
	node, coredNodes, err := appF.CoredNetwork("coredev", cored.DefaultPorts, 1, 0)
	must.OK(err)

	faucet := appF.Faucet("faucet", node)

	var mode infra.Mode
	mode = append(mode, coredNodes...)
	mode = append(mode, faucet)
	mode = append(mode, appF.BlockExplorer("explorer", node)...)
	return mode
}

// TestMode returns environment used for testing
func TestMode(appF *apps.Factory) infra.Mode {
	node, coredNodes, err := appF.CoredNetwork("coredev", cored.DefaultPorts, 3, 0)
	must.OK(err)

	faucet := appF.Faucet("faucetdev", node)

	var mode infra.Mode
	mode = append(mode, coredNodes...)
	mode = append(mode, faucet)
	return mode
}
