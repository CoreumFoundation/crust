package _go

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

type goBuildConfig struct {
	// PackagePath is the path to package to build
	PackagePath string

	// BinOutputPath is the path for compiled binary file
	BinOutputPath string

	// DockerTags is the list of additional tags to build only in docker
	DockerTags []string

	// DockerStatic triggers static compilation inside docker
	DockerStatic bool

	// CGOEnabled builds cgo binary
	CGOEnabled bool

	// BuildForDocker cuases a docker-specific binary to be built inside docker container
	BuildForDocker bool

	// BuildForLocal causes a local-specific binary to be built locally
	BuildForLocal bool
}

func ensureGo(ctx context.Context) error {
	return ensureLocal(ctx, "go")
}

func ensureGolangCI(ctx context.Context) error {
	return ensureLocal(ctx, "golangci")
}

func ensureLibWASMVMMuslC(ctx context.Context) error {
	return ensureDocker(ctx, "libwasmvm_muslc")
}

func goBuildLocally(ctx context.Context, config goBuildConfig) error {
	logger.Get(ctx).Info("Building go package locally", zap.String("package", config.PackagePath), zap.String("binary", config.BinOutputPath))

	args, envs := goBuildArgsAndEnvs(config, filepath.Join(cacheDir(), "lib"), false)
	args = append(args, "-o", must.String(filepath.Abs(config.BinOutputPath)), ".")
	envs = append(envs, os.Environ()...)

	cmd := exec.Command(toolBin("go"), args...)
	cmd.Dir = config.PackagePath
	cmd.Env = envs

	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "building go package '%s' failed", config.PackagePath)
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
	}{
		GOVersion:     tools["go"].Version,
		AlpineVersion: goAlpineVersion,
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

	out := filepath.Join("bin/.cache/docker", filepath.Base(config.BinOutputPath))

	logger.Get(ctx).Info("Building go package in docker", zap.String("package", config.PackagePath), zap.String("binary", out))

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
	workDir := filepath.Clean(filepath.Join("/src", "crust", config.PackagePath))
	nameSuffix := make([]byte, 4)
	must.Any(rand.Read(nameSuffix))

	args, envs := goBuildArgsAndEnvs(config, "/crust-cache/lib", true)
	runArgs := []string{
		"run", "--rm",
		"-v", srcDir + ":/src",
		"-v", cacheDir() + ":/crust-cache",
		"-v", goPath + ":/go",
		"-v", goCache + ":/go-cache",
		"--env", "GOPATH=/go",
		"--env", "GOCACHE=/go-cache",
		"--workdir", workDir,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--name", "crust-build-" + filepath.Base(out) + "-" + hex.EncodeToString(nameSuffix),
	}
	for _, env := range envs {
		runArgs = append(runArgs, "--env", env)
	}
	runArgs = append(runArgs, image)
	runArgs = append(runArgs, args...)
	runArgs = append(runArgs, "-o", "/src/crust/"+out, ".")
	if err := libexec.Exec(ctx, exec.Command("docker", runArgs...)); err != nil {
		return errors.Wrapf(err, "building package '%s' failed", config.PackagePath)
	}
	return nil
}

func goBuild(ctx context.Context, config goBuildConfig) error {
	if config.BuildForDocker {
		if err := goBuildInDocker(ctx, config); err != nil {
			return err
		}
	}
	if config.BuildForLocal {
		if err := goBuildLocally(ctx, config); err != nil {
			return err
		}
	}
	return nil
}

func goBuildArgsAndEnvs(config goBuildConfig, libDir string, docker bool) (args []string, envs []string) {
	ldFlags := []string{"-w", "-s"}
	if docker && config.DockerStatic {
		ldFlags = append(ldFlags, "-extldflags=-static")
	}
	args = []string{
		"build",
		"-trimpath",
		"-ldflags=" + strings.Join(ldFlags, " "),
	}
	if docker && len(config.DockerTags) > 0 {
		args = append(args, "-tags="+strings.Join(config.DockerTags, ","))
	}

	envs = []string{
		"LIBRARY_PATH=" + libDir,
	}
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
