package contracts

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

const (
	minimalRustVersion  = "1.62.0"
	minimalCargoVersion = "1.62.0"

	cargoGenerateVersion  = "0.15.2"
	cargoRunScriptVersion = "0.1.0"
	cargoTarpaulinVersion = "0.20.1"

	optimizedBuildDockerImage = "cosmwasm/rust-optimizer:0.12.6"

	wasmTarget = "wasm32-unknown-unknown"
)

var (
	// versionRx is the most basic semVer regexp,
	// if you want to fall into a turing tarpit, google the full one.
	versionRx = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+)`)
)

var (
	toolIsNotInstalled  = errors.New("tool is not installed")
	crateIsNotInstalled = errors.New("crate is not installed")
)

func ensureInitTools(ctx context.Context) error {
	if err := ensureRustToolchain(ctx); err != nil {
		err = errors.Wrap(err, "problem with checking the Rust toolchain")
		return err
	}

	if err := ensureCrateAndVersion(ctx, "cargo-generate", cargoGenerateVersion,
		"--features", "vendored-openssl",
	); err != nil {
		err = errors.Wrap(err, "problem with cargo-generate crate")
		return err
	}

	if err := ensureCrateAndVersion(ctx, "cargo-run-script", cargoRunScriptVersion); err != nil {
		err = errors.Wrap(err, "problem with cargo-run-script crate")
		return err
	}

	return nil
}

func ensureUnitTestTools(ctx context.Context) error {
	if err := ensureRustToolchain(ctx); err != nil {
		err = errors.Wrap(err, "problem with checking the Rust toolchain")
		return err
	}

	if err := ensureRustTarget(ctx, wasmTarget); err != nil {
		err = errors.Wrapf(err, "problem with ensuring the WASM target (%s) for rustc", wasmTarget)
		return err
	}

	return nil
}

func ensureBuildTools(ctx context.Context, needOptimized bool) error {
	if needOptimized {
		// optimized predictable builds require only Docker, an image will do the rest
		_, err := exec.LookPath("docker")
		if err != nil {
			err = errors.Wrap(err, "docker command is not available in PATH")
			return err
		}

		return nil
	}

	if err := ensureRustToolchain(ctx); err != nil {
		err = errors.Wrap(err, "problem with checking the Rust toolchain")
		return err
	}

	if err := ensureRustTarget(ctx, wasmTarget); err != nil {
		err = errors.Wrapf(err, "problem with ensuring the WASM target (%s) for rustc", wasmTarget)
		return err
	}

	return nil
}

func ensureRustToolchain(ctx context.Context) error {
	log := logger.Get(ctx)

	if _, err := exec.LookPath("rustup"); err != nil {
		log.Warn("You may want to install rustup into your system:\ncurl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh")
		err = errors.Wrap(err, "rustup command is not available in PATH")
		return err
	}

	if _, err := exec.LookPath("cargo"); err != nil {
		err = errors.Wrap(err, "cargo command is not available in PATH")
		return err
	} else if cargoVersion, err := readCargoVersion(ctx); err != nil {
		err = errors.Wrap(err, "failed to read cargo version")
		return err
	} else if len(cargoVersion) == 0 {
		return errors.New("failed to read cargo version: empty match")
	} else if isLessVersion(cargoVersion, minimalCargoVersion) {
		log.Warn("You may want to update the cargo version using `rustup update stable`")
		err = errors.Errorf("found cargo version %s but minimal is %s", cargoVersion, minimalCargoVersion)
		return err
	}

	if _, err := exec.LookPath("rustc"); err != nil {
		err = errors.Wrap(err, "rustc command is not available in PATH")
		return err
	} else if rustVersion, err := readRustVersion(ctx); err != nil {
		err = errors.Wrap(err, "failed to read rustc version")
		return err
	} else if len(rustVersion) == 0 {
		return errors.New("failed to read rustc version: empty match")
	} else if isLessVersion(rustVersion, minimalRustVersion) {
		log.Warn("You may want to update the rustc version using `rustup update stable`")
		err = errors.Errorf("found rustc version %s but minimal is %s", rustVersion, minimalRustVersion)
		return err
	}

	return nil
}

func readRustVersion(ctx context.Context) (string, error) {
	cmd := exec.Command("rustc", "--version")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return "", err
	}

	semVerMatches := versionRx.FindStringSubmatch(out.String())
	if len(semVerMatches) < 2 {
		return "", errors.WithStack(toolIsNotInstalled)
	}

	return semVerMatches[1], nil
}

func readCargoVersion(ctx context.Context) (string, error) {
	cmd := exec.Command("cargo", "--version")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return "", err
	}

	semVerMatches := versionRx.FindStringSubmatch(out.String())
	if len(semVerMatches) < 2 {
		return "", errors.WithStack(toolIsNotInstalled)
	}

	return semVerMatches[1], nil
}

func readCrateVersion(ctx context.Context, crateName string) (string, error) {
	cmd := exec.Command("cargo", "install", "--list")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return "", err
	}

	crateVerRx, err := regexp.Compile(fmt.Sprintf("%s v([0-9]+\\.[0-9]+\\.[0-9]+)", crateName))
	if err != nil {
		err = errors.Wrap(err, "failed to compile crate semver regexp")
		return "", err
	}

	semVerMatches := crateVerRx.FindStringSubmatch(out.String())
	if len(semVerMatches) < 2 {
		return "", errors.WithStack(crateIsNotInstalled)
	}

	return semVerMatches[1], nil
}

func ensureCrateAndVersion(
	ctx context.Context,
	crateName, requiredVersion string,
	installArgs ...string,
) error {
	log := logger.Get(ctx)
	if crateVer, err := readCrateVersion(ctx, crateName); errors.Is(err, crateIsNotInstalled) {
		log.Info("Crate is missing and requires installation",
			zap.String("crate", crateName),
			zap.String("version", requiredVersion),
		)

		cmdArgs := append([]string{
			"install", crateName,
			"--version", requiredVersion,
		}, installArgs...)

		cmd := exec.Command("cargo", cmdArgs...)
		if err := libexec.Exec(ctx, cmd); err != nil {
			err = errors.Wrap(err, "cmd exec failed")
			return err
		}
	} else if err != nil {
		err = errors.Wrap(err, "failed to check crate version")
		return err
	} else if isLessVersion(crateVer, requiredVersion) {
		log.Info("Crate is outdated and requires update",
			zap.String("crate", crateName),
			zap.String("oldVersion", crateVer),
			zap.String("newVersion", requiredVersion),
		)

		cmdArgs := append([]string{
			"install", crateName,
			"--version", requiredVersion,
		}, installArgs...)

		cmd := exec.Command("cargo", cmdArgs...)
		if err := libexec.Exec(ctx, cmd); err != nil {
			err = errors.Wrap(err, "cmd exec failed")
			return err
		}
	}

	return nil
}

func ensureRustTarget(ctx context.Context, targetName string) error {
	cmd := exec.Command("rustup", "target", "list", "--installed")
	out := new(bytes.Buffer)
	cmd.Stdout = out
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return err
	}

	if strings.Contains(out.String(), targetName) {
		// target already installed
		return nil
	}

	log := logger.Get(ctx)
	log.Info("Installing new rustc target", zap.String("name", targetName))

	cmd = exec.Command("rustup", "target", "add", targetName)
	if err := libexec.Exec(ctx, cmd); err != nil {
		err = errors.Wrap(err, "cmd exec failed")
		return err
	}

	return nil
}

func isLessVersion(a, b string) bool {
	return semver.Compare(ensureV(a), ensureV(b)) < 0
}

func ensureV(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}

	return "v" + version
}
