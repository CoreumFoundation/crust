package testing

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

// Run deploys testing environment and runs tests there
func Run(ctx context.Context, target infra.Target, mode infra.Mode, config infra.Config) error {
	testDir := filepath.Join(config.BinDir, ".cache", "integration-tests")
	files, err := os.ReadDir(testDir)
	if err != nil {
		return errors.WithStack(err)
	}

	fundingPrivKey, err := cored.PrivateKeyFromMnemonic(FundingMnemonic)
	if err != nil {
		return err
	}

	if err := target.Deploy(ctx, mode); err != nil {
		return err
	}

	waitForApps := make([]infra.HealthCheckCapable, 0, len(mode))
	for _, app := range mode {
		withHealthCheck, ok := app.(infra.HealthCheckCapable)
		if !ok {
			withHealthCheck = infra.IsRunning(app)
		}
		waitForApps = append(waitForApps, withHealthCheck)
	}

	log := logger.Get(ctx)
	log.Info("Waiting until all applications start...")

	waitCtx, waitCancel := context.WithTimeout(ctx, 20*time.Second)
	defer waitCancel()
	if err := infra.WaitUntilHealthy(waitCtx, waitForApps...); err != nil {
		return err
	}

	log.Info("All the applications are ready")

	coredApp := mode.FindApp(cored.AppType)
	if coredApp == nil {
		return errors.New("no running cored app found")
	}

	coredNode := coredApp.(cored.Cored)

	args := []string{
		"-cored-address", infra.JoinNetAddr("", coredNode.Info().HostFromHost, coredNode.Ports().RPC),
		"-priv-key", base64.RawURLEncoding.EncodeToString(fundingPrivKey),
		"-log-format", config.LogFormat,

		// The tests themselves are not computationally expensive, most of the time they spend waiting for
		// transactions to be included in blocks, so it should be safe to run more tests in parallel than we have CPus
		// available.
		"-test.parallel", strconv.Itoa(2 * runtime.NumCPU()),
	}
	if config.TestFilter != "" {
		log.Info("Running only tests matching filter", zap.String("filter", config.TestFilter))
		args = append(args, "-filter", config.TestFilter)
	}
	if config.VerboseLogging {
		args = append(args, "-test.v")
	}

	var failed bool
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		binPath := filepath.Join(testDir, f.Name())
		log := log.With(zap.String("binary", binPath))
		log.Info("Running tests")

		if err := libexec.Exec(ctx, exec.Command(binPath, args...)); err != nil {
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
