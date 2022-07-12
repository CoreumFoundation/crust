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

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var tools = map[string]Tool{
	"go": {
		Version:  "1.18.3",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://go.dev/dl/go1.18.3.linux-amd64.tar.gz",
				Hash: "sha256:956f8507b302ab0bb747613695cdae10af99bbd39a90cae522b7c0302cc27245",
			},
			darwinAMD64: {
				URL:  "https://go.dev/dl/go1.18.3.darwin-amd64.tar.gz",
				Hash: "sha256:d9dcf8fc35da54c6f259be41954783a9f4984945a855d03a003a7fd6ea4c5ca1",
			},
			darwinARM64: {
				URL:  "https://go.dev/dl/go1.18.3.darwin-arm64.tar.gz",
				Hash: "sha256:40ecd383c941cc9f0682e6a6f2a333539d58c7dea15c842434d03afafe2f7242",
			},
		},
		Binaries: map[string]string{
			"bin/go":    "go/bin/go",
			"bin/gofmt": "go/bin/gofmt",
		},
	},
	"golangci": {
		Version:  "1.46.2",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.46.2/golangci-lint-1.46.2-linux-amd64.tar.gz",
				Hash: "sha256:242cd4f2d6ac0556e315192e8555784d13da5d1874e51304711570769c4f2b9b",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.46.2-linux-amd64/golangci-lint",
				},
			},
			darwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.46.2/golangci-lint-1.46.2-darwin-amd64.tar.gz",
				Hash: "sha256:658078aaaf7608693f37c4cf1380b2af418ab8b2d23fdb33e7e2d4339328590e",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.46.2-darwin-amd64/golangci-lint",
				},
			},
			darwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.46.2/golangci-lint-1.46.2-darwin-arm64.tar.gz",
				Hash: "sha256:81f9b4afd62ec5e612ef8bc3b1d612a88b56ff289874831845cdad394427385f",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.46.2-darwin-arm64/golangci-lint",
				},
			},
		},
	},
	"ignite": {
		Version:  "v0.22.2",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.22.2/ignite_0.22.2_linux_amd64.tar.gz",
				Hash: "sha256:c73654403ce7c27a8a8d2845a45beb284120c83d854c4312955658125764c296",
			},
			darwinAMD64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.22.2/ignite_0.22.2_darwin_amd64.tar.gz",
				Hash: "sha256:25cfb61c6c5d26ab303153211097d357f3c1bce63351276a870b7b6b4420b8b5",
			},
			darwinARM64: {
				URL:  "https://github.com/ignite/cli/releases/download/v0.22.2/ignite_0.22.2_darwin_arm64.tar.gz",
				Hash: "sha256:19757865d00e0d08c36a83a3cb9035a76ee0b542c20efed00f48a01eb12fb879",
			},
		},
		Binaries: map[string]string{
			"bin/ignite": "ignite",
		},
	},

	// https://github.com/CosmWasm/wasmvm/releases
	"libwasmvm_muslc": {
		Version:   "v1.0.0",
		ForDocker: true,
		Sources: Sources{
			dockerAMD64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:f6282df732a13dec836cda1f399dd874b1e3163504dbd9607c6af915b2740479",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			dockerARM64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.aarch64.a",
				Hash: "sha256:7d2239e9f25e96d0d4daba982ce92367aacf0cbd95d2facb8442268f2b1cc1fc",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.aarch64.a",
				},
			},
		},
	},
}

type platform struct {
	OS   string
	Arch string
}

func (p platform) String() string {
	return p.OS + "/" + p.Arch
}

const dockerOS = "docker"

var (
	linuxAMD64  = platform{OS: "linux", Arch: "amd64"}
	darwinAMD64 = platform{OS: "darwin", Arch: "amd64"}
	darwinARM64 = platform{OS: "darwin", Arch: "arm64"}
	dockerAMD64 = platform{OS: dockerOS, Arch: "amd64"}
	dockerARM64 = platform{OS: dockerOS, Arch: "arm64"}
)

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
type Sources map[platform]Source

// InstallAll installs all the tools
func InstallAll(ctx context.Context) error {
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
func EnsureLocal(ctx context.Context, tool string) error {
	return ensurePlatform(ctx, tool, platform{OS: runtime.GOOS, Arch: runtime.GOARCH})
}

// EnsureDocker ensures that tool is installed for docker
func EnsureDocker(ctx context.Context, tool string) error {
	return ensurePlatform(ctx, tool, platform{OS: dockerOS, Arch: runtime.GOARCH})
}

func ensurePlatform(ctx context.Context, tool string, platform platform) error {
	info, exists := tools[tool]
	if !exists {
		return errors.Errorf("tool %s is not defined", tool)
	}

	source, exists := info.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", tool, platform))
	}

	toolDir := toolDir(tool)
	for dst, src := range combine(info.Binaries, source.Binaries) {
		srcPath, err := filepath.Abs(toolDir + "/" + src)
		if err != nil {
			return install(ctx, tool, info, platform)
		}

		dstPlatform := dst
		if platform.OS == dockerOS {
			dstPlatform = filepath.Join(CacheDir(), dstPlatform)
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

func install(ctx context.Context, name string, info Tool, platform platform) (retErr error) {
	source, exists := info.Sources[platform]
	if !exists {
		panic(errors.Errorf("tool %s is not configured for platform %s", name, platform))
	}
	ctx = logger.With(ctx, zap.String("name", name), zap.String("version", info.Version),
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
	toolDir := toolDir(name)
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
		dstDir = filepath.Join(CacheDir(), dstDir)
	}
	for dst, src := range combine(info.Binaries, source.Binaries) {
		srcPath := toolDir + "/" + src
		srcLinkPath := filepath.Join(strings.Repeat("../", strings.Count(dst, "/")), name+"-"+info.Version, src)
		dstPath := dstDir + "/" + dst
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		if platform.OS == dockerOS {
			must.OK(os.Symlink(srcLinkPath, dstPath))
		} else {
			must.OK(os.Symlink(srcPath, dstPath))
		}
		must.Any(filepath.EvalSymlinks(dstPath))
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
			// linked file may not exist yet, so let's create it - i will be overwritten later
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

func toolDir(name string) string {
	info, exists := tools[name]
	if !exists {
		panic(errors.Errorf("tool %s is not defined", name))
	}

	return CacheDir() + "/" + name + "-" + info.Version
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

func combine(m1 map[string]string, m2 map[string]string) map[string]string {
	m := make(map[string]string, len(m1)+len(m2))
	for k, v := range m1 {
		m[k] = v
	}
	for k, v := range m2 {
		m[k] = v
	}
	return m
}

// ByName returns tool definition by its name
func ByName(name string) Tool {
	return tools[name]
}

// Path returns path to installed tool
func Path(tool string) string {
	return must.String(filepath.Abs("bin/" + tool))
}
