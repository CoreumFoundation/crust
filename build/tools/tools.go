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

// Tool names.
const (
	Go           Name = "go"
	GolangCI     Name = "golangci"
	Ignite       Name = "ignite"
	Cosmovisor   Name = "cosmovisor"
	LibWASMMuslC Name = "libwasmvm_muslc"
	Gaia         Name = "gaia"
	Relayer      Name = "relayer"
	Hermes       Name = "hermes"
	CoredV100    Name = "cored-v1.0.0"
)

var tools = map[Name]Tool{
	// https://go.dev/dl/
	Go: {
		Version: "1.20.4",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://go.dev/dl/go1.20.4.linux-amd64.tar.gz",
				Hash: "sha256:698ef3243972a51ddb4028e4a1ac63dc6d60821bf18e59a807e051fee0a385bd",
			},
			PlatformDarwinAMD64: {
				URL:  "https://go.dev/dl/go1.20.4.darwin-amd64.tar.gz",
				Hash: "sha256:242b099b5b9bd9c5d4d25c041216bc75abcdf8e0541aec975eeabcbce61ad47f",
			},
			PlatformDarwinARM64: {
				URL:  "https://go.dev/dl/go1.20.4.darwin-arm64.tar.gz",
				Hash: "sha256:61bd4f7f2d209e2a6a7ce17787fc5fea52fb11cc9efb3d8471187a8b39ce0dc9",
			},
		},
		Binaries: map[string]string{
			"bin/go":    "go/bin/go",
			"bin/gofmt": "go/bin/gofmt",
		},
	},

	// https://github.com/golangci/golangci-lint/releases/
	GolangCI: {
		Version: "1.52.2",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.52.2/golangci-lint-1.52.2-linux-amd64.tar.gz",
				Hash: "sha256:c9cf72d12058a131746edd409ed94ccd578fbd178899d1ed41ceae3ce5f54501",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.52.2-linux-amd64/golangci-lint",
				},
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.52.2/golangci-lint-1.52.2-darwin-amd64.tar.gz",
				Hash: "sha256:e57f2599de73c4da1d36d5255b9baec63f448b3d7fb726ebd3cd64dabbd3ee4a",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.52.2-darwin-amd64/golangci-lint",
				},
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.52.2/golangci-lint-1.52.2-darwin-arm64.tar.gz",
				Hash: "sha256:89e523d45883903cfc472ab65621073f850abd4ffbb7720bbdd7ba66ee490bc8",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.52.2-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://github.com/ignite/cli/releases/
	// v0.23.0 is the last version based on Cosmos v0.45.x
	Ignite: {
		Version: "0.23.0",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.23.0/ignite_0.23.0_linux_amd64.tar.gz",
				Hash: "sha256:915a96eb366fbf9c353af32d0ddb01796a30b86343ac77d613cc8a8af3dd395a",
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.23.0/ignite_0.23.0_darwin_amd64.tar.gz",
				Hash: "sha256:b9ca67a70f4d1b43609c4289a7e83dc2174754d35f30fb43f1518c0434361c4e",
			},
			PlatformDarwinARM64: {
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
		Version: "1.3.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.3.0/cosmovisor-v1.3.0-linux-amd64.tar.gz",
				Hash: "sha256:34d7c9fbaa03f49b8278e13768d0fd82e28101dfa9625e25379c36a86d558826",
			},
			PlatformDockerARM64: {
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
		Version: "v1.1.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.1.1/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:6e4de7ba9bad4ae9679c7f9ecf7e283dd0160e71567c6a7be6ae47c81ebe7f32",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			PlatformDockerARM64: {
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
		Version: "v9.1.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v9.1.0/gaiad-v9.1.0-linux-amd64",
				Hash: "sha256:591cd6d5432a1996f9d658057ed9d446f1ecfaf5f060f3a7a30740d8722d7b5d",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v9.1.0-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v9.1.0/gaiad-v9.1.0-linux-arm64",
				Hash: "sha256:098c6fd577b7509c2a2d66a25f1b76bd57a7ed992250e22c6d848e603a30deeb",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v9.1.0-linux-arm64",
				},
			},
		},
	},

	// https://github.com/cosmos/relayer/releases
	Relayer: {
		Version: "v2.3.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.3.1/Cosmos.Relayer_2.3.1_linux_amd64.tar.gz",
				Hash: "sha256:68c94403959239683cc80f17e50ca99c7e5caff8d70a17d2171009969d4c2692",
				Binaries: map[string]string{
					"bin/relayer": "Cosmos Relayer_2.3.1_linux_amd64/rly",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.3.1/Cosmos.Relayer_2.3.1_linux_arm64.tar.gz",
				Hash: "sha256:5466606e6d1186ce70321a7ae421b7da121308960719caf6cc7c5a4923585230",
				Binaries: map[string]string{
					"bin/relayer": "Cosmos Relayer_2.3.1_linux_arm64/rly",
				},
			},
		},
	},

	// https://github.com/informalsystems/hermes/releases
	Hermes: {
		Version: "v1.4.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.4.1/hermes-v1.4.1-x86_64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:013b7603e915305e9cd0fc6164ad90e0c9751082a589f6efef706c8a4f562647",
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.4.1/hermes-v1.4.1-aarch64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:c11cd7eb941ee0ec4b62ef569f76ec04cb8bc09fe107dcb5732241a45fd19d8d",
			},
		},
		Binaries: map[string]string{
			"bin/hermes": "hermes",
		},
	},

	// https://github.com/CoreumFoundation/coreum/releases
	CoredV100: {
		Version: "v1.0.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v1.0.0/cored-linux-amd64",
				Hash: "sha256:34098ad7586bda364b1b2e7c4569cbcefb630cd4ed7c8f68eb5bced834082c57",
				Binaries: map[string]string{
					"bin/cored-v1.0.0": "cored-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v1.0.0/cored-linux-arm64",
				Hash: "sha256:3ced97f06607f0cdaf77e7ff0b36b2011d101c660684e4f3e54c2ac6bf344dd6",
				Binaries: map[string]string{
					"bin/cored-v1.0.0": "cored-linux-arm64",
				},
			},
		},
	},
}

// Name is the type used for defining tool names.
type Name string

// Platform defines platform to install tool on.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "." + p.Arch
}

// DockerOS represents docker environment.
const DockerOS = "docker"

// Platform definitions.
var (
	PlatformLocal       = Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
	PlatformLinuxAMD64  = Platform{OS: "linux", Arch: "amd64"}
	PlatformDarwinAMD64 = Platform{OS: "darwin", Arch: "amd64"}
	PlatformDarwinARM64 = Platform{OS: "darwin", Arch: "arm64"}
	PlatformDockerAMD64 = Platform{OS: DockerOS, Arch: "amd64"}
	PlatformDockerARM64 = Platform{OS: DockerOS, Arch: "arm64"}
	PlatformDockerLocal = Platform{OS: DockerOS, Arch: runtime.GOARCH}
)

// Tool represents tool to be installed.
type Tool struct {
	Version  string
	Local    bool
	Sources  Sources
	Binaries map[string]string
}

// Source represents source where tool is fetched from.
type Source struct {
	URL      string
	Hash     string
	Binaries map[string]string
}

// Sources is the map of sources.
type Sources map[Platform]Source

// InstallAll installs all the tools.
func InstallAll(ctx context.Context, deps build.DepsFunc) error {
	for tool := range tools {
		if tools[tool].Local {
			if err := EnsureTool(ctx, tool); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureTool ensures that tool is installed and available in bin folder.
func EnsureTool(ctx context.Context, tool Name) error {
	info, exists := tools[tool]
	if !exists {
		return errors.Errorf("tool %s is not defined", tool)
	}

	if !info.Local {
		return errors.Errorf("tool %s is not intended to be installed locally", tool)
	}

	if err := EnsureBinaries(ctx, tool, PlatformLocal); err != nil {
		return err
	}

	source, exists := info.Sources[PlatformLocal]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", tool, PlatformLocal))
	}

	for binaryName := range lo.Assign(info.Binaries, source.Binaries) {
		srcPath := filepath.Join(CacheDir(), PlatformLocal.String(), binaryName)

		absSrcPath, err := filepath.Abs(srcPath)
		if err != nil {
			return errors.WithStack(err)
		}
		realSrcPath, err := filepath.EvalSymlinks(absSrcPath)
		if err != nil {
			return errors.WithStack(err)
		}

		absDstPath, err := filepath.Abs(binaryName)
		if err != nil {
			return linkTool(binaryName, srcPath)
		}
		realDstPath, err := filepath.EvalSymlinks(absDstPath)
		if err != nil {
			return linkTool(binaryName, srcPath)
		}

		if realSrcPath != realDstPath {
			return linkTool(binaryName, srcPath)
		}
	}
	return nil
}

// EnsureBinaries ensures that tool's binaries are installed.
func EnsureBinaries(ctx context.Context, tool Name, platform Platform) error {
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
			return installBinary(ctx, tool, info, platform)
		}

		dstPath, err := filepath.Abs(filepath.Join(CacheDir(), platform.String(), dst))
		if err != nil {
			return installBinary(ctx, tool, info, platform)
		}

		realPath, err := filepath.EvalSymlinks(dstPath)
		if err != nil || realPath != srcPath {
			return installBinary(ctx, tool, info, platform)
		}

		fInfo, err := os.Stat(realPath)
		if err != nil {
			return installBinary(ctx, tool, info, platform)
		}
		if fInfo.Mode()&0o700 == 0 {
			return installBinary(ctx, tool, info, platform)
		}
	}
	return nil
}

func installBinary(ctx context.Context, name Name, info Tool, platform Platform) (retErr error) {
	source, exists := info.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", name, platform))
	}
	ctx = logger.With(ctx, zap.String("tool", string(name)), zap.String("version", info.Version),
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

	dstDir := filepath.Join(CacheDir(), platform.String())
	for dst, src := range lo.Assign(info.Binaries, source.Binaries) {
		srcPath := toolDir + "/" + src
		dstPath := dstDir + "/" + dst
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		srcLinkPath := filepath.Join(strings.Repeat("../", strings.Count(dst, "/")), "downloads", string(name)+"-"+info.Version, src)
		must.OK(os.Symlink(srcLinkPath, dstPath))
		must.Any(filepath.EvalSymlinks(dstPath))
		log.Info("Binary installed to path", zap.String("path", dstPath))
	}

	log.Info("Binaries installed")
	return nil
}

func linkTool(dst, src string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return errors.WithStack(err)
	}

	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.WithStack(err)
	}

	return errors.WithStack(os.Symlink(src, dst))
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

// CacheDir returns path to cache directory.
func CacheDir() string {
	return must.String(os.UserCacheDir()) + "/crust"
}

func toolDir(name Name, platform Platform) string {
	info, exists := tools[name]
	if !exists {
		panic(errors.Errorf("tool %s is not defined", name))
	}

	return filepath.Join(CacheDir(), platform.String(), "downloads", string(name)+"-"+info.Version)
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

// ByName returns tool definition by its name.
func ByName(name Name) Tool {
	return tools[name]
}

// CopyToolBinaries moves the tools artifacts from the local cache to the target local location.
// In case the binPath doesn't exist the method will create it.
func CopyToolBinaries(tool Name, platform Platform, path string, binaryNames ...string) error {
	info, exists := tools[tool]
	if !exists {
		return errors.Errorf("tool %s is not defined", tool)
	}

	infoPlatform, exists := info.Sources[platform]
	if !exists {
		return errors.Errorf("tool %s is not defined for platform %s", tool, platform)
	}

	if len(binaryNames) == 0 {
		return nil
	}

	storedBinaryNames := map[string]struct{}{}
	// combine binaries
	for key := range info.Binaries {
		storedBinaryNames[key] = struct{}{}
	}
	for key := range infoPlatform.Binaries {
		storedBinaryNames[key] = struct{}{}
	}

	// initial validation to check that we have all binaries
	for _, binaryName := range binaryNames {
		if _, ok := storedBinaryNames[binaryName]; !ok {
			return errors.Errorf("the binary %q doesn't exist for the requested tool %q", binaryName, tool)
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
		fw, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o777)
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

// Path returns path to the installed binary.
func Path(binary string, platform Platform) string {
	return must.String(filepath.Abs(filepath.Join(CacheDir(), platform.String(), binary)))
}
