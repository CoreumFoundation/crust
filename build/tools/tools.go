package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/types"
)

// Tool names.
const (
	Go         Name = "go"
	GolangCI   Name = "golangci"
	RustUpInit Name = "rustup-init"
	Rust       Name = "rust"
	WASMOpt    Name = "wasm-opt"
	TyposLint  Name = "typos-lint"
)

func init() {
	AddTools(tools...)
}

var tools = []Tool{
	// https://go.dev/dl/
	BinaryTool{
		Name:    Go,
		Version: "1.24.2",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://go.dev/dl/go1.24.2.linux-amd64.tar.gz",
				Hash: "sha256:68097bd680839cbc9d464a0edce4f7c333975e27a90246890e9f1078c7e702ad",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://go.dev/dl/go1.24.2.darwin-amd64.tar.gz",
				Hash: "sha256:238d9c065d09ff6af229d2e3b8b5e85e688318d69f4006fb85a96e41c216ea83",
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://go.dev/dl/go1.24.2.darwin-arm64.tar.gz",
				Hash: "sha256:b70f8b3c5b4ccb0ad4ffa5ee91cd38075df20fdbd953a1daedd47f50fbcff47a",
			},
		},
		Binaries: map[string]string{
			"bin/go":    "go/bin/go",
			"bin/gofmt": "go/bin/gofmt",
		},
	},

	// https://github.com/golangci/golangci-lint/releases/
	BinaryTool{
		Name:    GolangCI,
		Version: "1.64.8",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.64.8/golangci-lint-1.64.8-linux-amd64.tar.gz",
				Hash: "sha256:b6270687afb143d019f387c791cd2a6f1cb383be9b3124d241ca11bd3ce2e54e",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.64.8-linux-amd64/golangci-lint",
				},
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.64.8/golangci-lint-1.64.8-darwin-amd64.tar.gz", //nolint:lll // breaking down urls is not beneficial
				Hash: "sha256:b52aebb8cb51e00bfd5976099083fbe2c43ef556cef9c87e58a8ae656e740444",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.64.8-darwin-amd64/golangci-lint",
				},
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.64.8/golangci-lint-1.64.8-darwin-arm64.tar.gz", //nolint:lll // breaking down urls is not beneficial
				Hash: "sha256:70543d21e5b02a94079be8aa11267a5b060865583e337fe768d39b5d3e2faf1f",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.64.8-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://rust-lang.github.io/rustup/installation/other.html
	BinaryTool{
		Name: RustUpInit,
		// update GCP bin source when update the version
		Version: "1.27.1",
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://storage.googleapis.com/cored-build-process-binaries/rustup-init/1.27.1/x86_64-unknown-linux-gnu/rustup-init", //nolint:lll // breaking down urls is not beneficial
				Hash: "sha256:6aeece6993e902708983b209d04c0d1dbb14ebb405ddb87def578d41f920f56d",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://storage.googleapis.com/cored-build-process-binaries/rustup-init/1.27.1/x86_64-apple-darwin/rustup-init", //nolint:lll // breaking down urls is not beneficial
				Hash: "sha256:f547d77c32d50d82b8228899b936bf2b3c72ce0a70fb3b364e7fba8891eba781",
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://storage.googleapis.com/cored-build-process-binaries/rustup-init/1.27.1/aarch64-apple-darwin/rustup-init", //nolint:lll // breaking down urls is not beneficial
				Hash: "sha256:760b18611021deee1a859c345d17200e0087d47f68dfe58278c57abe3a0d3dd0",
			},
		},
		Binaries: map[string]string{
			"bin/rustup-init": "rustup-init",
		},
	},

	// https://releases.rs
	RustInstaller{
		Version: "1.81.0",
	},

	// https://crates.io/crates/wasm-opt
	CargoTool{
		Name:    WASMOpt,
		Version: "0.116.0",
		Tool:    "wasm-opt",
	},

	BinaryTool{
		Name:    TyposLint,
		Version: "v1.31.1",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/crate-ci/typos/releases/download/v1.31.1/typos-v1.31.1-x86_64-unknown-linux-musl.tar.gz",
				Hash: "sha256:f683c2abeaff70379df7176110100e18150ecd17a4b9785c32908aca11929993",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/crate-ci/typos/releases/download/v1.31.1/typos-v1.31.1-x86_64-apple-darwin.tar.gz",
				Hash: "sha256:5e052ea461debbe03cfbdb2ed28cf0f12efdeda630cc23473db09ed795bf4f71",
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/crate-ci/typos/releases/download/v1.31.1/typos-v1.31.1-aarch64-apple-darwin.tar.gz",
				Hash: "sha256:a172195e1b1f1e011b3034913d1c87f0bbf0552a096b4ead0e3fa0620f4329cd",
			},
		},
		Binaries: map[string]string{
			"bin/typos": "typos",
		},
	},
}

// Name is the type used for defining tool names.
type Name string

var toolsMap = map[Name]Tool{}

// AddTools add tools to the toolset.
func AddTools(tools ...Tool) {
	for _, tool := range tools {
		toolsMap[tool.GetName()] = tool
	}
}

// OS constants.
const (
	OSLinux  = "linux"
	OSDarwin = "darwin"
)

// Arch constants.
const (
	ArchAMD64 = "amd64"
	ArchARM64 = "arm64"
)

// TargetPlatform defines platform to install tool on.
type TargetPlatform struct {
	BuildInDocker bool
	OS            string
	Arch          string
}

func (p TargetPlatform) String() string {
	path := make([]string, 0)
	if p.BuildInDocker {
		path = append(path, "docker")
	}
	path = append(path, p.OS, p.Arch)

	return strings.Join(path, ".")
}

// TargetPlatform definitions.
var (
	TargetPlatformLocal                  = TargetPlatform{BuildInDocker: false, OS: runtime.GOOS, Arch: runtime.GOARCH}
	TargetPlatformLinuxAMD64             = TargetPlatform{BuildInDocker: false, OS: OSLinux, Arch: ArchAMD64}
	TargetPlatformLinuxARM64             = TargetPlatform{BuildInDocker: false, OS: OSLinux, Arch: ArchARM64}
	TargetPlatformDarwinAMD64            = TargetPlatform{BuildInDocker: false, OS: OSDarwin, Arch: ArchAMD64}
	TargetPlatformDarwinARM64            = TargetPlatform{BuildInDocker: false, OS: OSDarwin, Arch: ArchARM64}
	TargetPlatformLinuxAMD64InDocker     = TargetPlatform{BuildInDocker: true, OS: OSLinux, Arch: ArchAMD64}
	TargetPlatformLinuxARM64InDocker     = TargetPlatform{BuildInDocker: true, OS: OSLinux, Arch: ArchARM64}
	TargetPlatformLinuxLocalArchInDocker = TargetPlatform{BuildInDocker: true, OS: OSLinux, Arch: runtime.GOARCH}
	TargetPlatformDarwinAMD64InDocker    = TargetPlatform{BuildInDocker: true, OS: OSDarwin, Arch: ArchAMD64}
	TargetPlatformDarwinARM64InDocker    = TargetPlatform{BuildInDocker: true, OS: OSDarwin, Arch: ArchARM64}
)

var (
	_ Tool = BinaryTool{}
	_ Tool = GoPackageTool{}
)

// Tool represents tool to be installed.
type Tool interface {
	GetName() Name
	GetVersion() string
	IsLocal() bool
	IsCompatible(platform TargetPlatform) bool
	GetBinaries(platform TargetPlatform) []string
	Ensure(ctx context.Context, platform TargetPlatform) error
}

// BinaryTool is the tool having compiled binaries available on the internet.
type BinaryTool struct {
	Name     Name
	Version  string
	Local    bool
	Sources  Sources
	Binaries map[string]string
}

// GetName returns the name of the tool.
func (bt BinaryTool) GetName() Name {
	return bt.Name
}

// GetVersion returns the version of the tool.
func (bt BinaryTool) GetVersion() string {
	return bt.Version
}

// IsLocal tells if tool should be installed locally.
func (bt BinaryTool) IsLocal() bool {
	return bt.Local
}

// IsCompatible tells if tool is defined for the platform.
func (bt BinaryTool) IsCompatible(platform TargetPlatform) bool {
	_, exists := bt.Sources[platform]
	return exists
}

// GetBinaries returns binaries defined for the platform.
func (bt BinaryTool) GetBinaries(platform TargetPlatform) []string {
	res := make([]string, 0, len(bt.Binaries)+len(bt.Sources[platform].Binaries))
	for k := range bt.Binaries {
		res = append(res, k)
	}
	for k := range bt.Sources[platform].Binaries {
		res = append(res, k)
	}
	return res
}

// Ensure ensures that tool is installed.
func (bt BinaryTool) Ensure(ctx context.Context, platform TargetPlatform) error {
	source, exists := bt.Sources[platform]
	if !exists {
		return errors.Errorf("tool %s is not configured for platform %s", bt.Name, platform)
	}

	var install bool
	for dst, src := range lo.Assign(bt.Binaries, source.Binaries) {
		if shouldReinstall(bt, platform, src, dst) {
			install = true
			break
		}
	}

	if install {
		if err := bt.install(ctx, platform); err != nil {
			return err
		}
	}

	return linkTool(bt, platform, lo.Keys(lo.Assign(bt.Binaries, source.Binaries))...)
}

func (bt BinaryTool) install(ctx context.Context, platform TargetPlatform) (retErr error) {
	source, exists := bt.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", bt.Name, platform))
	}
	ctx = logger.With(ctx, zap.String("tool", string(bt.Name)), zap.String("version", bt.Version),
		zap.String("url", source.URL), zap.Stringer("platform", platform))
	log := logger.Get(ctx)
	log.Info("Installing binaries")

	resp, err := http.DefaultClient.Do(must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)))
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	hasher, expectedChecksum := hasher(source.Hash)
	reader := io.TeeReader(resp.Body, hasher)
	toolDir := toolDir(bt, platform)
	if err := os.RemoveAll(toolDir); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(toolDir, 0o700); err != nil {
		panic(err)
	}
	defer func() {
		if retErr != nil {
			must.OK(os.RemoveAll(toolDir))
		}
	}()

	if err := save(source.URL, reader, toolDir); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return errors.Errorf("checksum does not match for tool %s, expected: %s, actual: %s, url: %s", bt.Name,
			expectedChecksum, actualChecksum, source.URL)
	}

	dstDir := filepath.Join(toolDir, "crust")
	for dst, src := range lo.Assign(bt.Binaries, source.Binaries) {
		srcPath := filepath.Join(toolDir, src)

		binChecksum, err := checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, dst)
		dstPathChecksum := dstPath + ":" + binChecksum
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Chmod(srcPath, 0o700); err != nil {
			return errors.WithStack(err)
		}
		srcLinkPath := filepath.Join(
			strings.Repeat("../", strings.Count(dst, "/")+1),
			src,
		)
		if err := os.Symlink(srcLinkPath, dstPathChecksum); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(filepath.Base(dstPathChecksum), dstPath); err != nil {
			return errors.WithStack(err)
		}
		log.Info("Binary installed to path", zap.String("path", dstPath))
	}

	log.Info("Binaries installed")
	return nil
}

// GoPackageTool is the tool installed using go install command.
type GoPackageTool struct {
	Name    Name
	Version string
	Package string
}

// GetName returns the name of the tool.
func (gpt GoPackageTool) GetName() Name {
	return gpt.Name
}

// GetVersion returns the version of the tool.
func (gpt GoPackageTool) GetVersion() string {
	return gpt.Version
}

// IsLocal tells if tool should be installed locally.
func (gpt GoPackageTool) IsLocal() bool {
	return true
}

// IsCompatible tells if tool is defined for the platform.
func (gpt GoPackageTool) IsCompatible(_ TargetPlatform) bool {
	return true
}

// GetBinaries returns binaries defined for the platform.
func (gpt GoPackageTool) GetBinaries(_ TargetPlatform) []string {
	return []string{
		"bin/" + filepath.Base(gpt.Package),
	}
}

// Ensure ensures that tool is installed.
func (gpt GoPackageTool) Ensure(ctx context.Context, platform TargetPlatform) error {
	binName := filepath.Base(gpt.Package)
	toolDir := toolDir(gpt, platform)
	dst := filepath.Join("bin", binName)

	//nolint:nestif // complexity comes from trivial error-handling ifs.
	if shouldReinstall(gpt, platform, binName, dst) {
		if err := Ensure(ctx, Go, platform); err != nil {
			return errors.Wrapf(err, "ensuring go failed")
		}

		cmd := exec.Command(Path("bin/go", TargetPlatformLocal), "install", gpt.Package+"@"+gpt.Version)
		cmd.Env = append(os.Environ(), "GOBIN="+toolDir)

		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}

		srcPath := filepath.Join(toolDir, binName)

		binChecksum, err := checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(toolDir, "crust", dst)
		dstPathChecksum := dstPath + ":" + binChecksum

		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Chmod(srcPath, 0o700); err != nil {
			return errors.WithStack(err)
		}
		srcLinkPath := filepath.Join("../..", binName)
		if err := os.Symlink(srcLinkPath, dstPathChecksum); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(filepath.Base(dstPathChecksum), dstPath); err != nil {
			return errors.WithStack(err)
		}
		if _, err := filepath.EvalSymlinks(dstPath); err != nil {
			return errors.WithStack(err)
		}
		logger.Get(ctx).Info("Binary installed to path", zap.String("path", dstPath))
	}

	return linkTool(gpt, platform, dst)
}

// RustInstaller installs rust.
type RustInstaller struct {
	Version string
}

// GetName returns the name of the tool.
func (ri RustInstaller) GetName() Name {
	return Rust
}

// GetVersion returns the version of the tool.
func (ri RustInstaller) GetVersion() string {
	return ri.Version
}

// IsLocal tells if tool should be installed locally.
func (ri RustInstaller) IsLocal() bool {
	return true
}

// IsCompatible tells if tool is defined for the platform.
func (ri RustInstaller) IsCompatible(platform TargetPlatform) bool {
	rustupInit, err := Get(RustUpInit)
	if err != nil {
		panic(err)
	}
	return rustupInit.IsCompatible(platform)
}

// GetBinaries returns binaries defined for the platform.
func (ri RustInstaller) GetBinaries(platform TargetPlatform) []string {
	return []string{
		"bin/cargo",
		"bin/rustc",
	}
}

// Ensure ensures that tool is installed.
func (ri RustInstaller) Ensure(ctx context.Context, platform TargetPlatform) error {
	binaries := ri.GetBinaries(platform)

	toolchain, err := ri.toolchain(platform)
	if err != nil {
		return err
	}

	install := toolchain == ""
	if !install {
		srcDir := filepath.Join(
			"rustup",
			"toolchains",
			toolchain,
		)

		for _, binary := range binaries {
			if shouldReinstall(ri, platform, filepath.Join(srcDir, binary), binary) {
				install = true
				break
			}
		}
	}

	if install {
		if err := ri.install(ctx, platform); err != nil {
			return err
		}
	}

	return linkTool(ri, platform, binaries...)
}

func (ri RustInstaller) install(ctx context.Context, platform TargetPlatform) (retErr error) {
	if err := Ensure(ctx, RustUpInit, platform); err != nil {
		return errors.Wrapf(err, "ensuring rustup-installer failed")
	}

	log := logger.Get(ctx)
	log.Info("Installing binaries")

	toolDir := toolDir(ri, platform)
	rustupHome := filepath.Join(toolDir, "rustup")
	toolchainsDir := filepath.Join(rustupHome, "toolchains")
	cargoHome := filepath.Join(toolDir, "cargo")
	rustupInstaller := Path("bin/rustup-init", platform)
	rustup := filepath.Join(cargoHome, "bin", "rustup")
	env := append(
		os.Environ(),
		"RUSTUP_HOME="+rustupHome,
		"CARGO_HOME="+cargoHome,
	)

	cmdRustupInstaller := exec.Command(rustupInstaller,
		"-y",
		"--no-update-default-toolchain",
		"--no-modify-path",
	)
	cmdRustupInstaller.Env = env

	cmdRustDefault := exec.Command(rustup,
		"default",
		ri.Version,
	)
	cmdRustDefault.Env = env

	cmdRustWASM := exec.Command(rustup,
		"target",
		"add",
		"wasm32-unknown-unknown",
	)
	cmdRustWASM.Env = env

	if err := libexec.Exec(ctx, cmdRustupInstaller, cmdRustDefault, cmdRustWASM); err != nil {
		return err
	}

	toolchain, err := ri.toolchain(platform)
	if err != nil {
		return err
	}

	srcDir := filepath.Join(
		"rustup",
		"toolchains",
		toolchain,
	)

	for _, binary := range ri.GetBinaries(platform) {
		binChecksum, err := checksum(filepath.Join(toolchainsDir, toolchain, binary))
		if err != nil {
			return err
		}

		dstPath := filepath.Join(toolDir, "crust", binary)
		dstPathChecksum := dstPath + ":" + binChecksum
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return errors.WithStack(err)
		}

		srcLinkPath := filepath.Join("../..", filepath.Join(srcDir, binary))
		if err := os.Symlink(srcLinkPath, dstPathChecksum); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(filepath.Base(dstPathChecksum), dstPath); err != nil {
			return errors.WithStack(err)
		}

		log.Info("Binary installed to path", zap.String("path", dstPath))
	}

	log.Info("Binaries installed")

	return nil
}

func (ri RustInstaller) toolchain(platform TargetPlatform) (string, error) {
	toolDir := toolDir(ri, platform)
	rustupHome := filepath.Join(toolDir, "rustup")
	toolchainsDir := filepath.Join(rustupHome, "toolchains")

	toolchains, err := os.ReadDir(toolchainsDir)
	switch {
	case err == nil:
		for _, dir := range toolchains {
			if dir.IsDir() && strings.HasPrefix(dir.Name(), ri.Version) {
				return dir.Name(), nil
			}
		}

		return "", nil
	case os.IsNotExist(err):
		return "", nil
	default:
		return "", errors.WithStack(err)
	}
}

// CargoTool is the tool installed using cargo install command.
type CargoTool struct {
	Name    Name
	Version string
	Tool    string
}

// GetName returns the name of the tool.
func (ct CargoTool) GetName() Name {
	return ct.Name
}

// GetVersion returns the version of the tool.
func (ct CargoTool) GetVersion() string {
	return ct.Version
}

// IsLocal tells if tool should be installed locally.
func (ct CargoTool) IsLocal() bool {
	return true
}

// IsCompatible tells if tool is defined for the platform.
func (ct CargoTool) IsCompatible(_ TargetPlatform) bool {
	return true
}

// GetBinaries returns binaries defined for the platform.
func (ct CargoTool) GetBinaries(_ TargetPlatform) []string {
	return []string{
		"bin/" + ct.Tool,
	}
}

// Ensure ensures that tool is installed.
func (ct CargoTool) Ensure(ctx context.Context, platform TargetPlatform) error {
	toolDir := toolDir(ct, platform)
	binPath := filepath.Join("bin", ct.Tool)

	//nolint:nestif // complexity comes from trivial error-handling ifs.
	if shouldReinstall(ct, platform, binPath, binPath) {
		if err := Ensure(ctx, Rust, platform); err != nil {
			return errors.Wrapf(err, "ensuring rust failed")
		}

		cmd := exec.Command(Path("bin/cargo", TargetPlatformLocal), "install",
			"--version", ct.Version, "--force", "--locked",
			"--root", toolDir, ct.Tool)
		cmd.Env = append(os.Environ(), "RUSTC="+Path("bin/rustc", TargetPlatformLocal))
		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}

		srcPath := filepath.Join(toolDir, "bin", ct.Tool)

		binChecksum, err := checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(toolDir, "crust", binPath)
		dstPathChecksum := dstPath + ":" + binChecksum

		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o700); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Chmod(srcPath, 0o700); err != nil {
			return errors.WithStack(err)
		}
		srcLinkPath := filepath.Join("../..", binPath)
		if err := os.Symlink(srcLinkPath, dstPathChecksum); err != nil {
			return errors.WithStack(err)
		}
		if err := os.Symlink(filepath.Base(dstPathChecksum), dstPath); err != nil {
			return errors.WithStack(err)
		}
		if _, err := filepath.EvalSymlinks(dstPath); err != nil {
			return errors.WithStack(err)
		}
		logger.Get(ctx).Info("Binary installed to path", zap.String("path", dstPath))
	}

	return linkTool(ct, platform, binPath)
}

// Source represents source where tool is fetched from.
type Source struct {
	URL      string
	Hash     string
	Binaries map[string]string
}

// Sources is the map of sources.
type Sources map[TargetPlatform]Source

// InstallAll installs all the toolsMap.
func InstallAll(ctx context.Context, deps types.DepsFunc) error {
	if err := Ensure(ctx, Go, TargetPlatformLocal); err != nil {
		return err
	}
	for toolName := range toolsMap {
		tool := toolsMap[toolName]
		if tool.IsLocal() {
			if err := tool.Ensure(ctx, TargetPlatformLocal); err != nil {
				return err
			}
		}
	}
	return nil
}

func linkTool(tool Tool, platform TargetPlatform, binaries ...string) error {
	for _, dst := range binaries {
		relink, err := shouldRelink(tool, platform, dst)
		if err != nil {
			return err
		}

		if !relink {
			continue
		}

		src := filepath.Join(
			strings.Repeat("../", strings.Count(dst, "/")+1),
			"downloads",
			fmt.Sprintf("%s-%s", tool.GetName(), tool.GetVersion()),
			"crust",
			dst,
		)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return errors.WithStack(err)
		}

		dstVersion := filepath.Join(VersionedRootPath(platform), dst)

		if err := os.Remove(dstVersion); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dstVersion), 0o700); err != nil {
			return errors.WithStack(err)
		}

		if err := os.Symlink(src, dstVersion); err != nil {
			return errors.WithStack(err)
		}

		if !tool.IsLocal() {
			continue
		}

		if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return errors.WithStack(err)
		}

		if err := os.Symlink(dstVersion, dst); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func hasher(hashStr string) (hash.Hash, string) {
	parts := strings.SplitN(hashStr, ":", 2)
	if len(parts) != 2 {
		panic(errors.Errorf("incorrect checksum format: %s", hashStr))
	}
	hashAlgorithm := parts[0]
	checksum := parts[1]

	var hasher hash.Hash
	switch hashAlgorithm {
	case "sha256":
		hasher = sha256.New()
	default:
		panic(errors.Errorf("unsupported hashing algorithm: %s", hashAlgorithm))
	}

	return hasher, strings.ToLower(checksum)
}

func checksum(file string) (string, error) {
	f, err := os.OpenFile(file, os.O_RDONLY, 0o600)
	if err != nil {
		return "", errors.WithStack(err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", errors.WithStack(err)
	}

	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func save(url string, reader io.Reader, path string) error {
	switch {
	case strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz"):
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return errors.WithStack(err)
		}
		return untar(reader, path)
	case strings.HasSuffix(url, ".zip"):
		return unpackZip(reader, path)
	default:
		f, err := os.OpenFile(filepath.Join(path, filepath.Base(url)), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
		if err != nil {
			return errors.WithStack(err)
		}
		defer f.Close()
		_, err = io.Copy(f, reader)
		return errors.WithStack(err)
	}
}

func untar(reader io.Reader, path string) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return errors.WithStack(err)
		case header == nil:
			continue
		}
		header.Name = path + "/" + header.Name

		// We take mode from header.FileInfo().Mode(), not from header.Mode because they may be in
		// different formats (meaning of bits may be different).
		// header.FileInfo().Mode() returns compatible value.
		mode := header.FileInfo().Mode()

		switch {
		case header.Typeflag == tar.TypeDir:
			if err := os.MkdirAll(header.Name, mode); err != nil && !os.IsExist(err) {
				return errors.WithStack(err)
			}
		case header.Typeflag == tar.TypeReg:
			if err := ensureDir(header.Name); err != nil {
				return err
			}

			f, err := os.OpenFile(header.Name, os.O_CREATE|os.O_WRONLY, mode)
			if err != nil {
				return errors.WithStack(err)
			}
			_, err = io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return errors.WithStack(err)
			}
		case header.Typeflag == tar.TypeSymlink:
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, header.Name); err != nil {
				return errors.WithStack(err)
			}
		case header.Typeflag == tar.TypeLink:
			header.Linkname = path + "/" + header.Linkname
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			if err := ensureDir(header.Linkname); err != nil {
				return err
			}
			// linked file may not exist yet, so let's create it - it will be overwritten later
			f, err := os.OpenFile(header.Linkname, os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				if !os.IsExist(err) {
					return errors.WithStack(err)
				}
			} else {
				_ = f.Close()
			}
			if err := os.Link(header.Linkname, header.Name); err != nil {
				return errors.WithStack(err)
			}
		default:
			return errors.Errorf("unsupported file type: %d", header.Typeflag)
		}
	}
}

func unpackZip(reader io.Reader, path string) error {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "zipfile")
	if err != nil {
		return errors.WithStack(err)
	}
	defer os.Remove(tempFile.Name()) //nolint: errcheck

	// Copy the contents of the reader to the temporary file
	_, err = io.Copy(tempFile, reader)
	if err != nil {
		return errors.WithStack(err)
	}

	// Open the temporary file for reading
	file, err := os.Open(tempFile.Name())
	if err != nil {
		return errors.WithStack(err)
	}
	defer file.Close()

	// Get the file information to obtain its size
	fileInfo, err := file.Stat()
	if err != nil {
		return errors.WithStack(err)
	}
	fileSize := fileInfo.Size()

	// Use the file as a ReaderAt to unpack the zip file
	zipReader, err := zip.NewReader(file, fileSize)
	if err != nil {
		return errors.WithStack(err)
	}

	// Process the files in the zip archive
	for _, zf := range zipReader.File {
		// Open each file in the archive
		rc, err := zf.Open()
		if err != nil {
			return errors.WithStack(err)
		}
		defer rc.Close()

		// Construct the destination path for the file
		destPath := filepath.Join(path, zf.Name)

		// skip empty dirs
		if zf.FileInfo().IsDir() {
			continue
		}

		err = os.MkdirAll(filepath.Dir(destPath), os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		// Create the file in the destination path
		outputFile, err := os.Create(destPath)
		if err != nil {
			return errors.WithStack(err)
		}
		defer outputFile.Close()

		// Copy the file contents
		_, err = io.Copy(outputFile, rc)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// CacheDir returns path to cache directory.
func CacheDir() string {
	return must.String(os.UserCacheDir()) + "/crust"
}

func toolDir(tool Tool, platform TargetPlatform) string {
	return filepath.Join(PlatformRootPath(platform), "downloads", string(tool.GetName())+"-"+tool.GetVersion())
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

func shouldReinstall(t Tool, platform TargetPlatform, src, dst string) bool {
	toolDir := toolDir(t, platform)

	srcPath, err := filepath.Abs(filepath.Join(toolDir, src))
	if err != nil {
		return true
	}

	dstPath, err := filepath.Abs(filepath.Join(toolDir, "crust", dst))
	if err != nil {
		return true
	}

	realPath, err := filepath.EvalSymlinks(dstPath)
	if err != nil || realPath != srcPath {
		return true
	}

	fInfo, err := os.Stat(realPath)
	if err != nil {
		return true
	}
	if fInfo.Mode()&0o700 == 0 {
		return true
	}

	linkedPath, err := os.Readlink(dstPath)
	if err != nil {
		return true
	}
	linkNameParts := strings.Split(filepath.Base(linkedPath), ":")
	if len(linkNameParts) < 3 {
		return true
	}

	hasher, expectedChecksum := hasher(linkNameParts[len(linkNameParts)-2] + ":" + linkNameParts[len(linkNameParts)-1])
	f, err := os.Open(realPath)
	if err != nil {
		return true
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return true
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	return actualChecksum != expectedChecksum
}

func shouldRelink(tool Tool, platform TargetPlatform, dst string) (bool, error) {
	srcPath := filepath.Join(toolDir(tool, platform), "crust", dst)

	realSrcPath, err := filepath.EvalSymlinks(srcPath)
	if err != nil {
		return false, errors.WithStack(err)
	}

	versionedPath := filepath.Join(VersionedRootPath(platform), dst)
	realVersionedPath, err := filepath.EvalSymlinks(versionedPath)
	if err != nil {
		return true, nil //nolint:nilerr // this is ok
	}

	if realSrcPath != realVersionedPath {
		return true, nil
	}

	if !tool.IsLocal() {
		return false, nil
	}

	absDstPath, err := filepath.Abs(dst)
	if err != nil {
		return true, nil //nolint:nilerr // this is ok
	}
	realDstPath, err := filepath.EvalSymlinks(absDstPath)
	if err != nil {
		return true, nil //nolint:nilerr // this is ok
	}

	if realSrcPath != realDstPath {
		return true, nil
	}

	return false, nil
}

// Get returns tool definition by its name.
func Get(name Name) (Tool, error) {
	t, exists := toolsMap[name]
	if !exists {
		return nil, errors.Errorf("tool %s does not exist", name)
	}
	return t, nil
}

// CopyToolBinaries moves the toolsMap artifacts from the local cache to the target local location.
// In case the binPath doesn't exist the method will create it.
func CopyToolBinaries(toolName Name, platform TargetPlatform, path string, binaryNames ...string) error {
	tool, err := Get(toolName)
	if err != nil {
		return err
	}

	if !tool.IsCompatible(platform) {
		return errors.Errorf("tool %s is not defined for platform %s", toolName, platform)
	}

	if len(binaryNames) == 0 {
		return nil
	}

	storedBinaryNames := map[string]struct{}{}
	// combine binaries
	for _, b := range tool.GetBinaries(platform) {
		storedBinaryNames[b] = struct{}{}
	}

	// initial validation to check that we have all binaries
	for _, binaryName := range binaryNames {
		if _, ok := storedBinaryNames[binaryName]; !ok {
			return errors.Errorf("the binary %q doesn't exist for the requested tool %q", binaryName, toolName)
		}
	}

	for _, binaryName := range binaryNames {
		dstPath := filepath.Join(path, binaryName)

		// create dir from path
		err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm)
		if err != nil {
			return errors.WithStack(err)
		}

		// copy the file we need
		fr, err := os.Open(Path(binaryName, platform))
		if err != nil {
			return errors.WithStack(err)
		}
		defer fr.Close()
		fw, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return errors.WithStack(err)
		}
		defer fw.Close()
		if _, err = io.Copy(fw, fr); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// PlatformRootPath returns path to the directory containing all platform-secific files.
func PlatformRootPath(platform TargetPlatform) string {
	return filepath.Join(CacheDir(), "tools", platform.String())
}

// VersionedRootPath returns the path to the root directory of crust version.
func VersionedRootPath(platform TargetPlatform) string {
	return filepath.Join(PlatformRootPath(platform), Version())
}

// Path returns path to the installed binary.
func Path(binary string, platform TargetPlatform) string {
	return must.String(filepath.Abs(must.String(filepath.EvalSymlinks(
		filepath.Join(VersionedRootPath(platform), binary)))))
}

// Ensure ensures tool exists for the platform.
func Ensure(ctx context.Context, toolName Name, platform TargetPlatform) error {
	tool, err := Get(toolName)
	if err != nil {
		return err
	}
	return tool.Ensure(ctx, platform)
}

// Version returns crust module version used to import this module in go.mod of the repository.
func Version() string {
	crustModule := CrustModule()

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("reading build info failed")
	}

	for _, m := range append([]*debug.Module{&bi.Main}, bi.Deps...) {
		if m.Path != crustModule {
			continue
		}
		if m.Replace != nil {
			m = m.Replace
		}

		// This happens in two cases:
		// - building is done in crust repository
		// - any other repository has `go.mod` modified to replace crust with the local source code
		if m.Version == "(devel)" {
			return "devel"
		}

		return m.Version
	}

	panic("impossible condition: crust module not found")
}

// CrustModule returns the name of crust module.
func CrustModule() string {
	//nolint:dogsled // yes, there are 3 blanks and what?
	_, file, _, _ := runtime.Caller(0)
	crustModule := strings.Join(strings.Split(file, "/")[:3], "/")
	index := strings.Index(crustModule, "@")
	if index > 0 {
		crustModule = crustModule[:index]
	}
	return crustModule
}
