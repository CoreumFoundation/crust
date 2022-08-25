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

	node := mode[0].(cored.Cored)
	waitCtx, waitCancel := context.WithTimeout(ctx, 20*time.Second)
	defer waitCancel()
	if err := infra.WaitUntilHealthy(waitCtx, node); err != nil {
		return err
	}

	log := logger.Get(ctx)
	args := []string{
		"-cored-address", infra.JoinNetAddr("", node.Info().HostFromHost, node.Ports().RPC),
		"-priv-key", base64.RawURLEncoding.EncodeToString(fundingPrivKey),
		"-log-format", config.LogFormat,

		// The tests themselves are not computationally expensive, most of the time they spend waiting for
		// transactions to be included in blocks, so it should be safe to run more tests in parallel than we have CPus
		// available.
		"-test.parallel", strconv.Itoa(2 * runtime.NumCPU()),
	}
	if config.TestFilter != "" {
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
