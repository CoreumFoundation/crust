package apps

import (
	"sort"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

var availableProfiles = map[string]bool{
	"1cored":            true,
	"3cored":            true,
	"faucet":            true,
	"explorer":          true,
	"integration-tests": true,
}

var (
	defaultProfiles          = []string{"1cored"}
	integrationTestsProfiles = []string{"integration-tests"}
)

var profileList = func() []string {
	keys := make([]string, 0, len(availableProfiles))
	for p := range availableProfiles {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	return keys
}()

// Profiles returns the list of available profiles
func Profiles() []string {
	return profileList
}

// DefaultProfiles returns the list of default profiles started if user didn't provide anything else
func DefaultProfiles() []string {
	return defaultProfiles
}

// IntegrationTestsProfiles returns the list of profiles started for integration tests
func IntegrationTestsProfiles() []string {
	return integrationTestsProfiles
}

// BuildAppSet builds the application set to deploy based on provided profiles
func BuildAppSet(appF *Factory, profiles []string) (infra.AppSet, error) {
	pMap := map[string]bool{}
	for _, p := range profiles {
		if !availableProfiles[p] {
			return nil, errors.Errorf("profile %s does not exist", p)
		}
		pMap[p] = true
	}

	if pMap["3cored"] && pMap["1cored"] {
		return nil, errors.Errorf("profiles 1cored and 3cored are mutually exclusive")
	}

	if pMap["integration-tests"] {
		if pMap["1cored"] {
			return nil, errors.Errorf("profile 1cored can't be used together with integration-tests as it requires 3cored")
		}
		pMap["3cored"] = true
		pMap["faucet"] = true
	}

	if (pMap["faucet"] || pMap["explorer"]) && !pMap["3cored"] {
		pMap["1cored"] = true
	}

	var numOfCoredValidators int
	switch {
	case pMap["1cored"]:
		numOfCoredValidators = 1
	case pMap["3cored"]:
		numOfCoredValidators = 3
	}

	var coredApp cored.Cored
	var appSet infra.AppSet

	if numOfCoredValidators > 0 {
		var err error
		var coredNodes infra.AppSet
		coredApp, coredNodes, err = appF.CoredNetwork("cored", cored.DefaultPorts, numOfCoredValidators, 0)
		if err != nil {
			return nil, err
		}
		appSet = append(appSet, coredNodes...)
	}

	if pMap["faucet"] {
		appSet = append(appSet, appF.Faucet("faucet", coredApp))
	}

	if pMap["explorer"] {
		appSet = append(appSet, appF.BlockExplorer("explorer", coredApp)...)
	}

	return appSet, nil
}
