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
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
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
	coredApp := appSet.FindRunningApp(cored.AppType, "cored-00")
	if coredApp == nil {
		return errors.New("no running cored app found")
	}

	coredNode := coredApp.(cored.Cored)
	args := []string{
		// The tests themselves are not computationally expensive, most of the time they spend waiting for transactions
		// to be included in blocks, so it should be safe to run more tests in parallel than we have CPus available.
		"-test.parallel", strconv.Itoa(2 * runtime.NumCPU()),
		"-cored-address", infra.JoinNetAddr("", coredNode.Info().HostFromHost, coredNode.Config().Ports.GRPC),
	}
	if config.TestFilter != "" {
		log.Info("Running only tests matching filter", zap.String("filter", config.TestFilter))
		args = append(args, "-test.run", config.TestFilter)
	}
	if config.VerboseLogging {
		args = append(args, "-test.v")
	}

	var failed bool
	// the execution order might be important
	for _, onlyTestGroup := range onlyTestGroups {
		// copy is not used here, since the linter complains in the next line that using append with pre-allocated
		// length leads to extra space getting allocated.
		fullArgs := append([]string{}, args...)
		switch onlyTestGroup {
		case "coreum-modules", "coreum-upgrade":

			fullArgs = append(fullArgs,
				"-log-format", config.LogFormat,
				"-funding-mnemonic", coredNode.Config().FaucetMnemonic,
				"-skip-unsafe=false",
			)

			for _, m := range appSet {
				coredApp, ok := m.(cored.Cored)
				if ok && coredApp.Config().IsValidator && strings.HasPrefix(coredApp.Name(), "cored-") {
					fullArgs = append(fullArgs, "-staker-mnemonic", coredApp.Config().StakerMnemonic)
				}
			}
		case "faucet":
			faucetApp := appSet.FindRunningApp(faucet.AppType, "faucet")
			if faucetApp == nil {
				return errors.New("no running faucet app found")
			}
			faucetNode := faucetApp.(faucet.Faucet)
			fullArgs = append(fullArgs,
				"-transfer-amount", "1000000",
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
