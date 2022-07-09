package build

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const goAlpineVersion = "3.16"

var repositories = []string{"../crust", "../coreum"}

type wasmMuslcSource struct {
	URL  string
	Hash string
}

// See https://github.com/CosmWasm/wasmvm/releases
var wasmMuslc = map[string]wasmMuslcSource{
	"amd64": {
		URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.x86_64.a",
		Hash: "f6282df732a13dec836cda1f399dd874b1e3163504dbd9607c6af915b2740479",
	},
	"arm64": {
		URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.aarch64.a",
		Hash: "7d2239e9f25e96d0d4daba982ce92367aacf0cbd95d2facb8442268f2b1cc1fc",
	},
}

type goBuildConfig struct {
	// Package is the path to package to build
	Package string

	// BinPath is the path for compiled binary file
	BinPath string

	// Tags is the list of additional tags to build
	Tags []string

	// CGOEnabled builds cgo binary
	CGOEnabled bool

	// BuildForDocker cuases to docker-specific binary to be built inside docker container
	BuildForDocker bool

	// BuildForHost causes tp host-specific binary to be built on host
	BuildForHost bool
}

func ensureGo(ctx context.Context) error {
	return ensure(ctx, "go")
}

func ensureGolangCI(ctx context.Context) error {
	return ensure(ctx, "golangci")
}

func goBuildOnHost(ctx context.Context, config goBuildConfig) error {
	logger.Get(ctx).Info("Building go package on host", zap.String("package", config.Package), zap.String("binary", config.BinPath))

	args, envs := goBuildArgsAndEnvs(config)
	args = append(args, "-o", must.String(filepath.Abs(config.BinPath)), ".")

	cmd := exec.Command(toolBin("go"), args...)
	cmd.Dir = config.Package

	cmd.Env = append(envs, os.Environ()...)
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "building go package '%s' failed", config.Package)
	}
	return nil
}

//go:embed docker/Dockerfile.tmpl.cgo
var cgoDockerfileTemplate string

var cgoDockerfileTemplateParsed = template.Must(template.New("Dockerfile").Parse(cgoDockerfileTemplate))

func ensureBuildDockerImage(ctx context.Context) (string, error) {
	dockerfileBuf := &bytes.Buffer{}
	err := cgoDockerfileTemplateParsed.Execute(dockerfileBuf, struct {
		GOVersion     string
		AlpineVersion string
		WASMMuslc     wasmMuslcSource
	}{
		GOVersion:     tools["go"].Version,
		AlpineVersion: goAlpineVersion,
		WASMMuslc:     wasmMuslc[runtime.GOARCH],
	})
	if err != nil {
		return "", errors.Wrap(err, "executing Dockerfile template failed")
	}

	dockerfileChecksum := sha256.Sum256(dockerfileBuf.Bytes())
	image := "crust-cgo-build:" + hex.EncodeToString(dockerfileChecksum[:4])

	imageBuf := &bytes.Buffer{}
	imageCmd := exec.Command("docker", "images", "-q", image)
	imageCmd.Stdout = imageBuf
	if err := libexec.Exec(ctx, imageCmd); err != nil {
		return "", errors.Wrapf(err, "failed to list image '%s'", image)
	}
	if imageBuf.Len() > 0 {
		return image, nil
	}

	buildCmd := exec.Command("docker", "build",
		"--tag", image, "-f", "-", "build/docker")
	buildCmd.Stdin = dockerfileBuf

	if err := libexec.Exec(ctx, buildCmd); err != nil {
		return "", errors.Wrapf(err, "failed to build image '%s'", image)
	}
	return image, nil
}

func goBuildInDocker(ctx context.Context, config goBuildConfig) error {
	// FIXME (wojciech): use docker API instead of docker executable

	out := filepath.Join("bin/.cache/docker", filepath.Base(config.BinPath))

	logger.Get(ctx).Info("Building go package in docker", zap.String("package", config.Package), zap.String("binary", out))

	_, err := exec.LookPath("docker")
	if err != nil {
		err = errors.Wrap(err, "docker command is not available in PATH")
		return err
	}

	image, err := ensureBuildDockerImage(ctx)
	if err != nil {
		return err
	}

	srcDir := must.String(filepath.Abs(".."))
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(must.String(os.UserHomeDir()), "go")
	}
	if err := os.MkdirAll(goPath, 0o700); err != nil {
		return errors.WithStack(err)
	}
	goCache := cacheDir() + "/docker/go-build"
	if err := os.MkdirAll(goCache, 0o700); err != nil {
		return errors.WithStack(err)
	}
	workDir := filepath.Clean(filepath.Join("/src", "crust", config.Package))
	nameSuffix := make([]byte, 4)
	must.Any(rand.Read(nameSuffix))

	args, envs := goBuildArgsAndEnvs(config)
	runArgs := append([]string{
		"run", "--rm",
		"-v", srcDir + ":/src",
		"-v", goPath + ":/go",
		"-v", goCache + ":/go-cache",
		"--env", "GOPATH=/go",
		"--env", "GOCACHE=/go-cache",
		"--workdir", workDir,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--name", "crust-build-" + filepath.Base(out) + "-" + hex.EncodeToString(nameSuffix),
	})
	for _, env := range envs {
		runArgs = append(runArgs, "--env", env)
	}
	runArgs = append(runArgs, image)
	runArgs = append(runArgs, args...)
	runArgs = append(runArgs, "-o", "/src/crust/"+out, ".")
	if err := libexec.Exec(ctx, exec.Command("docker", runArgs...)); err != nil {
		return errors.Wrapf(err, "building package '%s' failed", config.Package)
	}
	return nil
}

func goBuild(ctx context.Context, config goBuildConfig) error {
	if config.BuildForDocker {
		if err := goBuildInDocker(ctx, config); err != nil {
			return err
		}
	}
	if config.BuildForHost {
		if err := goBuildOnHost(ctx, config); err != nil {
			return err
		}
	}
	return nil
}

func goBuildArgsAndEnvs(config goBuildConfig) (args []string, envs []string) {
	args = []string{
		"build",
		"-trimpath",
		"-ldflags=-w -s -extldflags=-static",
	}
	if len(config.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(config.Tags, ","))
	}

	envs = []string{}
	if config.CGOEnabled {
		envs = append(envs, "CGO_ENABLED=1")
	}
	return args, envs
}

// goLint runs golangci linter, runs go mod tidy and checks that git status is clean
func goLint(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo, ensureGolangCI, ensureAllRepos)
	log := logger.Get(ctx)
	config := must.String(filepath.Abs("build/.golangci.yaml"))
	err := onModule(func(path string) error {
		log.Info("Running linter", zap.String("path", path))
		cmd := exec.Command(toolBin("golangci-lint"), "run", "--config", config)
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "linter errors found in module '%s'", path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// goTest runs go test
func goTest(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo, ensureAllRepos)
	log := logger.Get(ctx)
	return onModule(func(path string) error {
		log.Info("Running go tests", zap.String("path", path))
		cmd := exec.Command(toolBin("go"), "test", "-count=1", "-shuffle=on", "-race", "./...")
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "unit tests failed in module '%s'", path)
		}
		return nil
	})
}

func goModTidy(ctx context.Context, deps build.DepsFunc) error {
	deps(ensureGo, ensureAllRepos)
	log := logger.Get(ctx)
	return onModule(func(path string) error {
		log.Info("Running go mod tidy", zap.String("path", path))
		cmd := exec.Command(toolBin("go"), "mod", "tidy")
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "'go mod tidy' failed in module '%s'", path)
		}
		return nil
	})
}

func onModule(fn func(path string) error) error {
	for _, repoPath := range repositories {
		err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != "go.mod" {
				return nil
			}
			return fn(filepath.Dir(path))
		})
		if err != nil {
			return err
		}
	}
	return nil
}
