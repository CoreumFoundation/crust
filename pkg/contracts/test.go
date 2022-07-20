package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// TestConfig provides params for the testing stage.
type TestConfig struct {
	// NeedCoverageReport enables running additional code coverage collector tool 'tarpaulin',
	// which is currently available for linux/amd64 only. Enabling this option on other OSes will trigger a warning.
	NeedCoverageReport bool

	// HasIntegrationTests triggers an additional stage where integration tests are run. This will rebuild
	// the target in release mode.
	HasIntegrationTests bool
}

// Test implements logic for "contracts test" CLI command
func Test(ctx context.Context, workspaceDir string, config TestConfig) error {
	log := logger.Get(ctx)

	if config.NeedCoverageReport {
		if runtime.GOOS != "linux" {
			log.Warn("need code coverage flag provided, but current OS is not supported (expected linux)", zap.String("os", runtime.GOOS))
			config.NeedCoverageReport = false
		} else if runtime.GOARCH != "amd64" {
			log.Warn("need code coverage flag provided, but current ARCH is not supported (expected amd64)", zap.String("arch", runtime.GOARCH))
			config.NeedCoverageReport = false
		}
	}

	if err := ensureUnitTestTools(ctx); err != nil {
		err = errors.Wrap(err, "not all testing dependencies are installed")
		return err
	}

	crateName, err := readCrateMetadata(ctx, workspaceDir)
	if err != nil {
		err = errors.Wrap(err, "problem with ensuring the target crate workspace")
		return err
	}
	crateLog := log.With(zap.String("name", crateName), zap.String("dir", workspaceDir))

	crateLog.Info("Running cargo check on the workspace")
	if err := runCargoCheckTests(ctx, workspaceDir); err != nil {
		err = errors.Wrap(err, "problem with checking the target crate workspace and tests")
		return err
	}

	if config.NeedCoverageReport {
		crateLog.Info("Running unit-tests suite with coverage enabled")
		if err := runTestsWithCoverage(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with tests")
			return err
		}
	} else {
		crateLog.Info("Running unit-tests suite")
		if err := runTests(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with tests")
			return err
		}
	}

	if config.HasIntegrationTests {
		crateLog.Info("Running build of the release WASM artefact for integration test")
		if err := runReleaseBuild(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with cargo build")
			return err
		}

		crateLog.Info("Running integration tests suite")
		if err := runIntegrationTests(ctx, workspaceDir); err != nil {
			err = errors.Wrap(err, "problem with tests")
			return err
		}
	}

	return nil
}

// runCargoCheckTests checks workspace tests and all dependencies for errors
func runCargoCheckTests(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"check", "--workspace", "--quiet", "--tests", "--lib",
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo check run")
		return err
	}

	return nil
}

// runTests runs the target WASM contract crate tests
func runTests(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"test", "--lib",
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RUST_BACKTRACE=1")

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo test run")
		return err
	}

	return nil
}

// runIntegrationTests runs the target WASM contract integration tests.
func runIntegrationTests(ctx context.Context, workspaceDir string) error {
	cmdArgs := []string{
		"test", "--test", "integration",
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RUST_BACKTRACE=1")

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo test run")
		return err
	}

	return nil
}

// runTestsWithCoverage runs the contact test-suite with code coverage enabled,
// uses cargo-tarpaulin entrypoint, which will be ensured to present during first run.
func runTestsWithCoverage(ctx context.Context, workspaceDir string) error {
	if err := ensureCrateAndVersion(ctx, "cargo-tarpaulin", cargoTarpaulinVersion); err != nil {
		err = errors.Wrap(err, "failed to ensure cargo-tarpaulin dependency")
	}
	cmdArgs := []string{
		"tarpaulin", "--out", "html", "--output-dir", "./coverage",
	}
	cmd := exec.Command("cargo", cmdArgs...)
	cmd.Dir = workspaceDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RUST_BACKTRACE=1")

	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "errors during cargo test run")
		return err
	}

	log := logger.Get(ctx)

	coveragePrefixAbs, err := filepath.Abs(filepath.Join(workspaceDir, "coverage"))
	if err != nil {
		log.With(zap.Error(err)).Warn("Ran tests with coverage enabled, but no coverage report could be located")
	} else {
		coverageReportPath := filepath.Join(coveragePrefixAbs, "tarpaulin-report.html")
		log.Info("Code coverage report written", zap.String("path", coverageReportPath))
	}

	return nil
}

func readCrateMetadata(ctx context.Context, workspaceDir string) (crateName string, err error) {
	cmd := exec.Command("cargo", "metadata", "--format-version", "1")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Dir = workspaceDir
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return "", err
	}

	var meta crateMetadata
	if err := json.Unmarshal(out.Bytes(), &meta); err != nil {
		err = errors.Wrap(err, "failed to unmarshal crate metadata")
		return "", err
	}

	if len(meta.WorkspaceMembers) == 0 {
		err = errors.Errorf("workspace at %s doesn't have any members", workspaceDir)
		return "", err
	}

	rootCrateSpec := strings.Split(meta.WorkspaceMembers[0], " ")
	crateName = rootCrateSpec[0]

	return crateName, nil
}

type crateMetadata struct {
	WorkspaceMembers []string `json:"workspace_members"`
}
