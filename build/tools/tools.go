package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
)

// Tool names
const (
	Go           Name = "go"
	GolangCI     Name = "golangci"
	Ignite       Name = "ignite"
	Protoc       Name = "protoc"
	ProtocGenGo  Name = "protoc-gen-go"
	LibWASMMuslC Name = "libwasmvm_muslc"
)

var tools = map[Name]Tool{
	// https://go.dev/dl/
	Go: {
		Version:  "1.19",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://go.dev/dl/go1.19.linux-amd64.tar.gz",
				Hash: "sha256:464b6b66591f6cf055bc5df90a9750bf5fbc9d038722bb84a9d56a2bea974be6",
			},
			darwinAMD64: {
				URL:  "https://go.dev/dl/go1.19.darwin-amd64.tar.gz",
				Hash: "sha256:df6509885f65f0d7a4eaf3dfbe7dda327569787e8a0a31cbf99ae3a6e23e9ea8",
			},
			darwinARM64: {
				URL:  "https://go.dev/dl/go1.19.darwin-arm64.tar.gz",
				Hash: "sha256:859e0a54b7fcea89d9dd1ec52aab415ac8f169999e5fdfb0f0c15b577c4ead5e",
			},
		},
		Binaries: map[string]string{
			"bin/go":    "go/bin/go",
			"bin/gofmt": "go/bin/gofmt",
		},
	},

	// https://github.com/golangci/golangci-lint/releases/
	GolangCI: {
		Version:  "1.48.0",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.48.0/golangci-lint-1.48.0-linux-amd64.tar.gz",
				Hash: "sha256:127c5c9d47cf3a3cf4128815dea1d9623d57a83a22005e91b986b0cbceb09233",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.48.0-linux-amd64/golangci-lint",
				},
			},
			darwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.48.0/golangci-lint-1.48.0-darwin-amd64.tar.gz",
				Hash: "sha256:ec2e1c3bb3d34268cd57baba6b631127beb185bbe8cfde8ac40ba9b4c8615784",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.48.0-darwin-amd64/golangci-lint",
				},
			},
			darwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.48.0/golangci-lint-1.48.0-darwin-arm64.tar.gz",
				Hash: "sha256:ce69d7b94940c197ee3d293cfae7530191c094f76f9aecca97554058b12725ac",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.48.0-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://github.com/ignite/cli/releases/
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

	// https://github.com/protocolbuffers/protobuf/releases/
	Protoc: {
		Version:  "21.5",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v21.5/protoc-21.5-linux-x86_64.zip",
				Hash: "sha256:92fb4f5066a6f7b870e09c73115a2c861852af8f6555d8da9955fdb80710bf7f",
			},
			darwinAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v21.5/protoc-21.5-osx-x86_64.zip",
				Hash: "sha256:495d86aaaf5e8b536fbf04471ee9d7b21addeee5f1e949742c67bd09bb59c890",
			},
			darwinARM64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v21.5/protoc-21.5-osx-aarch_64.zip",
				Hash: "sha256:b22aed8dce62656687c6c4a323aab4e6baf1cb81ee423e77bc671bd69679e2c3",
			},
		},
		Binaries: map[string]string{
			"bin/protoc": "bin/protoc",
		},
	},

	// https://github.com/protocolbuffers/protobuf-go/releases
	ProtocGenGo: {
		Version:  "1.28.1",
		ForLocal: true,
		Sources: Sources{
			linuxAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf-go/releases/download/v1.28.1/protoc-gen-go.v1.28.1.linux.amd64.tar.gz",
				Hash: "sha256:5c5802081fb9998c26cdfe607017a677c3ceaa19aae7895dbb1eef9518ebcb7f",
			},
			darwinAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf-go/releases/download/v1.28.1/protoc-gen-go.v1.28.1.darwin.amd64.tar.gz",
				Hash: "sha256:6bc912fcc453741477568ae758c601ef74696e1e37027911f202479666f441f2",
			},
			darwinARM64: {
				URL:  "https://github.com/protocolbuffers/protobuf-go/releases/download/v1.28.1/protoc-gen-go.v1.28.1.darwin.arm64.tar.gz",
				Hash: "sha256:8ed99262b74cfdb89efbae8e2cb7d0409457d66dcf18dbdb124143186a6804d5",
			},
		},
		Binaries: map[string]string{
			"bin/protoc-gen-go": "protoc-gen-go",
		},
	},

	// https://github.com/CosmWasm/wasmvm/releases
	LibWASMMuslC: {
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
	for dst, src := range combine(info.Binaries, source.Binaries) {
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
	for dst, src := range combine(info.Binaries, source.Binaries) {
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
	case strings.HasSuffix(url, ".zip"):
		return unzip(reader, path)
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

func unzip(reader io.Reader, path string) error {
	// To unzip archive it is required to store it entirely on disk or in memory.
	// Zip does not support unzipping from one-way reader.
	// Here we store entire file in memory, so it's feasible only for small archives.
	archive, err := io.ReadAll(reader)
	if err != nil {
		return errors.WithStack(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		panic(err)
	}

	for _, f := range zr.File {
		filePath := filepath.Join(path, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, f.Mode()); err != nil {
				return errors.WithStack(err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return errors.WithStack(err)
		}

		err := func() error {
			fileInArchive, err := f.Open()
			if err != nil {
				return errors.WithStack(err)
			}
			defer fileInArchive.Close()

			dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return errors.WithStack(err)
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, fileInArchive)
			return errors.WithStack(err)

		}()
		if err != nil {
			return err
		}
	}
	return nil
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

// FIXME (wojciech): once build uses go 1.18 replace `combine` with https://github.com/samber/lo#assign
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
func ByName(name Name) Tool {
	return tools[name]
}

// Path returns path to installed tool
func Path(tool string) string {
	return must.String(filepath.Abs("bin/" + tool))
}
