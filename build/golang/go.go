package golang

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

const (
	repoPath        = "."
	goAlpineVersion = "3.18"
)

// BinaryBuildConfig is the configuration for `go build`.
type BinaryBuildConfig struct {
	// TargetPlatform is the platform to build the binary for
	TargetPlatform tools.TargetPlatform

	// ModuleRef is the reference used to get module path
	ModuleRef any

	// PackagePath is the path to package to build relative to the ModulePath
	PackagePath string

	// BinOutputPath is the path for compiled binary file
	BinOutputPath string

	// CGOEnabled builds cgo binary
	CGOEnabled bool

	// Flags is a slice of additional flags to pass to `go build`. E.g -cover, -compiler, -ldflags=... etc.
	Flags []string

	// Envs is a slice of additional environment variables to pass to `go build`.
	Envs []string
}

// TestConfig is the configuration for `go test -c`.
type TestConfig struct {
	// PackagePath is the path to package to build
	PackagePath string

	// Flags is a slice of additional flags to pass to `go test -c`. E.g -cover, -compiler, -ldflags=... etc.
	Flags []string
}

// env gets environment variables set in the system excluding Go env vars that
// causes conflict with the build tools used by crust. For example having
// the GOROOT and GOPATH that point to another version of Go, build binaries
// with incompatible Go version that fails to run properly.
func env() []string {
	osEnv := os.Environ()
	envVars := make([]string, 0, len(osEnv))
	for _, envVar := range osEnv {
		e := strings.ToUpper(envVar)
		if !strings.Contains(e, "GOROOT=") && !strings.Contains(e, "GOPATH=") {
			envVars = append(envVars, envVar)
		}
	}
	return envVars
}

// EnsureGo ensures that go is available.
func EnsureGo(ctx context.Context, deps types.DepsFunc) error {
	return tools.Ensure(ctx, tools.Go, tools.TargetPlatformLocal)
}

// EnsureGolangCI ensures that go linter is available.
func EnsureGolangCI(ctx context.Context, deps types.DepsFunc) error {
	deps(storeLintConfig)
	return tools.Ensure(ctx, tools.GolangCI, tools.TargetPlatformLocal)
}

// Build builds go binary.
func Build(ctx context.Context, deps types.DepsFunc, config BinaryBuildConfig) error {
	deps(EnsureGo)

	if config.TargetPlatform.BuildInDocker {
		return buildInDocker(ctx, config)
	}
	return buildLocally(ctx, config)
}

func buildLocally(ctx context.Context, config BinaryBuildConfig) error {
	if config.TargetPlatform != tools.TargetPlatformLocal {
		return errors.Errorf("building requested for platform %s while only %s is supported",
			config.TargetPlatform, tools.TargetPlatformLocal)
	}

	libDir := filepath.Join(tools.CacheDir(), "lib")
	if err := os.MkdirAll(libDir, 0o700); err != nil {
		return errors.WithStack(err)
	}
	args, envs, err := buildArgsAndEnvs(ctx, config, libDir)
	if err != nil {
		return err
	}
	args = append(args, "-o", must.String(filepath.Abs(config.BinOutputPath)), ".")
	envs = append(envs, env()...)

	modulePath, err := findModulePath(ctx, config.ModuleRef)
	if err != nil {
		return err
	}

	cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), args...)
	cmd.Dir = filepath.Join(modulePath, config.PackagePath)
	cmd.Env = envs

	logger.Get(ctx).Info(
		"Building go package locally",
		zap.String("package", config.PackagePath),
		zap.String("command", cmd.String()),
	)
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "building go package '%s' failed", config.PackagePath)
	}
	return nil
}

func buildInDocker(ctx context.Context, config BinaryBuildConfig) error {
	// FIXME (wojciech): use docker API instead of docker executable

	if _, err := exec.LookPath("docker"); err != nil {
		return errors.Wrap(err, "docker command is not available in PATH")
	}

	modulePath, err := findModulePath(ctx, config.ModuleRef)
	if err != nil {
		return err
	}

	image, err := ensureBuildDockerImage(ctx)
	if err != nil {
		return err
	}

	srcDir := must.String(filepath.Abs(".."))
	dockerRepoDir := filepath.Join("/src", filepath.Base(modulePath)+"-tmp")

	goPath := GoPath()
	if err := os.MkdirAll(goPath, 0o700); err != nil {
		return errors.WithStack(err)
	}
	cacheDir := tools.PlatformRootPath(config.TargetPlatform)
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return errors.WithStack(err)
	}
	workDir := filepath.Clean(filepath.Join(dockerRepoDir, config.PackagePath))
	nameSuffix := make([]byte, 4)
	must.Any(rand.Read(nameSuffix))

	args, envs, err := buildArgsAndEnvs(ctx, config, filepath.Join("/crust-cache", tools.Version(), "lib"))
	if err != nil {
		return err
	}
	runArgs := []string{
		"run", "--rm",
		"--label", docker.LabelKey + "=" + docker.LabelValue,
		"-v", srcDir + ":/src",
		"-v", modulePath + ":" + dockerRepoDir,
		"-v", goPath + ":/go",
		"-v", cacheDir + ":/crust-cache",
		"--env", "GOPATH=/go",
		"--env", "GOCACHE=/crust-cache/go-build",
		"--workdir", workDir,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--name", "crust-build-" + filepath.Base(config.PackagePath) + "-" + hex.EncodeToString(nameSuffix),
	}
	if config.CGOEnabled &&
		tools.TargetPlatformLocal == tools.TargetPlatformLinuxAMD64 &&
		config.TargetPlatform == tools.TargetPlatformLinuxARM64InDocker {
		crossCompilerPath := filepath.Dir(
			filepath.Dir(tools.Path("bin/aarch64-linux-musl-gcc", tools.TargetPlatformLinuxAMD64InDocker)),
		)
		libWasmVMPath := tools.Path("lib/libwasmvm_muslc.a", tools.TargetPlatformLinuxARM64InDocker)
		runArgs = append(runArgs,
			"-v", crossCompilerPath+":/aarch64-linux-musl-cross",
			"-v", libWasmVMPath+":/aarch64-linux-musl-cross/aarch64-linux-musl/lib/libwasmvm_muslc.a",
		)
	}
	for _, env := range envs {
		runArgs = append(runArgs, "--env", env)
	}
	runArgs = append(runArgs, image)
	runArgs = append(runArgs, args...)
	runArgs = append(runArgs, "-o", filepath.Join(dockerRepoDir, config.BinOutputPath), ".")

	cmd := exec.Command("docker", runArgs...)
	logger.Get(ctx).Info(
		"Building go package in docker",
		zap.String("package", config.PackagePath),
		zap.String("command", cmd.String()),
	)
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "building package '%s' failed", config.PackagePath)
	}
	return nil
}

// RunTests run tests.
func RunTests(ctx context.Context, deps types.DepsFunc, config TestConfig) error {
	deps(EnsureGo)

	args := append([]string{"test", "-v"}, config.Flags...)

	cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), args...)
	cmd.Dir = config.PackagePath
	cmd.Env = env()

	logger.Get(ctx).Info(
		"Running go tests",
		zap.String("package", config.PackagePath),
		zap.String("command", cmd.String()),
	)
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "go tests '%s' failed", config.PackagePath)
	}
	return nil
}

//go:embed Dockerfile.tmpl
var dockerfileTemplate string

var dockerfileTemplateParsed = template.Must(template.New("Dockerfile").Parse(dockerfileTemplate))

func ensureBuildDockerImage(ctx context.Context) (string, error) {
	goTool, err := tools.Get(tools.Go)
	if err != nil {
		return "", err
	}
	dockerfileBuf := &bytes.Buffer{}
	err = dockerfileTemplateParsed.Execute(dockerfileBuf, struct {
		GOVersion     string
		AlpineVersion string
	}{
		GOVersion:     goTool.GetVersion(),
		AlpineVersion: goAlpineVersion,
	})
	if err != nil {
		return "", errors.Wrap(err, "executing Dockerfile template failed")
	}
	dockerfileChecksum := sha256.Sum256(dockerfileBuf.Bytes())
	image := "crust-go-build:" + hex.EncodeToString(dockerfileChecksum[:4])

	imageBuf := &bytes.Buffer{}
	imageCmd := exec.Command("docker", "images", "-q", image)
	imageCmd.Stdout = imageBuf
	if err := libexec.Exec(ctx, imageCmd); err != nil {
		return "", errors.Wrapf(err, "failed to list image '%s'", image)
	}
	if imageBuf.Len() > 0 {
		return image, nil
	}

	buildCmd := exec.Command(
		"docker",
		"build",
		"--label", docker.LabelKey+"="+docker.LabelValue,
		"--tag", image,
		"--tag", "crust-go-build:latest",
		"-",
	)
	buildCmd.Stdin = dockerfileBuf

	if err := libexec.Exec(ctx, buildCmd); err != nil {
		return "", errors.Wrapf(err, "failed to build image '%s'", image)
	}
	return image, nil
}

func buildArgsAndEnvs(
	ctx context.Context,
	config BinaryBuildConfig,
	libDir string,
) (args, envs []string, err error) {
	var crossCompileARM64 bool

	switch config.TargetPlatform {
	case tools.TargetPlatformLocal,
		tools.TargetPlatformLinuxLocalArchInDocker,
		tools.TargetPlatformDarwinAMD64InDocker,
		tools.TargetPlatformDarwinARM64InDocker:
	case tools.TargetPlatformLinuxARM64InDocker:
		if config.CGOEnabled {
			if tools.TargetPlatformLocal != tools.TargetPlatformLinuxAMD64 {
				return nil, nil, errors.Errorf(
					"crosscompiling for %s is possible only on platform %s",
					config.TargetPlatform,
					tools.TargetPlatformLinuxAMD64,
				)
			}
			crossCompileARM64 = true
		}
	default:
		return nil, nil,
			errors.Errorf(
				"building is not possible for platform %s on platform %s",
				config.TargetPlatform,
				tools.TargetPlatformLocal,
			)
	}

	ldFlags := []string{"-w", "-s"}
	args = []string{
		"build",
		"-trimpath",
		"-ldflags=" + strings.Join(ldFlags, " "),
	}
	args = append(args, config.Flags...)

	cgoEnabled := "0"
	if config.CGOEnabled {
		cgoEnabled = "1"
	}
	envs = []string{
		"LIBRARY_PATH=" + libDir,
		"CGO_ENABLED=" + cgoEnabled,
		"GOOS=" + config.TargetPlatform.OS,
		"GOARCH=" + config.TargetPlatform.Arch,
	}
	if crossCompileARM64 {
		if err := tools.Ensure(ctx, tools.Aarch64LinuxMuslCross, tools.TargetPlatformLinuxAMD64InDocker); err != nil {
			return nil, nil, err
		}

		envs = append(envs, "CC=/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc")
	}
	envs = append(envs, config.Envs...)

	return args, envs, nil
}

// Generate calls `go generate` for specific package.
func Generate(ctx context.Context, deps types.DepsFunc) error {
	deps(EnsureGo)
	log := logger.Get(ctx)
	log.Info("Running go generate")

	cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), "generate", "./...")
	cmd.Dir = repoPath
	cmd.Env = env()
	if err := libexec.Exec(ctx, cmd); err != nil {
		return errors.Wrapf(err, "generation failed")
	}
	return nil
}

// Test runs go tests in repository.
func Test(ctx context.Context, deps types.DepsFunc) error {
	deps(EnsureGo)
	log := logger.Get(ctx)

	rootDir := filepath.Dir(must.String(filepath.Abs(must.String(filepath.EvalSymlinks(must.String(os.Getwd()))))))
	repoPath := must.String(filepath.Abs(must.String(filepath.EvalSymlinks(repoPath))))
	coverageReportsDir := filepath.Join(repoPath, "coverage")
	if err := os.MkdirAll(coverageReportsDir, 0o700); err != nil {
		return errors.WithStack(err)
	}

	return onModule(repoPath, func(path string) error {
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return errors.WithStack(err)
		}

		goCodePresent, err := containsGoCode(path)
		if err != nil {
			return err
		}
		if !goCodePresent {
			log.Info("No code to test", zap.String("path", path))
			return nil
		}

		if filepath.Base(path) == "integration-tests" {
			log.Info("Skipping integration-tests", zap.String("path", path))
			return nil
		}

		coverageName := strings.ReplaceAll(relPath, "/", "-")
		coverageProfile := filepath.Join(coverageReportsDir, coverageName)

		log.Info("Running go tests", zap.String("path", path))
		cmd := exec.Command(
			tools.Path("bin/go", tools.TargetPlatformLocal),
			"test",
			"-count=1",
			"-shuffle=on",
			"-race",
			"-cover", "./...",
			"-coverpkg", "./...",
			"-coverprofile", coverageProfile,
			"./...",
		)
		cmd.Dir = path
		cmd.Env = env()
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "unit tests failed in module '%s'", path)
		}
		return nil
	})
}

// Tidy runs go mod tidy in repository.
func Tidy(ctx context.Context, deps types.DepsFunc) error {
	deps(EnsureGo)
	log := logger.Get(ctx)
	return onModule(repoPath, func(path string) error {
		log.Info("Running go mod tidy", zap.String("path", path))

		cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), "mod", "tidy")
		cmd.Dir = path
		cmd.Env = env()
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "'go mod tidy' failed in module '%s'", path)
		}
		return nil
	})
}

// DownloadDependencies downloads all the go dependencies.
func DownloadDependencies(ctx context.Context, deps types.DepsFunc, repoPath string) error {
	deps(EnsureGo)
	log := logger.Get(ctx)
	return onModule(repoPath, func(path string) error {
		log.Info("Running go mod download", zap.String("path", path))

		cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), "mod", "download")
		cmd.Dir = path
		cmd.Env = env()
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "'go mod download' failed in module '%s'", path)
		}
		return nil
	})
}

func onModule(repoPath string, fn func(path string) error) error {
	return filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "go.mod" {
			return nil
		}
		return fn(filepath.Dir(path))
	})
}

func containsGoCode(path string) (bool, error) {
	errFound := errors.New("found")
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		return errFound
	})
	if errors.Is(err, errFound) {
		return true, nil
	}
	return false, errors.WithStack(err)
}

// ModuleDirs return directories where modules are kept.
func ModuleDirs(
	ctx context.Context,
	deps types.DepsFunc,
	repoPath string,
	modules ...string,
) (map[string]string, error) {
	deps(EnsureGo)

	out := &bytes.Buffer{}
	cmd := exec.Command(
		tools.Path("bin/go", tools.TargetPlatformLocal),
		append([]string{"list", "-m", "-json"}, modules...)...)
	cmd.Stdout = out
	cmd.Dir = repoPath
	cmd.Env = env()

	if err := libexec.Exec(ctx, cmd); err != nil {
		return nil, err
	}

	var info struct {
		Dir  string
		Path string
	}

	res := map[string]string{}
	dec := json.NewDecoder(out)
	for dec.More() {
		if err := dec.Decode(&info); err != nil {
			return nil, errors.WithStack(err)
		}
		res[info.Path] = info.Dir
	}

	return res, nil
}

// ModuleName returns name of the go module.
func ModuleName(modulePath string) (string, error) {
	f, err := os.Open(filepath.Join(modulePath, "go.mod"))
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer f.Close()

	rx := regexp.MustCompile("^module (.+?)$")

	s := bufio.NewScanner(f)
	s.Split(bufio.ScanLines)
	for s.Scan() {
		matches := rx.FindStringSubmatch(s.Text())
		if len(matches) < 2 {
			continue
		}
		return matches[1], nil
	}
	return "", nil
}

// GoPath returns $GOPATH.
func GoPath() string {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(must.String(os.UserHomeDir()), "go")
	}

	return goPath
}

func findModulePath(ctx context.Context, pkgRef any) (string, error) {
	if pkgRef == nil {
		path, err := filepath.Abs(repoPath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		return path, nil
	}

	out := &bytes.Buffer{}
	cmd := exec.Command(
		tools.Path("bin/go", tools.TargetPlatformLocal),
		"list", "-f", "{{ .Module.Dir }}", reflect.TypeOf(pkgRef).PkgPath())
	cmd.Dir = filepath.Join(repoPath, "build")
	cmd.Stdout = out
	cmd.Env = env()

	if err := libexec.Exec(ctx, cmd); err != nil {
		return "", err
	}

	modulePath := strings.TrimSuffix(out.String(), "\n")

	// FIXME (wojciech): Temporary hack
	if err := os.Chmod(modulePath, 0o755); err != nil {
		return "", errors.WithStack(err)
	}
	for _, file := range []string{
		"go.work",
		"go.work.sum",
	} {
		if err := os.Remove(filepath.Join(modulePath, file)); err != nil && !os.IsNotExist(err) {
			return "", errors.WithStack(err)
		}
	}

	return modulePath, nil
}
