package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
)

// Tool names.
const (
	Go                    Name = "go"
	GolangCI              Name = "golangci"
	Cosmovisor            Name = "cosmovisor"
	Aarch64LinuxMuslCross Name = "aarch64-linux-musl-cross"
	LibWASMMuslC          Name = "libwasmvm_muslc"
	Gaia                  Name = "gaia"
	Osmosis               Name = "osmosis"
	Hermes                Name = "hermes"
	CoredV202             Name = "cored-v2.0.2"
	Buf                   Name = "buf"
	Protoc                Name = "protoc"
	ProtocGenDoc          Name = "protoc-gen-doc"
	ProtocGenGRPCGateway  Name = "protoc-gen-grpc-gateway"
	ProtocGenOpenAPIV2    Name = "protoc-gen-openapiv2"
	ProtocGenGoCosmos     Name = "protoc-gen-gocosmos"
	ProtocGenBufLint      Name = "protoc-gen-buf-lint"
	ProtocGenBufBreaking  Name = "protoc-gen-buf-breaking"
)

var tools = []Tool{
	// https://go.dev/dl/
	BinaryTool{
		Name:    Go,
		Version: "1.21.0",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://go.dev/dl/go1.21.0.linux-amd64.tar.gz",
				Hash: "sha256:d0398903a16ba2232b389fb31032ddf57cac34efda306a0eebac34f0965a0742",
			},
			PlatformDarwinAMD64: {
				URL:  "https://go.dev/dl/go1.21.0.darwin-amd64.tar.gz",
				Hash: "sha256:b314de9f704ab122c077d2ec8e67e3670affe8865479d1f01991e7ac55d65e70",
			},
			PlatformDarwinARM64: {
				URL:  "https://go.dev/dl/go1.21.0.darwin-arm64.tar.gz",
				Hash: "sha256:3aca44de55c5e098de2f406e98aba328898b05d509a2e2a356416faacf2c4566",
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
		Version: "1.54.0",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.54.0/golangci-lint-1.54.0-linux-amd64.tar.gz",
				Hash: "sha256:a694f19dbfab3ea4d3956cb105d2e74c1dc49cb4c06ece903a3c534bce86b3dc",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.54.0-linux-amd64/golangci-lint",
				},
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.54.0/golangci-lint-1.54.0-darwin-amd64.tar.gz",
				Hash: "sha256:0a76fcb91bca94c0b3bcb931662eafd320fbe458b3a29ce368b0bffbd4eff2fb",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.54.0-darwin-amd64/golangci-lint",
				},
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.54.0/golangci-lint-1.54.0-darwin-arm64.tar.gz",
				Hash: "sha256:aeb77a00c24720e223ef73da18eea3afb29ea46356db33e1f503c66f2799d387",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.54.0-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://github.com/cosmos/cosmos-sdk/releases
	BinaryTool{
		Name:    Cosmovisor,
		Version: "1.5.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.5.0/cosmovisor-v1.5.0-linux-amd64.tar.gz",
				Hash: "sha256:7f4bebfb18a170bff1c725f13dda326e0158132deef9f037ab0c2a48727c3077",
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.5.0/cosmovisor-v1.5.0-linux-arm64.tar.gz",
				Hash: "sha256:e15f2625b1d208ac2fed51bc84ae75678009888648ac2186fd0ed5ab6177dc14",
			},
		},
		Binaries: map[string]string{
			"bin/cosmovisor": "cosmovisor",
		},
	},

	// http://musl.cc/#binaries
	BinaryTool{
		Name:    Aarch64LinuxMuslCross,
		Version: "11.2.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "http://musl.cc/aarch64-linux-musl-cross.tgz",
				Hash: "sha256:c909817856d6ceda86aa510894fa3527eac7989f0ef6e87b5721c58737a06c38",
			},
		},
		Binaries: map[string]string{
			"bin/aarch64-linux-musl-gcc": "aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc",
		},
	},

	// https://github.com/CosmWasm/wasmvm/releases
	// Check compatibility with wasmd beore upgrading: https://github.com/CosmWasm/wasmd
	BinaryTool{
		Name:    LibWASMMuslC,
		Version: "v1.3.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.3.0/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:b4aad4480f9b4c46635b4943beedbb72c929eab1d1b9467fe3b43e6dbf617e32",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.3.0/libwasmvm_muslc.aarch64.a",
				Hash: "sha256:b1610f9c8ad8bdebf5b8f819f71d238466f83521c74a2deb799078932e862722",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.aarch64.a",
				},
			},
		},
	},

	// https://github.com/cosmos/gaia/releases
	// Before upgrading verify in go.mod that they use the same version of IBC
	BinaryTool{
		Name:    Gaia,
		Version: "v11.0.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v11.0.0/gaiad-v11.0.0-linux-amd64",
				Hash: "sha256:258df2eec5b22f8baadc988e184fbfd2ae6f9f888e9f4461a110cc365fe86300",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v11.0.0-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v11.0.0/gaiad-v11.0.0-linux-arm64",
				Hash: "sha256:688e3ae4aa5ed91978f537798e012322336c7309fe5ee9169fdd607ab6c348b8",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v11.0.0-linux-arm64",
				},
			},
			PlatformLinuxAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v11.0.0/gaiad-v11.0.0-linux-amd64",
				Hash: "sha256:258df2eec5b22f8baadc988e184fbfd2ae6f9f888e9f4461a110cc365fe86300",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v11.0.0-linux-amd64",
				},
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v11.0.0/gaiad-v11.0.0-darwin-amd64",
				Hash: "sha256:f115875122386496254905a1de0c0cb45f1b731536281586f77a41be55458505",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v11.0.0-darwin-amd64",
				},
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v11.0.0/gaiad-v11.0.0-darwin-arm64",
				Hash: "sha256:53d0ffe4d8353e51d0be543edf764de033e24d703d4c408244a141e635b27628",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v11.0.0-darwin-arm64",
				},
			},
		},
	},

	BinaryTool{
		Name:    Osmosis,
		Version: "16.1.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-linux-amd64",
				Hash: "sha256:0ec66e32584fff24b6d62fc9938c69ff1a1bbdd8641d2ec9e0fd084aaa767ed3",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-linux-arm64",
				Hash: "sha256:e2ccc743dd66da91d1df1ae4ecf92b36d658575f4ff507d5056eb640804e0401",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-linux-arm64",
				},
			},
			PlatformLinuxAMD64: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-linux-amd64",
				Hash: "sha256:0ec66e32584fff24b6d62fc9938c69ff1a1bbdd8641d2ec9e0fd084aaa767ed3",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-linux-amd64",
				},
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-darwin-amd64",
				Hash: "sha256:d856ebda9c31f052d10a78443967a93374f2033292f0afdb6434b82b4ed79790",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-darwin-amd64",
				},
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-darwin-arm64",
				Hash: "sha256:c743da4d3632a2bc3ea0ce784bbd13383492a4a34d53295eb2c96987bacf8e8c",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-darwin-arm64",
				},
			},
		},
	},

	// https://github.com/informalsystems/hermes/releases
	BinaryTool{
		Name:    Hermes,
		Version: "v1.6.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.6.0/hermes-v1.6.0-x86_64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:20d2798e221a6b90956bfd237bb171b5ca5fd3e1368fb58d4fbac3dc0093aadb",
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.6.0/hermes-v1.6.0-aarch64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:3d4939aef95886d5016f1346de62a16e18469326ecf9b1159aa571ab8682b38d",
			},
		},
		Binaries: map[string]string{
			"bin/hermes": "hermes",
		},
	},

	// https://github.com/CoreumFoundation/coreum/releases
	BinaryTool{
		Name:    CoredV202,
		Version: "v2.0.2",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-amd64",
				Hash: "sha256:3facf55f7ff795719f68b9bcf76ea08262bc7c9e9cd735c660257ba73678250e",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-arm64",
				Hash: "sha256:35e261eb3b87c833c30174e6b8667a6155f5962441275d443157e209bbb0bf0d",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-arm64",
				},
			},
		},
	},
	BinaryTool{
		Name:    CoredV202,
		Version: "v2.0.2",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-amd64",
				Hash: "sha256:3facf55f7ff795719f68b9bcf76ea08262bc7c9e9cd735c660257ba73678250e",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-arm64",
				Hash: "sha256:35e261eb3b87c833c30174e6b8667a6155f5962441275d443157e209bbb0bf0d",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-arm64",
				},
			},
		},
	},

	// https://github.com/bufbuild/buf/releases
	BinaryTool{
		Name:    Buf,
		Version: "v1.27.2",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.27.2/buf-Linux-x86_64",
				Hash: "sha256:512893e5802eff80611104fb0aa75cc3729d95ef7697deddf5e7e86f468408b3",
				Binaries: map[string]string{
					"bin/buf": "buf-Linux-x86_64",
				},
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.27.2/buf-Darwin-x86_64",
				Hash: "sha256:7f22d4e102b91624fd8bc18899a0c0c467790ab12b421a21617fad8a9ca7d5b6",
				Binaries: map[string]string{
					"bin/buf": "buf-Darwin-x86_64",
				},
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.27.2/buf-Darwin-arm64",
				Hash: "sha256:1d8a49c12890830bdeb2839b4903af4695700a11c787c4e2f683a1eb3352badd",
				Binaries: map[string]string{
					"bin/buf": "buf-Darwin-arm64",
				},
			},
		},
	},

	// https://github.com/protocolbuffers/protobuf/releases
	BinaryTool{
		Name:    Protoc,
		Version: "v24.0",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v24.0/protoc-24.0-linux-x86_64.zip",
				Hash: "sha256:4feef12d91c4e452eac8c45bd11f43d51541db7fc6c18b4ca7bdd38af52c9d15",
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v24.0/protoc-24.0-osx-x86_64.zip",
				Hash: "sha256:c438ae68427aa848f4e2dbf7d6cbd4e6a13e30a6bcc61007fd95cfc90451264a",
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v24.0/protoc-24.0-osx-aarch_64.zip",
				Hash: "sha256:e4cc0739f0f8ae31633fb11335f11e6fbe067ecda8fd1b4716e80cfe3661ee1d",
			},
		},
		Binaries: map[string]string{
			"bin/protoc": "bin/protoc",
		},
	},

	// https://github.com/pseudomuto/protoc-gen-doc/releases/
	BinaryTool{
		Name:    ProtocGenDoc,
		Version: "v1.5.1",
		Local:   true,
		Sources: Sources{
			PlatformLinuxAMD64: {
				URL:  "https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.5.1/protoc-gen-doc_1.5.1_linux_amd64.tar.gz",
				Hash: "sha256:47cd72b07e6dab3408d686a65d37d3a6ab616da7d8b564b2bd2a2963a72b72fd",
			},
			PlatformDarwinAMD64: {
				URL:  "https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.5.1/protoc-gen-doc_1.5.1_darwin_amd64.tar.gz",
				Hash: "sha256:f429e5a5ddd886bfb68265f2f92c1c6a509780b7adcaf7a8b3be943f28e144ba",
			},
			PlatformDarwinARM64: {
				URL:  "https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.5.1/protoc-gen-doc_1.5.1_darwin_arm64.tar.gz",
				Hash: "sha256:6e8c737d9a67a6a873a3f1d37ed8bb2a0a9996f6dcf6701aa1048c7bd798aaf9",
			},
		},
		Binaries: map[string]string{
			"bin/protoc-gen-doc": "protoc-gen-doc",
		},
	},

	// https://github.com/grpc-ecosystem/grpc-gateway/releases
	GoPackageTool{
		Name:    ProtocGenGRPCGateway,
		Version: "v1.16.0",
		Package: "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway",
	},

	// https://github.com/grpc-ecosystem/grpc-gateway/releases
	GoPackageTool{
		Name:    ProtocGenOpenAPIV2,
		Version: "v2.17.0",
		Package: "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2",
	},

	// https://github.com/regen-network/cosmos-proto/releases
	GoPackageTool{
		Name:    ProtocGenGoCosmos,
		Version: "v1.4.10",
		Package: "github.com/cosmos/gogoproto/protoc-gen-gocosmos",
	},

	// https://github.com/bufbuild/buf/releases
	GoPackageTool{
		Name:    ProtocGenBufLint,
		Version: "v1.26.1",
		Package: "github.com/bufbuild/buf/cmd/protoc-gen-buf-lint",
	},

	// https://github.com/bufbuild/buf/releases
	GoPackageTool{
		Name:    ProtocGenBufBreaking,
		Version: "v1.26.1",
		Package: "github.com/bufbuild/buf/cmd/protoc-gen-buf-breaking",
	},
}

var toolsMap = func(tools []Tool) map[Name]Tool {
	res := make(map[Name]Tool, len(tools))
	for _, tool := range tools {
		res[tool.GetName()] = tool
	}
	return res
}(tools)

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

var (
	_ Tool = BinaryTool{}
	_ Tool = GoPackageTool{}
)

// Tool represents tool to be installed.
type Tool interface {
	GetName() Name
	GetVersion() string
	IsLocal() bool
	IsCompatible(platform Platform) bool
	GetBinaries(platform Platform) []string
	Ensure(ctx context.Context, platform Platform) error
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
func (bt BinaryTool) IsCompatible(platform Platform) bool {
	_, exists := bt.Sources[platform]
	return exists
}

// GetBinaries returns binaries defined for the platform.
func (bt BinaryTool) GetBinaries(platform Platform) []string {
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
func (bt BinaryTool) Ensure(ctx context.Context, platform Platform) error {
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

	for dst := range lo.Assign(bt.Binaries, source.Binaries) {
		if bt.Local {
			if err := linkTool(dst); err != nil {
				return err
			}
		}
	}

	return nil
}

func (bt BinaryTool) install(ctx context.Context, platform Platform) (retErr error) {
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

	actualChecksum := fmt.Sprintf("%02x", hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return errors.Errorf("checksum does not match for tool %s, expected: %s, actual: %s, url: %s", bt.Name,
			expectedChecksum, actualChecksum, source.URL)
	}

	dstDir := BinariesRootPath(platform)
	for dst, src := range lo.Assign(bt.Binaries, source.Binaries) {
		srcPath := toolDir + "/" + src
		dstPath := dstDir + "/" + dst
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		srcLinkPath := filepath.Join(strings.Repeat("../", strings.Count(dst, "/")), "downloads", string(bt.Name)+"-"+bt.Version, src)
		must.OK(os.Symlink(srcLinkPath, dstPath))
		must.Any(filepath.EvalSymlinks(dstPath))
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
func (gpt GoPackageTool) IsCompatible(_ Platform) bool {
	return true
}

// GetBinaries returns binaries defined for the platform.
func (gpt GoPackageTool) GetBinaries(_ Platform) []string {
	return []string{
		"bin/" + filepath.Base(gpt.Package),
	}
}

// Ensure ensures that tool is installed.
func (gpt GoPackageTool) Ensure(ctx context.Context, platform Platform) error {
	binName := filepath.Base(gpt.Package)
	toolDir := toolDir(gpt, platform)
	dst := filepath.Join("bin", binName)
	if shouldReinstall(gpt, platform, binName, dst) {
		cmd := exec.Command(Path("bin/go", PlatformLocal), "install", "-tags=tools", gpt.Package)
		cmd.Dir = "build/tools"
		cmd.Env = append(os.Environ(), "GOBIN="+toolDir)

		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}

		srcPath := toolDir + "/" + binName
		dstPath := BinariesRootPath(platform) + "/" + dst
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		srcLinkPath := filepath.Join("..", "downloads", string(gpt.Name)+"-"+gpt.Version, binName)
		must.OK(os.Symlink(srcLinkPath, dstPath))
		must.Any(filepath.EvalSymlinks(dstPath))
		logger.Get(ctx).Info("Binary installed to path", zap.String("path", dstPath))
	}

	return linkTool(dst)
}

// Source represents source where tool is fetched from.
type Source struct {
	URL      string
	Hash     string
	Binaries map[string]string
}

// Sources is the map of sources.
type Sources map[Platform]Source

// InstallAll installs all the toolsMap.
func InstallAll(ctx context.Context, deps build.DepsFunc) error {
	for toolName := range toolsMap {
		tool := toolsMap[toolName]
		if tool.IsLocal() {
			if err := tool.Ensure(ctx, PlatformLocal); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureBuf ensures that buf is available.
func EnsureBuf(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, Buf, PlatformLocal)
}

// EnsureProtoc ensures that protoc is available.
func EnsureProtoc(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, Protoc, PlatformLocal)
}

// EnsureProtocGenDoc ensures that protoc-gen-doc is available.
func EnsureProtocGenDoc(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenDoc, PlatformLocal)
}

// EnsureProtocGenGRPCGateway ensures that protoc-gen-grpc-gateway is available.
func EnsureProtocGenGRPCGateway(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenGRPCGateway, PlatformLocal)
}

// EnsureProtocGenGoCosmos ensures that protoc-gen-gocosmos is available.
func EnsureProtocGenGoCosmos(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenGoCosmos, PlatformLocal)
}

// EnsureProtocGenOpenAPIV2 ensures that protoc-gen-openapiv2 is available.
func EnsureProtocGenOpenAPIV2(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenOpenAPIV2, PlatformLocal)
}

// EnsureProtocGenBufLint ensures that protoc-gen-buf-lint is available.
func EnsureProtocGenBufLint(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenBufLint, PlatformLocal)
}

// EnsureProtocGenBufBreaking ensures that protoc-gen-buf-breaking is available.
func EnsureProtocGenBufBreaking(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenBufBreaking, PlatformLocal)
}

func linkTool(dst string) error {
	relink, err := shouldRelink(dst)
	if err != nil {
		return err
	}

	if !relink {
		return nil
	}

	src := filepath.Join(BinariesRootPath(PlatformLocal), dst)
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

func toolDir(tool Tool, platform Platform) string {
	return filepath.Join(BinariesRootPath(platform), "downloads", string(tool.GetName())+"-"+tool.GetVersion())
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

func shouldReinstall(t Tool, platform Platform, src, dst string) bool {
	toolDir := toolDir(t, platform)

	srcPath, err := filepath.Abs(toolDir + "/" + src)
	if err != nil {
		return true
	}

	dstPath, err := filepath.Abs(filepath.Join(BinariesRootPath(platform), dst))
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

	return false
}

func shouldRelink(dst string) (bool, error) {
	dstPath := filepath.Join(BinariesRootPath(PlatformLocal), dst)

	absSrcPath, err := filepath.Abs(dstPath)
	if err != nil {
		return false, errors.WithStack(err)
	}
	realSrcPath, err := filepath.EvalSymlinks(absSrcPath)
	if err != nil {
		return false, errors.WithStack(err)
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
func CopyToolBinaries(toolName Name, platform Platform, path string, binaryNames ...string) error {
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

// BinariesRootPath returns the root path of cached binaries.
func BinariesRootPath(platform Platform) string {
	return filepath.Join(CacheDir(), "bin", platform.String())
}

// Path returns path to the installed binary.
func Path(binary string, platform Platform) string {
	return must.String(filepath.Abs(must.String(filepath.EvalSymlinks(filepath.Join(BinariesRootPath(platform), binary)))))
}

// Ensure ensures tool exists for the platform.
func Ensure(ctx context.Context, toolName Name, platform Platform) error {
	tool, err := Get(toolName)
	if err != nil {
		return err
	}
	return tool.Ensure(ctx, platform)
}
