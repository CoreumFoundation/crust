package apps

import (
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/hermes"
	"github.com/CoreumFoundation/crust/infra/apps/osmosis"
	"github.com/CoreumFoundation/crust/infra/apps/relayercosmos"
)

// TestGroup constant values.
const (
	TestGroupCoreumModules = "coreum-modules"
	TestGroupCoreumUpgrade = "coreum-upgrade"
	TestGroupCoreumIBC     = "coreum-ibc"
	TestGroupFaucet        = "faucet"
)

// AppPrefix constants are the prefixes used in the app factories.
const (
	AppPrefixCored      = "cored"
	AppPrefixIBC        = "ibc"
	AppPrefixExplorer   = "explorer"
	AppPrefixMonitoring = "monitoring"
)

// Predefined Profiles.
const (
	Profile1Cored                  = "1cored"
	Profile3Cored                  = "3cored"
	Profile5Cored                  = "5cored"
	ProfileIBC                     = "ibc"
	ProfileFaucet                  = "faucet"
	ProfileExplorer                = "explorer"
	ProfileMonitoring              = "monitoring"
	ProfileIntegrationTestsIBC     = "integration-tests-ibc"
	ProfileIntegrationTestsModules = "integration-tests-modules"
)

var profiles = []string{
	Profile1Cored,
	Profile3Cored,
	Profile5Cored,
	ProfileIBC,
	ProfileFaucet,
	ProfileExplorer,
	ProfileMonitoring,
	ProfileIntegrationTestsIBC,
	ProfileIntegrationTestsModules,
}

var (
	defaultProfiles = []string{Profile1Cored}
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

// BuildAppSet builds the application set to deploy based on provided profiles.
func BuildAppSet(appF *Factory, profiles []string, coredVersion string) (infra.AppSet, error) {
	pMap := map[string]bool{}
	pMap, err := checkProfiles(profiles, pMap)
	if err != nil {
		return nil, err
	}

	if pMap[ProfileIntegrationTestsIBC] || pMap[ProfileIntegrationTestsModules] {
		if pMap[Profile1Cored] {
			return nil, errors.Errorf("profile 1cored can't be used together with integration-tests as it requires 3cored or 5cored")
		}
		if !pMap[Profile5Cored] {
			pMap[Profile3Cored] = true
		}
	}

	if pMap[ProfileIntegrationTestsIBC] {
		pMap[ProfileIBC] = true
	}

	if (pMap[ProfileIBC] || pMap[ProfileFaucet] || pMap[ProfileExplorer] || pMap[ProfileMonitoring]) && !pMap[Profile3Cored] && !pMap[Profile5Cored] {
		pMap[Profile1Cored] = true
	}

	numOfCoredValidators := decideNumOfCoredValidators(pMap)

	var coredApp cored.Cored
	var appSet infra.AppSet

	coredApp, coredNodes, err := appF.CoredNetwork(AppPrefixCored, cored.DefaultPorts, numOfCoredValidators, 0, coredVersion)
	if err != nil {
		return nil, err
	}
	for _, coredNode := range coredNodes {
		appSet = append(appSet, coredNode)
	}

	if pMap[ProfileIBC] {
		appSet = append(appSet, appF.IBC(AppPrefixIBC, coredApp)...)
	}

	var faucetApp faucet.Faucet
	if pMap[ProfileFaucet] {
		appSet = append(appSet, appF.Faucet(string(faucet.AppType), coredApp))
	}

	if pMap[ProfileExplorer] {
		appSet = append(appSet, appF.BlockExplorer(AppPrefixExplorer, coredApp).ToAppSet()...)
	}

	if pMap[ProfileMonitoring] {
		var bdJunoApp bdjuno.BDJuno
		if bdJunoAppSetApp, ok := appSet.FindAppByName(
			BuildPrefixedAppName(AppPrefixExplorer, string(bdjuno.AppType)),
		).(bdjuno.BDJuno); ok {
			bdJunoApp = bdJunoAppSetApp
		}

		var hermesApp hermes.Hermes
		if hermesAppSetApp, ok := appSet.FindAppByName(
			BuildPrefixedAppName(AppPrefixIBC, string(hermes.AppType), string(gaiad.AppType)),
		).(hermes.Hermes); ok {
			hermesApp = hermesAppSetApp
		}

		var relayerCosmosApp relayercosmos.Relayer
		if relayerCosmosAppSetApp, ok := appSet.FindAppByName(
			BuildPrefixedAppName(AppPrefixIBC, string(relayercosmos.AppType), string(osmosis.AppType)),
		).(relayercosmos.Relayer); ok {
			relayerCosmosApp = relayerCosmosAppSetApp
		}

		appSet = append(appSet, appF.Monitoring(
			AppPrefixMonitoring,
			coredNodes,
			faucetApp,
			bdJunoApp,
			hermesApp,
			relayerCosmosApp,
		)...)
	}

	return appSet, nil
}

func decideNumOfCoredValidators(pMap map[string]bool) int {
	switch {
	case pMap[Profile1Cored]:
		return 1
	case pMap[Profile3Cored]:
		return 3
	case pMap[Profile5Cored]:
		return 5
	default:
		return 1
	}
}

func checkProfiles(profiles []string, pMap map[string]bool) (map[string]bool, error) {
	coredProfilePresent := false
	for _, p := range profiles {
		if _, ok := availableProfiles[p]; !ok {
			return nil, errors.Errorf("profile %s does not exist", p)
		}
		if p == Profile1Cored || p == Profile3Cored || p == Profile5Cored {
			if coredProfilePresent {
				return nil, errors.Errorf("profiles 1cored, 3cored and 5cored are mutually exclusive")
			}
			coredProfilePresent = true
		}
		pMap[p] = true
	}

	return pMap, nil
}
