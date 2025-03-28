package golang

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/docker"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
	"github.com/CoreumFoundation/crust/build/types"
)

const (
	repoPath = "."
)

const (
	funcPrefix         = "func "
	fuzzFuncPrefix     = funcPrefix + "Fuzz"
	fuzzTestFileSuffix = "fuzz_test.go"
)

// BinaryBuildConfig is the configuration for `go build`.
type BinaryBuildConfig struct {
	// TargetPlatform is the platform to build the binary for.
	TargetPlatform tools.TargetPlatform

	// PackagePath is the path to package to build relative to the ModulePath.
	PackagePath string

	// BinOutputPath is the path for compiled binary file.
	BinOutputPath string

	// CGOEnabled builds cgo binary.
	CGOEnabled bool

	// Tags is go build tags.
	Tags []string

	// Flags is a slice of additional ldflags to pass to `go build`. E.g -w, -s ... etc.
	LDFlags []string

	// Flags is a slice of additional flags to pass to `go build`. E.g -cover, -compiler ... etc.
	Flags []string

	// Envs is a slice of additional environment variables to pass to `go build`.
	Envs []string

	// DockerVolumes list of the volumes to use for the docker build.
	DockerVolumes []string
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
		if !strings.Contains(e, "GOROOT=") && !strings.Contains(e, "GOPATH=") &&
			!strings.Contains(e, "PATH=") && !strings.Contains(e, "GOVCS=") {
			envVars = append(envVars, envVar)
		}
	}
	envVars = append(envVars, "GOVCS=public:git|hg|bzr") // we add bzr here because some modules use it
	return append(envVars, "PATH="+lo.Must1(filepath.Abs(filepath.Join(repoPath, "bin")))+":"+os.Getenv("PATH"))
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
	if len(config.DockerVolumes) != 0 {
		return errors.New("the usage of the `DockerVolumes` config is prohibited for the local build")
	}

	if config.TargetPlatform != tools.TargetPlatformLocal {
		return errors.Errorf("building requested for platform %s while only %s is supported",
			config.TargetPlatform, tools.TargetPlatformLocal)
	}

	libDir := filepath.Join(tools.CacheDir(), "lib")
	if err := os.MkdirAll(libDir, 0o700); err != nil {
		return errors.WithStack(err)
	}
	args, envs, err := buildArgsAndEnvs(config, libDir)
	if err != nil {
		return err
	}
	args = append(args, "-o", must.String(filepath.Abs(config.BinOutputPath)), ".")
	envs = append(envs, env()...)

	modulePath, err := findModulePath(ctx)
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

	modulePath, err := findModulePath(ctx)
	if err != nil {
		return err
	}

	goTool, err := tools.Get(tools.Go)
	if err != nil {
		return err
	}

	// use goreleaser-cross with pre-installed cross-compilers
	image := "ghcr.io/goreleaser/goreleaser-cross:v" + goTool.GetVersion()

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

	outPath, err := filepath.Abs(filepath.Join(repoPath, filepath.Dir(config.BinOutputPath)))
	if err != nil {
		return errors.WithStack(err)
	}
	if err := os.MkdirAll(outPath, 0o755); err != nil {
		return errors.WithStack(err)
	}

	args, envs, err := buildArgsAndEnvs(config, filepath.Join("/crust-cache", tools.Version(), "lib"))
	if err != nil {
		return err
	}
	runArgs := []string{
		"run", "--rm",
		"--label", docker.LabelKey + "=" + docker.LabelValue,
		"-v", srcDir + ":/src",
		"-v", modulePath + ":" + dockerRepoDir,
		"-v", outPath + ":/out",
		"-v", goPath + ":/go",
		"-v", cacheDir + ":/crust-cache",
		"--env", "GOPATH=/go",
		"--env", "GOCACHE=/crust-cache/go-build",
		"--workdir", workDir,
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--name", "crust-build-" + filepath.Base(config.PackagePath) + "-" + hex.EncodeToString(nameSuffix),
		"--entrypoint", "go", // override default goreleaser
	}

	for _, env := range envs {
		runArgs = append(runArgs, "--env", env)
	}

	for _, v := range config.DockerVolumes {
		runArgs = append(runArgs, "-v", v)
	}

	runArgs = append(runArgs, image)
	runArgs = append(runArgs, args...)
	runArgs = append(runArgs, "-o", filepath.Join("/out", filepath.Base(config.BinOutputPath)), ".")

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

func buildArgsAndEnvs(
	config BinaryBuildConfig,
	libDir string,
) (args, envs []string, err error) {
	ldFlags := []string{"-w", "-s"}
	for _, flag := range config.Flags {
		if strings.Contains(flag, "-ldflags") {
			return nil, nil, errors.Errorf(
				"it's prohibited to use `-ldflags` in the Flags config, use LDFlags instead",
			)
		}
		if strings.Contains(flag, "-tags") {
			return nil, nil, errors.Errorf(
				"it's prohibited to use `-tags` in the Flags config, use Tags instead",
			)
		}
	}
	ldFlags = append(ldFlags, config.LDFlags...)

	args = []string{
		"build",
		"-trimpath",
		"-buildvcs=false",
	}
	if len(ldFlags) != 0 {
		args = append(args, "-ldflags="+strings.Join(ldFlags, " "))
	}
	if len(config.Tags) != 0 {
		args = append(args, "-tags="+strings.Join(config.Tags, ","))
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
	envs = append(envs, config.Envs...)

	return args, envs, nil
}

// Generate calls `go generate`.
func Generate(ctx context.Context, deps types.DepsFunc) error {
	deps(EnsureGo)
	log := logger.Get(ctx)

	return onModule(repoPath, func(path string) error {
		log.Info("Running go generate", zap.String("path", path))

		cmd := exec.Command(tools.Path("bin/go", tools.TargetPlatformLocal), "generate", "./...")
		cmd.Dir = path
		cmd.Env = env()
		if err := libexec.Exec(ctx, cmd); err != nil {
			return errors.Wrapf(err, "generation failed in module '%s'", path)
		}
		return nil
	})
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

// TestFuzz runs go fuzz tests in repository.
func TestFuzz(ctx context.Context, deps types.DepsFunc, fuzzTime time.Duration) error {
	deps(EnsureGo)
	log := logger.Get(ctx)

	repoPath := must.String(filepath.Abs(must.String(filepath.EvalSymlinks(repoPath))))
	return onModule(repoPath, func(path string) error {
		fuzzTests, err := findFuzzTests(path)
		if err != nil {
			return err
		}
		if len(fuzzTests) == 0 {
			log.Info("No fuzz tests found", zap.String("path", path))
			return nil
		}

		goFuzzCachePath := filepath.Join(tools.CacheDir(), "go-fuzz")

		for filePath, testNames := range fuzzTests {
			fileHash, err := getFileHash(filePath)
			if err != nil {
				return err
			}

			fuzzCacheDir := filepath.Join(goFuzzCachePath, fileHash)
			if err := os.MkdirAll(fuzzCacheDir, 0o700); err != nil {
				return errors.WithStack(err)
			}

			for _, testName := range testNames {
				args := []string{
					"test",
					"-v",
					"-fuzz", testName,
					"-run", "^$", // An empty regex that matches nothing, to avoid running unit tests
					"-fuzztime", fuzzTime.String(),
					"-test.fuzzcachedir", fuzzCacheDir,
				}
				log.Info("Running fuzz test", zap.Strings("args", args))
				cmd := exec.Command(
					tools.Path("bin/go", tools.TargetPlatformLocal),
					args...,
				)
				cmd.Dir = filepath.Dir(filePath)
				cmd.Env = env()
				if err := libexec.Exec(ctx, cmd); err != nil {
					return errors.Wrapf(err, "fuzz tests failed in module '%s'", path)
				}
			}
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

func findModulePath(ctx context.Context) (string, error) {
	file := findMainFile()
	if file == "" {
		path, err := filepath.Abs(repoPath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		return path, nil
	}
	commitID := findCommitID(file)
	if commitID == "" {
		path, err := filepath.Abs(repoPath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		return path, nil
	}

	repoURL := findRepositoryURL(file)
	repoName := filepath.Base(repoURL)
	repoDir := filepath.Join(tools.CacheDir(), "repos", repoName)

	if err := git.CloneRemoteCommit(ctx, repoURL, commitID, repoDir); err != nil {
		return "", err
	}

	return repoDir, nil
}

func findMainFile() string {
	crustModule := tools.CrustModule()
	for i := 1; ; i++ {
		_, file, _, ok := runtime.Caller(i)
		if !ok || strings.HasPrefix(file, "runtime/") {
			return ""
		}
		if !strings.HasPrefix(file, crustModule) {
			return file
		}
	}
}

func findCommitID(path string) string {
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		index := strings.Index(parts[i], "@")
		if index >= 0 {
			tagParts := strings.Split(parts[i][index+1:], "-")
			return tagParts[len(tagParts)-1]
		}
	}
	return ""
}

func findRepositoryURL(path string) string {
	parts := strings.Split(path, "/")
	parts = parts[:3]
	index := strings.Index(parts[2], "@")
	if index >= 0 {
		parts[2] = parts[2][:index]
	}

	return "https://" + strings.Join(parts, "/")
}

func findFuzzTests(path string) (map[string][]string, error) {
	fuzzTestPaths := make(map[string][]string, 0)
	if err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), fuzzTestFileSuffix) {
			return nil
		}

		testNames := make([]string, 0)
		file, err := os.Open(path)
		if err != nil {
			return errors.WithStack(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, fuzzFuncPrefix) {
				lines := strings.Split(strings.Replace(line, funcPrefix, "", 1), "(")
				if len(lines) != 2 {
					return errors.Errorf("invalid fuzz test %s", line)
				}
				testNames = append(testNames, lines[0])
			}
		}

		if len(testNames) == 0 {
			return errors.Errorf("invlaid %s fuzz test file, no fuzz tests found", path)
		}

		fuzzTestPaths[path] = testNames

		return nil
	}); err != nil {
		return nil, errors.WithStack(err)
	}

	return fuzzTestPaths, nil
}

func getFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer f.Close()

	fileHash := sha256.New()
	if _, err := io.Copy(fileHash, f); err != nil {
		return "", errors.WithStack(err)
	}

	return hex.EncodeToString(fileHash.Sum(nil)), nil
}
