package apps

import (
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/CoreumFoundation/coreum/v4/pkg/config/constant"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/hermes"
	"github.com/CoreumFoundation/crust/infra/apps/osmosis"
	"github.com/CoreumFoundation/crust/infra/apps/xrpl"
)

// AppPrefix constants are the prefixes used in the app factories.
const (
	AppPrefixCored      = "cored"
	AppPrefixIBC        = "ibc"
	AppPrefixExplorer   = "explorer"
	AppPrefixMonitoring = "monitoring"
	AppPrefixXRPL       = "xrpl"
	AppPrefixBridgeXRPL = "bridge-xrpl"
)

// Predefined Profiles.
const (
	Profile1Cored     = "1cored"
	Profile3Cored     = "3cored"
	Profile5Cored     = "5cored"
	ProfileDevNet     = "devnet"
	ProfileIBC        = "ibc"
	ProfileFaucet     = "faucet"
	ProfileExplorer   = "explorer"
	ProfileMonitoring = "monitoring"
	ProfileXRPL       = "xrpl"
	ProfileXRPLBridge = "bridge-xrpl"
)

var profiles = []string{
	Profile1Cored,
	Profile3Cored,
	Profile5Cored,
	ProfileDevNet,
	ProfileIBC,
	ProfileFaucet,
	ProfileExplorer,
	ProfileMonitoring,
	ProfileXRPL,
	ProfileXRPLBridge,
}

var defaultProfiles = []string{Profile1Cored}

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

// ValidateProfiles verifies that profie set is correct.
func ValidateProfiles(profiles []string) error {
	pMap := map[string]bool{}
	coredProfilePresent := false
	for _, p := range profiles {
		if _, ok := availableProfiles[p]; !ok {
			return errors.Errorf("profile %s does not exist", p)
		}
		if p == Profile1Cored || p == Profile3Cored || p == Profile5Cored || p == ProfileDevNet {
			if coredProfilePresent {
				return errors.Errorf("profiles 1cored, 3cored, 5cored and devnet are mutually exclusive")
			}
			coredProfilePresent = true
		}
		pMap[p] = true
	}

	return nil
}

// MergeProfiles removes redundant profiles from the list.
func MergeProfiles(pMap map[string]bool) map[string]bool {
	switch {
	case pMap[ProfileDevNet]:
		delete(pMap, Profile1Cored)
		delete(pMap, Profile3Cored)
		delete(pMap, Profile5Cored)
	case pMap[Profile5Cored]:
		delete(pMap, Profile1Cored)
		delete(pMap, Profile3Cored)
	case pMap[Profile3Cored]:
		delete(pMap, Profile1Cored)
	}

	return pMap
}

// BuildAppSet builds the application set to deploy based on provided profiles.
//
//nolint:funlen
func BuildAppSet(appF *Factory, profiles []string, coredVersion string) (infra.AppSet, cored.Cored, error) {
	pMap := lo.SliceToMap(profiles, func(profile string) (string, bool) {
		return profile, true
	})

	if pMap[ProfileIBC] || pMap[ProfileFaucet] || pMap[ProfileXRPLBridge] || pMap[ProfileExplorer] ||
		pMap[ProfileMonitoring] {
		pMap[Profile1Cored] = true
	}

	if pMap[ProfileXRPLBridge] {
		pMap[ProfileXRPL] = true
	}

	MergeProfiles(pMap)

	validatorCount, sentryCount, seedCount, fullCount := decideNumOfCoredNodes(pMap)

	var coredApp cored.Cored
	var appSet infra.AppSet

	coredApp, coredNodes, err := appF.CoredNetwork(
		cored.GenesisInitConfig{
			ChainID:       constant.ChainIDDev,
			Denom:         constant.DenomDev,
			DisplayDenom:  constant.DenomDevDisplay,
			AddressPrefix: constant.AddressPrefixDev,
			GenesisTime:   time.Now(),
			GovConfig: cored.GovConfig{
				MinDeposit:   sdk.NewCoins(sdk.NewInt64Coin(constant.DenomDev, 1000)),
				VotingPeriod: 20 * time.Second,
			},
			CustomParamsConfig: cored.CustomParamsConfig{
				MinSelfDelegation: sdkmath.NewInt(10_000_000),
			},
			BankBalances: []banktypes.Balance{
				// Faucet's account
				{
					Address: "devcore1ckuncyw0hftdq5qfjs6ee2v6z73sq0urd390cd",
					Coins:   sdk.NewCoins(sdk.NewCoin(constant.DenomDev, sdkmath.NewInt(100_000_000_000_000))),
				},
			},
		},
		AppPrefixCored,
		cored.DefaultPorts,
		validatorCount, sentryCount, seedCount, fullCount,
		coredVersion,
	)
	if err != nil {
		return nil, cored.Cored{}, err
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

		var hermesApps []hermes.Hermes
		if hermesAppSetApp, ok := appSet.FindAppByName(
			BuildPrefixedAppName(AppPrefixIBC, string(hermes.AppType), string(gaiad.AppType)),
		).(hermes.Hermes); ok {
			hermesApps = append(hermesApps, hermesAppSetApp)
		}

		if hermesAppSetApp, ok := appSet.FindAppByName(
			BuildPrefixedAppName(AppPrefixIBC, string(hermes.AppType), string(osmosis.AppType)),
		).(hermes.Hermes); ok {
			hermesApps = append(hermesApps, hermesAppSetApp)
		}

		appSet = append(appSet, appF.Monitoring(
			AppPrefixMonitoring,
			coredNodes,
			faucetApp,
			bdJunoApp,
			hermesApps,
		)...)
	}

	var xrplApp xrpl.XRPL
	if pMap[ProfileXRPL] {
		xrplApp = appF.XRPL(AppPrefixXRPL)
		appSet = append(appSet, xrplApp)
	}

	if pMap[ProfileXRPLBridge] {
		relayers, err := appF.BridgeXRPLRelayers(
			AppPrefixBridgeXRPL,
			coredApp,
			xrplApp,
			3,
		)
		if err != nil {
			return nil, cored.Cored{}, err
		}
		appSet = append(appSet, relayers...)
	}

	return appSet, coredApp, nil
}

func decideNumOfCoredNodes(pMap map[string]bool) (validatorCount, sentryCount, seedCount, fullCount int) {
	switch {
	case pMap[Profile1Cored]:
		return 1, 0, 0, 0
	case pMap[Profile3Cored]:
		return 3, 0, 0, 0
	case pMap[Profile5Cored]:
		return 5, 0, 0, 0
	case pMap[ProfileDevNet]:
		return 3, 1, 1, 2
	default:
		panic("no cored profile specified.")
	}
}
