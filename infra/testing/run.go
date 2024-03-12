package testing

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/osmosis"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
)

// TestGroup constant values.
const (
	TestGroupCoreumModules = "coreum-modules"
	TestGroupCoreumUpgrade = "coreum-upgrade"
	TestGroupCoreumIBC     = "coreum-ibc"
	TestGroupFaucet        = "faucet"
)

// TestGroup describes test group.
type TestGroup struct {
	Binary           string
	RequiredProfiles []string
}

// TestGroups is the list of available test groups.
var TestGroups = map[string]TestGroup{
	TestGroupCoreumModules: {
		Binary:           "coreum/bin/.cache/integration-tests/coreum-modules",
		RequiredProfiles: []string{apps.Profile3Cored},
	},
	TestGroupCoreumUpgrade: {
		Binary:           "coreum/bin/.cache/integration-tests/coreum-upgrade",
		RequiredProfiles: []string{apps.Profile3Cored, apps.ProfileIBC},
	},
	TestGroupCoreumIBC: {
		Binary:           "coreum/bin/.cache/integration-tests/coreum-ibc",
		RequiredProfiles: []string{apps.Profile3Cored, apps.ProfileIBC},
	},
	TestGroupFaucet: {
		Binary:           "faucet/bin/.cache/integration-tests/faucet",
		RequiredProfiles: []string{apps.Profile1Cored, apps.ProfileFaucet},
	},
}

// Run deploys testing environment and runs tests there.
//
//nolint:funlen
func Run(
	ctx context.Context,
	target infra.Target,
	appSet infra.AppSet,
	coredApp cored.Cored,
	config infra.Config,
) error {
	for _, tg := range config.TestGroups {
		if _, exists := TestGroups[tg]; !exists {
			return errors.Errorf("test group %q does not exist", tg)
		}
	}

	if err := target.Deploy(ctx, appSet); err != nil {
		return err
	}

	args := []string{
		// The tests themselves are not computationally expensive, most of the time they spend waiting for transactions
		// to be included in blocks, so it should be safe to run more tests in parallel than we have CPus available.
		"-test.v", "-test.parallel", strconv.Itoa(2 * runtime.NumCPU()),
		"-coreum-grpc-address", infra.JoinNetAddr("", coredApp.Info().HostFromHost, coredApp.Config().Ports.GRPC),
	}

	log := logger.Get(ctx)
	if config.TestFilter != "" {
		log.Info("Running only tests matching filter", zap.String("filter", config.TestFilter))
		args = append(args, "-test.run", config.TestFilter)
	}

	// the execution order might be important
	for _, tg := range config.TestGroups {
		// copy is not used here, since the linter complains in the next line that using append with pre-allocated
		// length leads to extra space getting allocated.
		fullArgs := append([]string{}, args...)
		switch tg {
		case TestGroupCoreumModules, TestGroupCoreumUpgrade, TestGroupCoreumIBC:
			fullArgs = append(fullArgs,
				"-run-unsafe=true",
				"-coreum-funding-mnemonic", coredApp.Config().FundingMnemonic,
			)

			for _, m := range appSet {
				coredApp, ok := m.(cored.Cored)
				if ok && coredApp.Config().IsValidator && strings.HasPrefix(coredApp.Name(), string(cored.AppType)) {
					fullArgs = append(fullArgs, "-coreum-staker-mnemonic", coredApp.Config().StakerMnemonic)
				}
			}

			if tg == TestGroupCoreumIBC {
				fullArgs = append(
					fullArgs,
					"-coreum-rpc-address",
					infra.JoinNetAddr("http", coredApp.Info().HostFromHost, coredApp.Config().Ports.RPC),
				)

				gaiaNode := appSet.FindRunningAppByName(apps.BuildPrefixedAppName(apps.AppPrefixIBC, string(gaiad.AppType)))
				if gaiaNode == nil {
					return errors.New("no running ibc gaia app found")
				}
				gaiaApp := gaiaNode.(cosmoschain.BaseApp)

				fullArgs = append(fullArgs,
					"-gaia-grpc-address", infra.JoinNetAddr("", gaiaApp.Info().HostFromHost, gaiaApp.Ports().GRPC),
					"-gaia-rpc-address", infra.JoinNetAddr("http", gaiaApp.Info().HostFromHost, gaiaApp.Ports().RPC),
					"-gaia-funding-mnemonic", gaiaApp.AppConfig().FundingMnemonic,
				)

				osmosisNode := appSet.FindRunningAppByName(apps.BuildPrefixedAppName(apps.AppPrefixIBC, string(osmosis.AppType)))
				if osmosisNode == nil {
					return errors.New("no running ibc osmosis app found")
				}
				osmosisApp := osmosisNode.(cosmoschain.BaseApp)

				fullArgs = append(fullArgs,
					"-osmosis-grpc-address", infra.JoinNetAddr("", osmosisApp.Info().HostFromHost, osmosisApp.Ports().GRPC),
					"-osmosis-rpc-address", infra.JoinNetAddr("http", osmosisApp.Info().HostFromHost, osmosisApp.Ports().RPC),
					"-osmosis-funding-mnemonic", osmosisApp.AppConfig().FundingMnemonic,
				)
			}
		case TestGroupFaucet:
			faucetApp := appSet.FindRunningAppByName(string(faucet.AppType))
			if faucetApp == nil {
				return errors.New("no running faucet app found")
			}
			faucetNode := faucetApp.(faucet.Faucet)
			fullArgs = append(fullArgs,
				"-faucet-address", infra.JoinNetAddr("http", faucetNode.Info().HostFromHost, faucetNode.Port()),
			)
		}

		binPath := filepath.Join(config.RootDir, TestGroups[tg].Binary)

		log := log.With(zap.String("binary", binPath), zap.Strings("args", fullArgs))
		log.Info("Running tests")

		if err := libexec.Exec(ctx, exec.Command(binPath, fullArgs...)); err != nil {
			return errors.Wrap(err, "tests failed")
		}
	}
	log.Info("All tests succeeded")
	return nil
}
