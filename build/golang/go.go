package golang

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

	"github.com/CoreumFoundation/coreum/build/git"
	"github.com/CoreumFoundation/coreum/build/tools"
)

const goAlpineVersion = "3.16"

// BuildConfig is the configuration for `go build`
type BuildConfig struct {
	// PackagePath is the path to package to build
	PackagePath string

	// BinOutputPath is the path for compiled binary file
	BinOutputPath string

	// Tags is the list of additional tags to build
	Tags []string

	// LinkStatically triggers static compilation
	LinkStatically bool

	// CGOEnabled builds cgo binary
	CGOEnabled bool
}

// EnsureGo ensures that go is available
func EnsureGo(ctx context.Context) error {
	return tools.EnsureLocal(ctx, tools.Go)
}

// EnsureGolangCI ensures that go linter is available
func EnsureGolangCI(ctx context.Context) error {
	return tools.EnsureLocal(ctx, tools.GolangCI)
}

// EnsureLibWASMVMMuslC ensures that libwasmvm_muslc is installed
func EnsureLibWASMVMMuslC(ctx context.Context) error {
	return tools.EnsureDocker(ctx, tools.LibWASMMuslC)
}

// BuildLocally builds binary locally
func BuildLocally(ctx context.Context, config BuildConfig) error {
	logger.Get(ctx).Info("Building go package locally", zap.String("package", config.PackagePath),
		zap.String("binary", config.BinOutputPath))

	args, envs := buildArgsAndEnvs(config, filepath.Join(tools.CacheDir(), "lib"))
	args = append(args, "-o", must.String(filepath.Abs(config.BinOutputPath)), ".")
	envs = append(envs, os.Environ()...)

	cmd := exec.Command(tools.Path("go"), args...)
	cmd.Dir = config.PackagePath
	cmd.Env = envs

	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "building go package '%s' failed", config.PackagePath)
	}
	return nil
}

// BuildInDocker builds binary inside docker container
func BuildInDocker(ctx context.Context, config BuildConfig) error {
	// FIXME (wojciech): use docker API instead of docker executable

	logger.Get(ctx).Info("Building go package in docker", zap.String("package", config.PackagePath),
		zap.String("binary", config.BinOutputPath))

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
	cacheDir := filepath.Join(tools.CacheDir(), tools.DockerPlatform.String())
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return errors.WithStack(err)
	}
	workDir := filepath.Clean(filepath.Join("/src", "crust", config.PackagePath))
	nameSuffix := make([]byte, 4)
	must.Any(rand.Read(nameSuffix))

	args, envs := buildArgsAndEnvs(config, "/crust-cache/lib")
	runArgs := []string{
		"run", "--rm",
		"-v", srcDir + ":/src",
		"-v", goPath + ":/go",
		"-v", cacheDir + ":/crust-cache",
		"--env", "GOPATH=/go",
		"--env", "GOCACHE=/crust-cache/go-build",
		"--workdir", workDir,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--name", "crust-build-" + filepath.Base(config.BinOutputPath) + "-" + hex.EncodeToString(nameSuffix),
	}
	for _, env := range envs {
		runArgs = append(runArgs, "--env", env)
	}
	runArgs = append(runArgs, image)
	runArgs = append(runArgs, args...)
	runArgs = append(runArgs, "-o", "/src/crust/"+config.BinOutputPath, ".")
	if err := libexec.Exec(ctx, exec.Command("docker", runArgs...)); err != nil {
		return errors.Wrapf(err, "building package '%s' failed", config.PackagePath)
	}
	return nil
}

//go:embed Dockerfile.tmpl.cgo
var cgoDockerfileTemplate string

var cgoDockerfileTemplateParsed = template.Must(template.New("Dockerfile").Parse(cgoDockerfileTemplate))

func ensureBuildDockerImage(ctx context.Context) (string, error) {
	dockerfileBuf := &bytes.Buffer{}
	err := cgoDockerfileTemplateParsed.Execute(dockerfileBuf, struct {
		GOVersion     string
		AlpineVersion string
	}{
		GOVersion:     tools.ByName(tools.Go).Version,
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

func buildArgsAndEnvs(config BuildConfig, libDir string) (args []string, envs []string) {
	ldFlags := []string{"-w", "-s"}
	if config.LinkStatically {
		ldFlags = append(ldFlags, "-extldflags=-static")
	}
	args = []string{
		"build",
		"-trimpath",
		"-ldflags=" + strings.Join(ldFlags, " "),
	}
	if len(config.Tags) > 0 {
		args = append(args, "-tags="+strings.Join(config.Tags, ","))
	}

	envs = []string{
		"LIBRARY_PATH=" + libDir,
	}
	if config.CGOEnabled {
		envs = append(envs, "CGO_ENABLED=1")
	}
	return args, envs
}

// Test runs go test
func Test(ctx context.Context, deps build.DepsFunc) error {
	deps(EnsureGo, git.EnsureAllRepos)
	log := logger.Get(ctx)
	return onModule(func(path string) error {
		log.Info("Running go tests", zap.String("path", path))
		cmd := exec.Command(tools.Path("go"), "test", "-count=1", "-shuffle=on", "-race", "./...")
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "unit tests failed in module '%s'", path)
		}
		return nil
	})
}

// Tidy runs go mod tidy
func Tidy(ctx context.Context, deps build.DepsFunc) error {
	deps(EnsureGo, git.EnsureAllRepos)
	log := logger.Get(ctx)
	return onModule(func(path string) error {
		log.Info("Running go mod tidy", zap.String("path", path))
		cmd := exec.Command(tools.Path("go"), "mod", "tidy")
		cmd.Dir = path
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "'go mod tidy' failed in module '%s'", path)
		}
		return nil
	})
}

func onModule(fn func(path string) error) error {
	for _, repoPath := range git.Repositories {
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
