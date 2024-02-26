package testing

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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

// FIXME (wojtek): Simplify it to contain only one path once everything is migrated.
var testBinaries = map[string][]string{
	apps.TestGroupCoreumModules: {
		"coreum/bin/.cache/integration-tests/coreum-modules",
		"crust/bin/.cache/integration-tests/coreum-modules",
	},
	apps.TestGroupCoreumUpgrade: {
		"coreum/bin/.cache/integration-tests/coreum-upgrade",
		"crust/bin/.cache/integration-tests/coreum-upgrade",
	},
	apps.TestGroupCoreumIBC: {
		"coreum/bin/.cache/integration-tests/coreum-ibc",
		"crust/bin/.cache/integration-tests/coreum-ibc",
	},
	apps.TestGroupFaucet: {
		"faucet/bin/.cache/integration-tests/faucet",
		"crust/bin/.cache/integration-tests/faucet",
	},
}

// TestGroups is the list of available test groups.
var TestGroups = func() []string {
	testGroups := make([]string, 0, len(testBinaries))
	for tg := range testBinaries {
		testGroups = append(testGroups, tg)
	}
	sort.Strings(testGroups)
	return testGroups
}()

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
		if _, exists := testBinaries[tg]; !exists {
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
		case apps.TestGroupCoreumModules, apps.TestGroupCoreumUpgrade, apps.TestGroupCoreumIBC:
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

			if tg == apps.TestGroupCoreumIBC {
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
		case apps.TestGroupFaucet:
			faucetApp := appSet.FindRunningAppByName(string(faucet.AppType))
			if faucetApp == nil {
				return errors.New("no running faucet app found")
			}
			faucetNode := faucetApp.(faucet.Faucet)
			fullArgs = append(fullArgs,
				"-faucet-address", infra.JoinNetAddr("http", faucetNode.Info().HostFromHost, faucetNode.Port()),
			)
		}

		var binPath string
	loop:
		for _, path := range testBinaries[tg] {
			path = filepath.Join(config.RootDir, path)
			_, err := os.Stat(path)
			switch {
			case err == nil:
				binPath = path
				break loop
			case os.IsNotExist(err):
			default:
				return errors.Wrapf(err, "cannot find binary for test group %q", tg)
			}
		}

		log := log.With(zap.String("binary", binPath), zap.Strings("args", fullArgs))
		log.Info("Running tests")

		if err := libexec.Exec(ctx, exec.Command(binPath, fullArgs...)); err != nil {
			return errors.Wrap(err, "tests failed")
		}
	}
	log.Info("All tests succeeded")
	return nil
}
