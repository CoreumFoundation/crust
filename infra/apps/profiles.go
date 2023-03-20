package apps

import (
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

const (
	profile1Cored           = "1cored"
	profile3Cored           = "3cored"
	profile5Cored           = "5cored"
	profileIBC              = "ibc"
	profileFaucet           = "faucet"
	profileExplorer         = "explorer"
	profileMonitoring       = "monitoring"
	profileIntegrationTests = "integration-tests"
)

var profiles = []string{
	profile1Cored,
	profile3Cored,
	profile5Cored,
	profileIBC,
	profileFaucet,
	profileExplorer,
	profileMonitoring,
	profileIntegrationTests,
}

var (
	defaultProfiles          = []string{profile1Cored}
	integrationTestsProfiles = []string{profileIntegrationTests}
)

var availableProfiles = func() map[string]struct{} {
	v := map[string]struct{}{}
	for _, p := range profiles {
		v[p] = struct{}{}
	}
	return v
}()

// Profiles returns the list of available profiles.
func Profiles() []string {
	return profiles
}

// DefaultProfiles returns the list of default profiles started if user didn't provide anything else.
func DefaultProfiles() []string {
	return defaultProfiles
}

// IntegrationTestsProfiles returns the list of profiles started for integration tests.
func IntegrationTestsProfiles() []string {
	return integrationTestsProfiles
}

// BuildAppSet builds the application set to deploy based on provided profiles.
func BuildAppSet(appF *Factory, profiles []string, coredVersion string) (infra.AppSet, error) {
	pMap := map[string]bool{}
	coredProfilePresent := false
	for _, p := range profiles {
		if _, ok := availableProfiles[p]; !ok {
			return nil, errors.Errorf("profile %s does not exist", p)
		}
		if p == profile1Cored || p == profile3Cored || p == profile5Cored {
			if coredProfilePresent {
				return nil, errors.Errorf("profiles 1cored, 3cored and 5cored are mutually exclusive")
			}
			coredProfilePresent = true
		}
		pMap[p] = true
	}

	if pMap[profileIntegrationTests] {
		if pMap[profile1Cored] {
			return nil, errors.Errorf("profile 1cored can't be used together with integration-tests as it requires 3cored or 5cored")
		}
		if !pMap[profile5Cored] {
			pMap[profile3Cored] = true
		}
		// pMap[profileIBC] = true
		pMap[profileFaucet] = true
	}

	if (pMap[profileIBC] || pMap[profileFaucet] || pMap[profileExplorer] || pMap[profileMonitoring]) && !pMap[profile3Cored] && !pMap[profile5Cored] {
		pMap[profile1Cored] = true
	}

	var numOfCoredValidators int
	switch {
	case pMap[profile1Cored]:
		numOfCoredValidators = 1
	case pMap[profile3Cored]:
		numOfCoredValidators = 3
	case pMap[profile5Cored]:
		numOfCoredValidators = 5
	}

	var coredApp cored.Cored
	var appSet infra.AppSet

	var err error
	coredApp, coredNodes, err := appF.CoredNetwork("cored", cored.DefaultPorts, numOfCoredValidators, 0, coredVersion)
	if err != nil {
		return nil, err
	}
	for _, coredNode := range coredNodes {
		appSet = append(appSet, coredNode)
	}

	if pMap[profileIBC] {
		appSet = append(appSet, appF.IBC("ibc", coredApp)...)
	}

	if pMap[profileFaucet] {
		appSet = append(appSet, appF.Faucet("faucet", coredApp))
	}

	explorerApp := appF.BlockExplorer("explorer", coredApp)
	if pMap[profileExplorer] {
		appSet = append(appSet, explorerApp.ToAppSet()...)
	}

	// FIXME (dhi) test what happens if explorer disabled but monitoring is enabled
	if pMap[profileMonitoring] {
		appSet = append(appSet, appF.Monitoring("monitoring", coredNodes, explorerApp.BDJuno)...)
	}

	return appSet, nil
}
