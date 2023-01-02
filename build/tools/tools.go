package tools

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
)

// Tool names
const (
	Go           Name = "go"
	GolangCI     Name = "golangci"
	Ignite       Name = "ignite"
	Cosmovisor   Name = "cosmovisor"
	LibWASMMuslC Name = "libwasmvm_muslc"
	Gaia         Name = "gaia"
	Relayer      Name = "relayer"
)

var tools = map[Name]Tool{
	// https://go.dev/dl/
	Go: {
		Version:  "1.19.4",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://go.dev/dl/go1.19.4.linux-amd64.tar.gz",
				Hash: "sha256:c9c08f783325c4cf840a94333159cc937f05f75d36a8b307951d5bd959cf2ab8",
			},
			darwinAMD64: {
				URL:  "https://go.dev/dl/go1.19.4.darwin-amd64.tar.gz",
				Hash: "sha256:44894862d996eec96ef2a39878e4e1fce4d05423fc18bdc1cbba745ebfa41253",
			},
			darwinARM64: {
				URL:  "https://go.dev/dl/go1.19.4.darwin-arm64.tar.gz",
				Hash: "sha256:bb3bc5d7655b9637cfe2b5e90055dee93b0ead50e2ffd091df320d1af1ca853f",
			},
		},
		Binaries: map[string]string{
			"bin/go":    "go/bin/go",
			"bin/gofmt": "go/bin/gofmt",
		},
	},

	// https://github.com/golangci/golangci-lint/releases/
	GolangCI: {
		Version:  "1.50.1",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.50.1/golangci-lint-1.50.1-linux-amd64.tar.gz",
				Hash: "sha256:4ba1dc9dbdf05b7bdc6f0e04bdfe6f63aa70576f51817be1b2540bbce017b69a",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.50.1-linux-amd64/golangci-lint",
				},
			},
			darwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.50.1/golangci-lint-1.50.1-darwin-amd64.tar.gz",
				Hash: "sha256:0f615fb8c364f6e4a213f2ed2ff7aa1fc2b208addf29511e89c03534067bbf57",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.50.1-darwin-amd64/golangci-lint",
				},
			},
			darwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.50.1/golangci-lint-1.50.1-darwin-arm64.tar.gz",
				Hash: "sha256:3ca9753d7804b34f9165427fbe339dbea69bd80be8a10e3f02c6037393b2e1c4",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.50.1-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://github.com/ignite/cli/releases/
	// v0.23.0 is the last version based on Cosmos v0.45.x
	Ignite: {
		Version:  "0.23.0",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.23.0/ignite_0.23.0_linux_amd64.tar.gz",
				Hash: "sha256:915a96eb366fbf9c353af32d0ddb01796a30b86343ac77d613cc8a8af3dd395a",
			},
			darwinAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.23.0/ignite_0.23.0_darwin_amd64.tar.gz",
				Hash: "sha256:b9ca67a70f4d1b43609c4289a7e83dc2174754d35f30fb43f1518c0434361c4e",
			},
			darwinARM64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.23.0/ignite_0.23.0_darwin_arm64.tar.gz",
				Hash: "sha256:daefd4ac83e3bb384cf97a2ff8cc6e81427f74e2c81c567fd0507fce647146ec",
			},
		},
		Binaries: map[string]string{
			"bin/ignite": "ignite",
		},
	},

	// https://github.com/cosmos/cosmos-sdk/releases
	// There is 1.4.0, but it is a dummy release without changes as described here:
	// https://github.com/cosmos/cosmos-sdk/issues/13654
	// and they didn't even provide compiled binaries for it.
	Cosmovisor: {
		Version:   "1.3.0",
		ForDocker: true,
		Sources: Sources{
			dockerAMD64: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.3.0/cosmovisor-v1.3.0-linux-amd64.tar.gz",
				Hash: "sha256:34d7c9fbaa03f49b8278e13768d0fd82e28101dfa9625e25379c36a86d558826",
			},
			dockerARM64: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.3.0/cosmovisor-v1.3.0-linux-arm64.tar.gz",
				Hash: "sha256:8d7de2a18eb2cc4a749efbdbe060ecb34c3e5ca12354b7118a6966fa46d3a33d",
			},
		},
		Binaries: map[string]string{
			"bin/cosmovisor": "cosmovisor",
		},
	},

	// https://github.com/CosmWasm/wasmvm/releases
	// Check compatibility with wasmd beore upgrading: https://github.com/CosmWasm/wasmd
	LibWASMMuslC: {
		Version:   "v1.1.1",
		ForDocker: true,
		Sources: Sources{
			dockerAMD64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.1.1/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:6e4de7ba9bad4ae9679c7f9ecf7e283dd0160e71567c6a7be6ae47c81ebe7f32",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			dockerARM64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.1.1/libwasmvm_muslc.aarch64.a",
				Hash: "sha256:9ecb037336bd56076573dc18c26631a9d2099a7f2b40dc04b6cae31ffb4c8f9a",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.aarch64.a",
				},
			},
		},
	},

	// https://github.com/cosmos/gaia/releases
	// Before upgrading verify in go.mod that they use the same version of IBC
	Gaia: {
		Version:   "v7.1.0",
		ForDocker: true,
		Sources: Sources{
			dockerAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v7.1.0/gaiad-v7.1.0-linux-amd64",
				Hash: "sha256:7a24fd361b0259878a5aeb1f890ca0df20be1a875e7fc94aaef36eec4edf99c4",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v7.1.0-linux-amd64",
				},
			},
			dockerARM64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v7.1.0/gaiad-v7.1.0-linux-arm64",
				Hash: "sha256:4ac73edab3a967af4af2549d48ff8c7600f73e103766dc97f2eeff35fd6b8c50",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v7.1.0-linux-arm64",
				},
			},
		},
	},

	// https://github.com/cosmos/relayer/releases
	// There is v2.1.2 but it doesn't expose prometheus metrics we use to detect that relayer is ready
	Relayer: {
		Version:   "v2.1.0",
		ForDocker: true,
		Sources: Sources{
			dockerAMD64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.1.0/Cosmos.Relayer_2.1.0_linux_amd64.tar.gz",
				Hash: "sha256:893537acd7fa5b5b9f0814f06ce6c26ba3f944262d7a43f5790216350d8399a9",
			},
			dockerARM64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.1.0/Cosmos.Relayer_2.1.0_linux_arm64.tar.gz",
				Hash: "sha256:e6dddf04c03254e86a32d9c74b35514f8c46399f1e33e17f1ae29aaac4d1f1f1",
			},
		},
		Binaries: map[string]string{
			// "Cosmos Relayer" is the binary name in the archive
			"bin/relayer": "Cosmos Relayer",
		},
	},
}

// Name is the type used for defining tool names
type Name string

// Platform defines platform to install tool on
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "." + p.Arch
}

const dockerOS = "docker"

var (
	linuxAMD64  = Platform{OS: "linux", Arch: "amd64"}
	darwinAMD64 = Platform{OS: "darwin", Arch: "amd64"}
	darwinARM64 = Platform{OS: "darwin", Arch: "arm64"}
	dockerAMD64 = Platform{OS: dockerOS, Arch: "amd64"}
	dockerARM64 = Platform{OS: dockerOS, Arch: "arm64"}
)

// DockerPlatform is the docker platform for current arch
var DockerPlatform = Platform{
	OS:   dockerOS,
	Arch: runtime.GOARCH,
}

// Tool represents tool to be installed
type Tool struct {
	Version   string
	ForDocker bool
	ForLocal  bool
	Sources   Sources
	Binaries  map[string]string
}

// Source represents source where tool is fetched from
type Source struct {
	URL      string
	Hash     string
	Binaries map[string]string
}

// Sources is the map of sources
type Sources map[Platform]Source

// InstallAll installs all the tools
func InstallAll(ctx context.Context, deps build.DepsFunc) error {
	for tool := range tools {
		if tools[tool].ForLocal {
			if err := EnsureLocal(ctx, tool); err != nil {
				return err
			}
		}
		if tools[tool].ForDocker {
			if err := EnsureDocker(ctx, tool); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureLocal ensures that tool is installed locally
func EnsureLocal(ctx context.Context, tool Name) error {
	return ensurePlatform(ctx, tool, Platform{OS: runtime.GOOS, Arch: runtime.GOARCH})
}

// EnsureDocker ensures that tool is installed for docker
func EnsureDocker(ctx context.Context, tool Name) error {
	return ensurePlatform(ctx, tool, Platform{OS: dockerOS, Arch: runtime.GOARCH})
}

func ensurePlatform(ctx context.Context, tool Name, platform Platform) error {
	info, exists := tools[tool]
	if !exists {
		return errors.Errorf("tool %s is not defined", tool)
	}

	source, exists := info.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", tool, platform))
	}

	toolDir := toolDir(tool, platform)
	for dst, src := range lo.Assign(info.Binaries, source.Binaries) {
		srcPath, err := filepath.Abs(toolDir + "/" + src)
		if err != nil {
			return install(ctx, tool, info, platform)
		}

		dstPlatform := dst
		if platform.OS == dockerOS {
			dstPlatform = filepath.Join(CacheDir(), platform.String(), dstPlatform)
		}
		dstPath, err := filepath.Abs(dstPlatform)
		if err != nil {
			return install(ctx, tool, info, platform)
		}

		realPath, err := filepath.EvalSymlinks(dstPath)
		if err != nil || realPath != srcPath {
			return install(ctx, tool, info, platform)
		}

		fInfo, err := os.Stat(realPath)
		if err != nil {
			return install(ctx, tool, info, platform)
		}
		if fInfo.Mode()&0o700 == 0 {
			return install(ctx, tool, info, platform)
		}
	}
	return nil
}

func install(ctx context.Context, name Name, info Tool, platform Platform) (retErr error) {
	source, exists := info.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", name, platform))
	}
	ctx = logger.With(ctx, zap.String("name", string(name)), zap.String("version", info.Version),
		zap.String("url", source.URL))
	log := logger.Get(ctx)
	log.Info("Installing tool")

	resp, err := http.DefaultClient.Do(must.HTTPRequest(http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)))
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	hasher, expectedChecksum := hasher(source.Hash)
	reader := io.TeeReader(resp.Body, hasher)
	toolDir := toolDir(name, platform)
	if err := os.RemoveAll(toolDir); err != nil && !os.IsNotExist(err) {
		panic(err)
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

	actualChecksum := fmt.Sprintf("%02x", hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return errors.Errorf("checksum does not match for tool %s, expected: %s, actual: %s, url: %s", name,
			expectedChecksum, actualChecksum, source.URL)
	}

	dstDir := "."
	if platform.OS == dockerOS {
		dstDir = filepath.Join(CacheDir(), platform.String())
	}
	for dst, src := range lo.Assign(info.Binaries, source.Binaries) {
		srcPath := toolDir + "/" + src
		dstPath := dstDir + "/" + dst
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		if platform.OS == dockerOS {
			srcLinkPath := filepath.Join(strings.Repeat("../", strings.Count(dst, "/")), string(name)+"-"+info.Version, src)
			must.OK(os.Symlink(srcLinkPath, dstPath))
		} else {
			must.OK(os.Symlink(srcPath, dstPath))
		}
		must.Any(filepath.EvalSymlinks(dstPath))
		log.Info("Tool installed to path", zap.String("path", dstPath))
	}

	log.Info("Tool installed")
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

func save(url string, reader io.Reader, path string) error {
	switch {
	case strings.HasSuffix(url, ".tar.gz"):
		var err error
		reader, err = gzip.NewReader(reader)
		if err != nil {
			return errors.WithStack(err)
		}
		return untar(reader, path)
	default:
		//nolint:nosnakecase // O_* constants are delivered by the sdk and we can't change them to follow MixedCap
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

		// We take mode from header.FileInfo().Mode(), not from header.Mode because they may be in different formats (meaning of bits may be different).
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

			//nolint:nosnakecase // O_* constants are delivered by the sdk and we can't change them to follow MixedCap
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
			if err := ensureDir(header.Name); err != nil {
				return err
			}
			// linked file may not exist yet, so let's create it - it will be overwritten later
			//nolint:nosnakecase // O_* constants are delivered by the sdk and we can't change them to follow MixedCap
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

// CacheDir returns path to cache directory
func CacheDir() string {
	return must.String(os.UserCacheDir()) + "/crust"
}

func toolDir(name Name, platform Platform) string {
	info, exists := tools[name]
	if !exists {
		panic(errors.Errorf("tool %s is not defined", name))
	}

	return filepath.Join(CacheDir(), platform.String(), string(name)+"-"+info.Version)
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

// ByName returns tool definition by its name
func ByName(name Name) Tool {
	return tools[name]
}

// CopyToolBinaries moves the tools artifacts form the local cache to the target local location.
// In case the binPath doesn't exist the method will create it.
func CopyToolBinaries(tool Name, path string, binaryNames ...string) error {
	if len(binaryNames) == 0 {
		return nil
	}

	// map[name]path
	storedBinaryNames := make(map[string]string)
	// combine binaries
	for key := range ByName(tool).Binaries {
		storedBinaryNames[filepath.Base(PathDocker(key))] = key
	}
	for key := range ByName(tool).Sources[DockerPlatform].Binaries {
		storedBinaryNames[filepath.Base(PathDocker(key))] = key
	}

	// initial validation to check that we have all binaries
	for _, binaryName := range binaryNames {
		if _, ok := storedBinaryNames[binaryName]; !ok {
			return errors.Errorf("the binary %q doesn't exists for the requested tool %q", binaryName, tool)
		}
	}

	// create dir from path
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return errors.WithStack(err)
	}

	for _, binaryName := range binaryNames {
		storedBinaryPath := storedBinaryNames[binaryName]
		pathDocker := PathDocker(storedBinaryPath)

		// copy the file we need
		absPath, err := filepath.EvalSymlinks(pathDocker)
		if err != nil {
			return errors.WithStack(err)
		}
		fr, err := os.Open(absPath)
		if err != nil {
			return errors.WithStack(err)
		}
		defer fr.Close()
		fw, err := os.OpenFile(filepath.Join(path, binaryName), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o777) //nolint:nosnakecase // os constants
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

// PathLocal returns path to locally installed tool
func PathLocal(tool string) string {
	return must.String(filepath.Abs(filepath.Join("bin", tool)))
}

// PathDocker returns path to docker-installed tool
func PathDocker(tool string) string {
	return must.String(filepath.Abs(filepath.Join(CacheDir(), DockerPlatform.String(), tool)))
}
