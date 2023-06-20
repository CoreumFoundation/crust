package docs

import (
	"context"
	"fmt"
	"github.com/CoreumFoundation/crust/build/tools"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/golang"
)

const (
	cosmosSdkModule  = "github.com/cosmos/cosmos-sdk"
	cosmWasmModule   = "github.com/CosmWasm/wasmd"
	tenderMintModule = "github.com/tendermint/tendermint"
	cometBftModule   = "github.com/cometbft/cometbft"

	protoPathKey = "--proto_path"
)

// Proto collects cosmos-sdk, cosmwasm and tendermint proto files from coreum go.mod,
// generates documentation using above proto files + coreum/proto, and places the result to docs/src/api.md.
func Proto(ctx context.Context, deps build.DepsFunc) error {
	log := logger.Get(ctx)

	//err := golang.EnsureProtoc(ctx, deps)
	//if err != nil {
	//	return err
	//}

	deps(coreum.Tidy)

	moduleToVersion, err := getModuleVersions(ctx, deps)
	if err != nil {
		log.Error("failed to get modules versions", zap.Error(err))
		return err
	}

	moduleToPath, err := getModulePaths(moduleToVersion)
	if err != nil {
		log.Error("failed to copy proto files", zap.Error(err))
		return err
	}

	err = executeProtocCommand(ctx, moduleToPath)
	if err != nil {
		log.Error("failed to execute protoc command", zap.Error(err))
		return err
	}

	return nil
}

func getModuleVersions(ctx context.Context, deps build.DepsFunc) (map[string]string, error) {
	var moduleToVersion = map[string]string{
		cosmosSdkModule:  "",
		tenderMintModule: "",
		cosmWasmModule:   "",
	}

	// Get versions for specific modules from coreum/go.mod.
	var version string
	var err error
	for moduleName := range moduleToVersion {
		version, err = golang.GetModuleVersion(ctx, deps, coreum.RepoPath, moduleName)
		if err != nil {
			return nil, err
		}

		moduleToVersion[moduleName] = version
	}

	return moduleToVersion, nil
}

// getModulePaths copies third-party proto files to coreum/third_party/proto dir.
func getModulePaths(modulesMap map[string]string) (map[string]string, error) {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(must.String(os.UserHomeDir()), "go")
	}

	for module, version := range modulesMap {
		switch module {
		case cosmosSdkModule:
			// example: $GOPATH/pkg/mod/github.com/cosmos/cosmos-sdk@v0.45.16/proto.
			modulesMap[module] = filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", module, version), "proto")
		// TODO tune it when we complete migration from tendermint to cometbft
		case tenderMintModule:
			modulesMap[module] = filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", cometBftModule, version), "proto")
		case cosmWasmModule:
			modulesMap[module] = filepath.Join(goPath, "pkg", "mod", "github.com", "!cosm!wasm", fmt.Sprintf("wasmd@%s", version), "proto")
		}
	}

	return modulesMap, nil
}

func executeProtocCommand(ctx context.Context, moduleToPath map[string]string) error {
	command := []string{
		`protoc \
			--doc_out=../docs/src/api \
			--doc_opt=../docs/proto/protodoc-markdown.tmpl,api.md \
			--proto_path "proto" \
			--proto_path "third_party/proto"`,
	}

	//pathList := []string{
	//	filepath.Join("proto", "coreum"),
	//	filepath.Join("third_party", "proto"),
	//}

	for _, path := range moduleToPath {
		command = append(command, protoPathKey, fmt.Sprintf("\"%s\"", path))
		//pathList = append(pathList, path)
	}

	command = append(command, `$(find "proto/coreum" -maxdepth 5 -name '*.proto')`)
	command = append(command, `$(find "third_party/proto" -maxdepth 5 -name '*.proto')`)

	for _, path := range moduleToPath {
		command = append(command, fmt.Sprintf(`$(find "%s" -maxdepth 5 -name '*.proto')`, path))
	}

	//allProtoFiles, err := findAllProtoFiles(pathList)
	//if err != nil {
	//	return err
	//}

	//command = append(command, allProtoFiles...)

	fmt.Println("### ", strings.Join(command, " "))

	cmd := exec.Command("sh", "-c", strings.Join(command, " "))
	cmd.Dir = coreum.RepoPath

	return libexec.Exec(ctx, cmd)
}

func findAllProtoFiles(pathList []string) (finalResult []string, err error) {
	var iterationResult []string
	for _, path := range pathList {
		iterationResult, err = tools.ListFilesByPath(path, ".proto")
		if err != nil {
			return nil, err
		}
		finalResult = append(finalResult, iterationResult...)
	}

	return finalResult, nil
}

func executeProtocCommand2(ctx context.Context, moduleToPath map[string]string) error {
	cmd := exec.Command("sh", "-c", `
		protoc \
			--doc_out=../docs/src/api \
			--doc_opt=../docs/proto/protodoc-markdown.tmpl,api.md \
			--proto_path "proto" \
			--proto_path "third_party/proto" \
      		--proto_path "/Users/vertex451/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.45.16/proto" \
      		--proto_path "/Users/vertex451/go/pkg/mod/github.com/cometbft/cometbft@v0.34.27/proto" \
			$(find "proto/coreum" -maxdepth 5 -name '*.proto') \
			$(find "third_party/proto" -maxdepth 5 -name '*.proto') \
      		$(find "/Users/vertex451/go/pkg/mod/github.com/cosmos/cosmos-sdk@v0.45.16/proto/cosmos" -maxdepth 5 -name '*.proto') \
      		$(find "/Users/vertex451/go/pkg/mod/github.com/cometbft/cometbft@v0.34.27/proto/tendermint" -maxdepth 5 -name '*.proto')
	`)
	cmd.Dir = coreum.RepoPath

	return libexec.Exec(ctx, cmd)
}
