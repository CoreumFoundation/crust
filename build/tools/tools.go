package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	CoredV100             Name = "cored-v1.0.0"
	CoredV202             Name = "cored-v2.0.2"
	CoredV300             Name = "cored-v3.0.0"
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
		Version: "1.21.4",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://go.dev/dl/go1.21.4.linux-amd64.tar.gz",
				Hash: "sha256:73cac0215254d0c7d1241fa40837851f3b9a8a742d0b54714cbdfb3feaf8f0af",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://go.dev/dl/go1.21.4.darwin-amd64.tar.gz",
				Hash: "sha256:cd3bdcc802b759b70e8418bc7afbc4a65ca73a3fe576060af9fc8a2a5e71c3b8",
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://go.dev/dl/go1.21.4.darwin-arm64.tar.gz",
				Hash: "sha256:8b7caf2ac60bdff457dba7d4ff2a01def889592b834453431ae3caecf884f6a5",
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
		Version: "1.55.2",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.55.2/golangci-lint-1.55.2-linux-amd64.tar.gz",
				Hash: "sha256:ca21c961a33be3bc15e4292dc40c98c8dcc5463a7b6768a3afc123761630c09c",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.55.2-linux-amd64/golangci-lint",
				},
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.55.2/golangci-lint-1.55.2-darwin-amd64.tar.gz",
				Hash: "sha256:632e96e6d5294fbbe7b2c410a49c8fa01c60712a0af85a567de85bcc1623ea21",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.55.2-darwin-amd64/golangci-lint",
				},
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/golangci/golangci-lint/releases/download/v1.55.2/golangci-lint-1.55.2-darwin-arm64.tar.gz",
				Hash: "sha256:234463f059249f82045824afdcdd5db5682d0593052f58f6a3039a0a1c3899f6",
				Binaries: map[string]string{
					"bin/golangci-lint": "golangci-lint-1.55.2-darwin-arm64/golangci-lint",
				},
			},
		},
	},

	// https://github.com/cosmos/cosmos-sdk/releases
	BinaryTool{
		Name:    Cosmovisor,
		Version: "1.5.0",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/cosmos/cosmos-sdk/releases/download/cosmovisor%2Fv1.5.0/cosmovisor-v1.5.0-linux-amd64.tar.gz",
				Hash: "sha256:7f4bebfb18a170bff1c725f13dda326e0158132deef9f037ab0c2a48727c3077",
			},
			TargetPlatformLinuxARM64InDocker: {
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
			TargetPlatformLinuxAMD64InDocker: {
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
		Version: "v1.5.0",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.5.0/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:465e3a088e96fd009a11bfd234c69fb8a0556967677e54511c084f815cf9ce63",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.5.0/libwasmvm_muslc.aarch64.a",
				Hash: "sha256:2687afbdae1bc6c7c8b05ae20dfb8ffc7ddc5b4e056697d0f37853dfe294e913",
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
		Version: "v13.0.2",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v13.0.2/gaiad-v13.0.2-linux-amd64",
				Hash: "sha256:729a55b29857fedfe1271f26e1fdf2cb12d2c6515c2ad0d9bbe432a81ae43df8",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v13.0.2-linux-amd64",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v13.0.2/gaiad-v13.0.2-linux-arm64",
				Hash: "sha256:9bfbe0d5212fa3cdabe34b75b42e1420f50fe8aff64ec9247dcbc5942969e333",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v13.0.2-linux-arm64",
				},
			},
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v13.0.2/gaiad-v13.0.2-linux-amd64",
				Hash: "sha256:729a55b29857fedfe1271f26e1fdf2cb12d2c6515c2ad0d9bbe432a81ae43df8",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v13.0.2-linux-amd64",
				},
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v13.0.2/gaiad-v13.0.2-darwin-amd64",
				Hash: "sha256:910b515369b0cfa8eecc54f2f930fd5de2634b7106825b62f4c4c563fe2a6a07",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v13.0.2-darwin-amd64",
				},
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/cosmos/gaia/releases/download/v13.0.2/gaiad-v13.0.2-darwin-arm64",
				Hash: "sha256:5a148b56bec7d9ef23d21778725eedb9bc70eaa6a61155b22947216812a17369",
				Binaries: map[string]string{
					"bin/gaiad": "gaiad-v13.0.2-darwin-arm64",
				},
			},
		},
	},

	// https://github.com/osmosis-labs/osmosis/releases
	BinaryTool{
		Name:    Osmosis,
		Version: "20.2.1",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v20.2.1/osmosisd-20.2.1-linux-amd64",
				Hash: "sha256:4e60a870861ca17819fbcb49fff981b5731ec1121d7cbab43987c5f04ff099fa",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-20.2.1-linux-amd64",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v20.2.1/osmosisd-20.2.1-linux-arm64",
				Hash: "sha256:4e7fe2cc369a9eef28a8083414c2d7e0a8cb5eb5b75e913ded06ee457dff62bb",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-20.2.1-linux-arm64",
				},
			},
		},
	},

	// https://github.com/informalsystems/hermes/releases
	BinaryTool{
		Name:    Hermes,
		Version: "v1.7.1",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.7.1/hermes-v1.7.1-x86_64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:10eff785eeb59f730eaa452ebc89236a6ae6aa30be47fcac95cb89682a77decc",
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/informalsystems/hermes/releases/download/v1.7.1/hermes-v1.7.1-aarch64-unknown-linux-gnu.tar.gz",
				Hash: "sha256:ecbac11a1ea48ae587ddd4a7abc6a464d706fc1e2efd2f5e1441c72c7570e46a",
			},
		},
		Binaries: map[string]string{
			"bin/hermes": "hermes",
		},
	},

	// https://github.com/CoreumFoundation/coreum/releases
	BinaryTool{
		Name:    CoredV100,
		Version: "v1.0.0",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v1.0.0/cored-linux-amd64",
				Hash: "sha256:34098ad7586bda364b1b2e7c4569cbcefb630cd4ed7c8f68eb5bced834082c57",
				Binaries: map[string]string{
					"bin/cored-v1.0.0": "cored-linux-amd64",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v1.0.0/cored-linux-arm64",
				Hash: "sha256:3ced97f06607f0cdaf77e7ff0b36b2011d101c660684e4f3e54c2ac6bf344dd6",
				Binaries: map[string]string{
					"bin/cored-v1.0.0": "cored-linux-arm64",
				},
			},
		},
	},
	BinaryTool{
		Name:    CoredV202,
		Version: "v2.0.2",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-amd64",
				Hash: "sha256:3facf55f7ff795719f68b9bcf76ea08262bc7c9e9cd735c660257ba73678250e",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-amd64",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.2/cored-linux-arm64",
				Hash: "sha256:35e261eb3b87c833c30174e6b8667a6155f5962441275d443157e209bbb0bf0d",
				Binaries: map[string]string{
					"bin/cored-v2.0.2": "cored-linux-arm64",
				},
			},
		},
	},

	BinaryTool{
		Name:    CoredV300,
		Version: "v3.0.0",
		Sources: Sources{
			TargetPlatformLinuxAMD64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v3.0.0/cored-linux-amd64",
				Hash: "sha256:0198b5bf7a32b0e76c57f5fe30bcb27bdc69ba91cd6e4fc8c416bea3a1251a24",
				Binaries: map[string]string{
					"bin/cored-v3.0.0": "cored-linux-amd64",
				},
			},
			TargetPlatformLinuxARM64InDocker: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v3.0.0/cored-linux-arm64",
				Hash: "sha256:192eaf4fcc49c3ea3339d12f53c905420575d00d605048e526a994a0c4368768",
				Binaries: map[string]string{
					"bin/cored-v3.0.0": "cored-linux-arm64",
				},
			},
		},
	},

	// https://github.com/bufbuild/buf/releases
	BinaryTool{
		Name:    Buf,
		Version: "v1.28.0",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.28.0/buf-Linux-x86_64",
				Hash: "sha256:97dc21ba30be34e2d4d11ee5fa4454453f635c8f5476bfe4cbca58420eb20299",
				Binaries: map[string]string{
					"bin/buf": "buf-Linux-x86_64",
				},
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.28.0/buf-Darwin-x86_64",
				Hash: "sha256:577fd9fe2e38693b690c88837f70503640897763376195404651f7071493a21a",
				Binaries: map[string]string{
					"bin/buf": "buf-Darwin-x86_64",
				},
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/bufbuild/buf/releases/download/v1.28.0/buf-Darwin-arm64",
				Hash: "sha256:8e51a9c3e09def469969002c15245cfbf1e7d8f878ddc5205125b8107a22cfbf",
				Binaries: map[string]string{
					"bin/buf": "buf-Darwin-arm64",
				},
			},
		},
	},

	// https://github.com/protocolbuffers/protobuf/releases
	BinaryTool{
		Name:    Protoc,
		Version: "v25.0",
		Local:   true,
		Sources: Sources{
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v25.0/protoc-25.0-linux-x86_64.zip",
				Hash: "sha256:d26c4efe0eae3066bb560625b33b8fc427f55bd35b16f246b7932dc851554e67",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v25.0/protoc-25.0-osx-x86_64.zip",
				Hash: "sha256:15eefb30ba913e8dc4dd21d2ccb34ce04a2b33124f7d9460e5fd815a5d6459e3",
			},
			TargetPlatformDarwinARM64: {
				URL:  "https://github.com/protocolbuffers/protobuf/releases/download/v25.0/protoc-25.0-osx-aarch_64.zip",
				Hash: "sha256:76a997df5dacc0608e880a8e9069acaec961828a47bde16c06116ed2e570588b",
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
			TargetPlatformLinuxAMD64: {
				URL:  "https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.5.1/protoc-gen-doc_1.5.1_linux_amd64.tar.gz",
				Hash: "sha256:47cd72b07e6dab3408d686a65d37d3a6ab616da7d8b564b2bd2a2963a72b72fd",
			},
			TargetPlatformDarwinAMD64: {
				URL:  "https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.5.1/protoc-gen-doc_1.5.1_darwin_amd64.tar.gz",
				Hash: "sha256:f429e5a5ddd886bfb68265f2f92c1c6a509780b7adcaf7a8b3be943f28e144ba",
			},
			TargetPlatformDarwinARM64: {
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

// Name is the type used for defining tool names.
type Name string

var toolsMap = func(tools []Tool) map[Name]Tool {
	res := make(map[Name]Tool, len(tools))
	for _, tool := range tools {
		res[tool.GetName()] = tool
	}
	return res
}(tools)

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

	for dst := range lo.Assign(bt.Binaries, source.Binaries) {
		if bt.Local {
			if err := linkTool(dst); err != nil {
				return err
			}
		}
	}

	return nil
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

	dstDir := BinariesRootPath(platform)
	for dst, src := range lo.Assign(bt.Binaries, source.Binaries) {
		srcPath := toolDir + "/" + src

		binChecksum, err := checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := dstDir + "/" + dst
		dstPathChecksum := dstPath + ":" + binChecksum
		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		srcLinkPath := filepath.Join(strings.Repeat("../", strings.Count(dst, "/")), "downloads", string(bt.Name)+"-"+bt.Version, src)
		must.OK(os.Symlink(srcLinkPath, dstPathChecksum))
		must.OK(os.Symlink(filepath.Base(dstPathChecksum), dstPath))
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
	if shouldReinstall(gpt, platform, binName, dst) {
		cmd := exec.Command(Path("bin/go", TargetPlatformLocal), "install", "-tags=tools", gpt.Package)
		cmd.Dir = "build/tools"
		cmd.Env = append(os.Environ(), "GOBIN="+toolDir)

		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}

		srcPath := toolDir + "/" + binName

		binChecksum, err := checksum(srcPath)
		if err != nil {
			return err
		}

		dstPath := BinariesRootPath(platform) + "/" + dst
		dstPathChecksum := dstPath + ":" + binChecksum

		if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
			panic(err)
		}
		if err := os.Remove(dstPathChecksum); err != nil && !os.IsNotExist(err) {
			return errors.WithStack(err)
		}

		must.OK(os.MkdirAll(filepath.Dir(dstPath), 0o700))
		must.OK(os.Chmod(srcPath, 0o700))
		srcLinkPath := filepath.Join("..", "downloads", string(gpt.Name)+"-"+gpt.Version, binName)
		must.OK(os.Symlink(srcLinkPath, dstPathChecksum))
		must.OK(os.Symlink(filepath.Base(dstPathChecksum), dstPath))
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
type Sources map[TargetPlatform]Source

// InstallAll installs all the toolsMap.
func InstallAll(ctx context.Context, deps build.DepsFunc) error {
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

// EnsureBuf ensures that buf is available.
func EnsureBuf(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, Buf, TargetPlatformLocal)
}

// EnsureProtoc ensures that protoc is available.
func EnsureProtoc(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, Protoc, TargetPlatformLocal)
}

// EnsureProtocGenDoc ensures that protoc-gen-doc is available.
func EnsureProtocGenDoc(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenDoc, TargetPlatformLocal)
}

// EnsureProtocGenGRPCGateway ensures that protoc-gen-grpc-gateway is available.
func EnsureProtocGenGRPCGateway(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenGRPCGateway, TargetPlatformLocal)
}

// EnsureProtocGenGoCosmos ensures that protoc-gen-gocosmos is available.
func EnsureProtocGenGoCosmos(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenGoCosmos, TargetPlatformLocal)
}

// EnsureProtocGenOpenAPIV2 ensures that protoc-gen-openapiv2 is available.
func EnsureProtocGenOpenAPIV2(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenOpenAPIV2, TargetPlatformLocal)
}

// EnsureProtocGenBufLint ensures that protoc-gen-buf-lint is available.
func EnsureProtocGenBufLint(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenBufLint, TargetPlatformLocal)
}

// EnsureProtocGenBufBreaking ensures that protoc-gen-buf-breaking is available.
func EnsureProtocGenBufBreaking(ctx context.Context, deps build.DepsFunc) error {
	return Ensure(ctx, ProtocGenBufBreaking, TargetPlatformLocal)
}

func linkTool(dst string) error {
	relink, err := shouldRelink(dst)
	if err != nil {
		return err
	}

	if !relink {
		return nil
	}

	src := filepath.Join(BinariesRootPath(TargetPlatformLocal), dst)
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

func toolDir(tool Tool, platform TargetPlatform) string {
	return filepath.Join(BinariesRootPath(platform), "downloads", string(tool.GetName())+"-"+tool.GetVersion())
}

func ensureDir(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); !os.IsExist(err) {
		return errors.WithStack(err)
	}
	return nil
}

func shouldReinstall(t Tool, platform TargetPlatform, src, dst string) bool {
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

func shouldRelink(dst string) (bool, error) {
	dstPath := filepath.Join(BinariesRootPath(TargetPlatformLocal), dst)

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

// BinariesRootPath returns the root path of cached binaries.
func BinariesRootPath(platform TargetPlatform) string {
	return filepath.Join(CacheDir(), "bin", platform.String())
}

// Path returns path to the installed binary.
func Path(binary string, platform TargetPlatform) string {
	return must.String(filepath.Abs(must.String(filepath.EvalSymlinks(filepath.Join(BinariesRootPath(platform), binary)))))
}

// Ensure ensures tool exists for the platform.
func Ensure(ctx context.Context, toolName Name, platform TargetPlatform) error {
	tool, err := Get(toolName)
	if err != nil {
		return err
	}
	return tool.Ensure(ctx, platform)
}
