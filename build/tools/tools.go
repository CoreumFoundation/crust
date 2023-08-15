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
	Go                    Name = "go"
	GolangCI              Name = "golangci"
	Ignite                Name = "ignite"
	Cosmovisor            Name = "cosmovisor"
	Aarch64LinuxMuslCross Name = "aarch64-linux-musl-cross"
	LibWASMMuslC          Name = "libwasmvm_muslc"
	Gaia                  Name = "gaia"
	Osmosis               Name = "osmosis"
	RelayerCosmos         Name = "relayercosmos"
	Hermes                Name = "hermes"
	CoredV100             Name = "cored-v1.0.0"
	CoredV200             Name = "cored-v2.0.0"
	Protoc                Name = "protoc"
	ProtocGenDoc          Name = "protoc-gen-doc"
)

var tools = map[Name]Tool{
	// https://go.dev/dl/
	Go: {
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
	GolangCI: {
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
	Cosmovisor: {
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
	Aarch64LinuxMuslCross: {
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
	LibWASMMuslC: {
		Version: "v1.1.2",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.1.2/libwasmvm_muslc.x86_64.a",
				Hash: "sha256:e0a0955815a23c139d42781f1cc70beffa916aa74fe649e5c69ee7e95ff13b6b",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.x86_64.a",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CosmWasm/wasmvm/releases/download/v1.1.2/libwasmvm_muslc.aarch64.a",
				Hash: "sha256:77b41e65f6c3327d910a7f9284538570727e380ab49bc3c88c8d4053811d5209",
				Binaries: map[string]string{
					"lib/libwasmvm_muslc.a": "libwasmvm_muslc.aarch64.a",
				},
			},
		},
	},

	// https://github.com/cosmos/gaia/releases
	// Before upgrading verify in go.mod that they use the same version of IBC
	Gaia: {
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

	// https://github.com/osmosis-labs/osmosis/releases
	// FIXME [Artem S] make it work with v16 or compile v15 for darwin https://app.clickup.com/t/8685af9am
	Osmosis: {
		Version: "16.1.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				//URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v15.2.0/osmosisd-15.2.0-linux-amd64",
				//Hash: "sha256:3aab2f2668cb5a713d5770e46a777ef01c433753378702d9ae941aa2d1ee5618",
				//Binaries: map[string]string{
				//	"bin/osmosisd": "osmosisd-15.2.0-linux-amd64",
				//},
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-linux-amd64",
				Hash: "sha256:0ec66e32584fff24b6d62fc9938c69ff1a1bbdd8641d2ec9e0fd084aaa767ed3",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				//URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v15.2.0/osmosisd-15.2.0-linux-arm64",
				//Hash: "sha256:e158d30707a0ea51482237f99676223e81ce5a353966a5c83791d2662a930f35",
				//Binaries: map[string]string{
				//	"bin/osmosisd": "osmosisd-15.2.0-linux-arm64",
				//},
				URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v16.1.1/osmosisd-16.1.1-linux-arm64",
				Hash: "sha256:e2ccc743dd66da91d1df1ae4ecf92b36d658575f4ff507d5056eb640804e0401",
				Binaries: map[string]string{
					"bin/osmosisd": "osmosisd-16.1.1-linux-arm64",
				},
			},
			PlatformLinuxAMD64: {
				//URL:  "https://github.com/osmosis-labs/osmosis/releases/download/v15.2.0/osmosisd-15.2.0-linux-amd64",
				//Hash: "sha256:3aab2f2668cb5a713d5770e46a777ef01c433753378702d9ae941aa2d1ee5618",
				//Binaries: map[string]string{
				//	"bin/osmosisd": "osmosisd-15.2.0-linux-amd64",
				//},
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

	// https://github.com/cosmos/relayer/releases
	RelayerCosmos: {
		Version: "v2.3.1",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.3.1/Cosmos.Relayer_2.3.1_linux_amd64.tar.gz",
				Hash: "sha256:68c94403959239683cc80f17e50ca99c7e5caff8d70a17d2171009969d4c2692",
				Binaries: map[string]string{
					"bin/relayercosmos": "Cosmos Relayer_2.3.1_linux_amd64/rly",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/cosmos/relayer/releases/download/v2.3.1/Cosmos.Relayer_2.3.1_linux_arm64.tar.gz",
				Hash: "sha256:5466606e6d1186ce70321a7ae421b7da121308960719caf6cc7c5a4923585230",
				Binaries: map[string]string{
					"bin/relayercosmos": "Cosmos Relayer_2.3.1_linux_arm64/rly",
				},
			},
		},
	},

	// FIXME (wojtek): When using v2.4.0 tests TestIBCTransferFromSmartContract and TestIBCAssetFTSendCommissionAndBurnRate don't pass.

	// RelayerCosmos: {
	//	Version: "v2.4.0",
	//	Sources: Sources{
	//		PlatformDockerAMD64: {
	//			URL:  "https://github.com/cosmos/relayer/releases/download/v2.4.0/Cosmos.Relayer_2.4.0_linux_amd64.tar.gz",
	//			Hash: "sha256:6c7b197656643ecb16dabaae67dfda943c4ccffe46baac4ab606762bf78b9a30",
	//			Binaries: map[string]string{
	//				"bin/relayercosmos": "Cosmos Relayer_2.4.0_linux_amd64/rly",
	//			},
	//		},
	//		PlatformDockerARM64: {
	//			URL:  "https://github.com/cosmos/relayer/releases/download/v2.4.0/Cosmos.Relayer_2.4.0_linux_arm64.tar.gz",
	//			Hash: "sha256:b7c1f4109db5fc8a095cf6a673233adc76103b4fa8e9aae9c79323e6ddb00687",
	//			Binaries: map[string]string{
	//				"bin/relayercosmos": "Cosmos Relayer_2.4.0_linux_arm64/rly",
	//			},
	//		},
	//	},
	// },

	// https://github.com/informalsystems/hermes/releases
	Hermes: {
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
	CoredV200: {
		Version: "v2.0.0",
		Sources: Sources{
			PlatformDockerAMD64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.0/cored-linux-amd64",
				Hash: "sha256:7848022a3a35723ecef02eb835fbf139989aace8d780186018dbcdebdc57d694",
				Binaries: map[string]string{
					"bin/cored-v2.0.0": "cored-linux-amd64",
				},
			},
			PlatformDockerARM64: {
				URL:  "https://github.com/CoreumFoundation/coreum/releases/download/v2.0.0/cored-linux-arm64",
				Hash: "sha256:c082eeebbc206633f1b71ef9c16a7f390f5ea5b27ce06c735ed7a632f38b5891",
				Binaries: map[string]string{
					"bin/cored-v2.0.0": "cored-linux-arm64",
				},
			},
		},
	},

	// https://github.com/protocolbuffers/protobuf/releases
	Protoc: {
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
	ProtocGenDoc: {
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

// EnsureProtoc ensures that protoc is available.
func EnsureProtoc(ctx context.Context, deps build.DepsFunc) error {
	return EnsureTool(ctx, Protoc)
}

// EnsureProtocGenDoc ensures that protoc-gen-doc is available.
func EnsureProtocGenDoc(ctx context.Context, deps build.DepsFunc) error {
	return EnsureTool(ctx, ProtocGenDoc)
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
		srcPath := filepath.Join(BinariesRootPath(PlatformLocal), binaryName)

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
		// todo: IMO in this loop we should also verify that checksum of binary matches & if it doesn't - reinstall it.
		// If I'm not mistaken, crust wasn't downloading new cosmovisor binary when I changed version in tools.
		// I had to remove old one before.

		srcPath, err := filepath.Abs(toolDir + "/" + src)
		if err != nil {
			return installBinary(ctx, tool, info, platform)
		}

		dstPath, err := filepath.Abs(filepath.Join(BinariesRootPath(platform), dst))
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

	dstDir := BinariesRootPath(platform)
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

func toolDir(name Name, platform Platform) string {
	info, exists := tools[name]
	if !exists {
		panic(errors.Errorf("tool %s is not defined", name))
	}

	return filepath.Join(BinariesRootPath(platform), "downloads", string(name)+"-"+info.Version)
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

// BinariesRootPath returns the root path of cached binaries.
func BinariesRootPath(platform Platform) string {
	return filepath.Join(CacheDir(), "bin", platform.String())
}

// Path returns path to the installed binary.
func Path(binary string, platform Platform) string {
	return must.String(filepath.Abs(must.String(filepath.EvalSymlinks(filepath.Join(BinariesRootPath(platform), binary)))))
}
