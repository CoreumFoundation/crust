package testing

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
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

// Run deploys testing environment and runs tests there.
func Run(ctx context.Context, target infra.Target, appSet infra.AppSet, config infra.Config, onlyTestGroups ...string) error { //nolint:funlen
	testDir := filepath.Join(config.BinDir, ".cache", "integration-tests")
	files, err := os.ReadDir(testDir)
	if err != nil {
		return errors.WithStack(err)
	}
	binaries := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		binaries = append(binaries, f.Name())
	}

	for _, tg := range onlyTestGroups {
		if !lo.Contains(binaries, tg) {
			return errors.Errorf("binary does not exist for test group %q", tg)
		}
	}
	// if not limitations were provided we test all binaries
	if len(onlyTestGroups) == 0 {
		onlyTestGroups = binaries
	}

	if err := target.Deploy(ctx, appSet); err != nil {
		return err
	}

	log := logger.Get(ctx)
	log.Info("Waiting until all applications start...")
	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer waitCancel()
	if err := infra.WaitUntilHealthy(waitCtx, buildWaitForApps(appSet)...); err != nil {
		return err
	}

	log.Info("All the applications are ready")
	coredApp := appSet.FindRunningAppByName("cored-00")
	if coredApp == nil {
		return errors.New("no running cored app found")
	}

	coredNode := coredApp.(cored.Cored)
	args := []string{
		// The tests themselves are not computationally expensive, most of the time they spend waiting for transactions
		// to be included in blocks, so it should be safe to run more tests in parallel than we have CPus available.
		"-test.v", "-test.parallel", strconv.Itoa(2 * runtime.NumCPU()),
		"-coreum-address", infra.JoinNetAddr("", coredNode.Info().HostFromHost, coredNode.Config().Ports.GRPC),
	}
	if config.TestFilter != "" {
		log.Info("Running only tests matching filter", zap.String("filter", config.TestFilter))
		args = append(args, "-test.run", config.TestFilter)
	}

	const (
		testGroupCoreumModules = "coreum-modules"
		testGroupCoreumUpgrade = "coreum-upgrade"
		testGroupCoreumIBC     = "coreum-ibc"
		testGroupFaucet        = "faucet"
	)

	var failed bool
	// the execution order might be important
	for _, onlyTestGroup := range onlyTestGroups {
		// copy is not used here, since the linter complains in the next line that using append with pre-allocated
		// length leads to extra space getting allocated.
		fullArgs := append([]string{}, args...)
		switch onlyTestGroup {
		case testGroupCoreumModules, testGroupCoreumUpgrade, testGroupCoreumIBC:

			fullArgs = append(fullArgs,
				"-run-unsafe=true",
				"-coreum-funding-mnemonic", coredNode.Config().FundingMnemonic,
			)

			for _, m := range appSet {
				coredApp, ok := m.(cored.Cored)
				if ok && coredApp.Config().IsValidator && strings.HasPrefix(coredApp.Name(), string(cored.AppType)) {
					fullArgs = append(fullArgs, "-coreum-staker-mnemonic", coredApp.Config().StakerMnemonic)
				}
			}

			if onlyTestGroup == testGroupCoreumIBC {
				gaiaNode := appSet.FindRunningAppByName(apps.BuildPrefixedAppName(apps.AppPrefixIBC, string(gaiad.AppType)))
				if gaiaNode == nil {
					return errors.New("no running ibc gaia app found")
				}
				gaiaApp := gaiaNode.(cosmoschain.BaseApp)

				fullArgs = append(fullArgs,
					"-gaia-address", infra.JoinNetAddr("", gaiaApp.Info().HostFromHost, gaiaApp.Ports().GRPC),
					"-gaia-funding-mnemonic", gaiaApp.AppConfig().FundingMnemonic,
				)

				osmosisNode := appSet.FindRunningAppByName(apps.BuildPrefixedAppName(apps.AppPrefixIBC, string(osmosis.AppType)))
				if osmosisNode == nil {
					return errors.New("no running ibc osmosis app found")
				}
				osmosisApp := osmosisNode.(cosmoschain.BaseApp)

				fullArgs = append(fullArgs,
					"-osmosis-address", infra.JoinNetAddr("", osmosisApp.Info().HostFromHost, osmosisApp.Ports().GRPC),
					"-osmosis-funding-mnemonic", osmosisApp.AppConfig().FundingMnemonic,
				)
			}
		case testGroupFaucet:
			faucetApp := appSet.FindRunningAppByName(string(faucet.AppType))
			if faucetApp == nil {
				return errors.New("no running faucet app found")
			}
			faucetNode := faucetApp.(faucet.Faucet)
			fullArgs = append(fullArgs,
				"-faucet-address", infra.JoinNetAddr("http", faucetNode.Info().HostFromHost, faucetNode.Port()),
			)
		}

		binPath := filepath.Join(testDir, onlyTestGroup)
		log := log.With(zap.String("binary", binPath), zap.Strings("args", fullArgs))
		log.Info("Running tests")

		if err := libexec.Exec(ctx, exec.Command(binPath, fullArgs...)); err != nil {
			log.Error("Tests failed", zap.Error(err))
			failed = true
		}
	}
	if failed {
		return errors.New("tests failed")
	}
	log.Info("All tests succeeded")
	return nil
}

func buildWaitForApps(appSet infra.AppSet) []infra.HealthCheckCapable {
	waitForApps := make([]infra.HealthCheckCapable, 0, len(appSet))
	for _, app := range appSet {
		withHealthCheck, ok := app.(infra.HealthCheckCapable)
		if !ok {
			withHealthCheck = infra.IsRunning(app)
		}
		waitForApps = append(waitForApps, withHealthCheck)
	}
	return waitForApps
}
