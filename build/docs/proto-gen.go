package docs

import (
	"context"
	"fmt"
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
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	cosmosSdkModule  = "github.com/cosmos/cosmos-sdk"
	cosmWasmModule   = "github.com/CosmWasm/wasmd"
	tenderMintModule = "github.com/tendermint/tendermint"
	cometBftModule   = "github.com/cometbft/cometbft"

	protoPathKey    = "--proto_path"
	protoDocsOutKey = "--doc_out"
	protoDOcsOptKey = "--doc_opt"

	protoExtension = ".proto"
)

// Proto collects cosmos-sdk, cosmwasm and tendermint proto files from coreum go.mod,
// generates documentation using above proto files + coreum/proto, and places the result to docs/src/api.md.
func Proto(ctx context.Context, deps build.DepsFunc) error {
	log := logger.Get(ctx)

	deps(coreum.Tidy)

	moduleToVersion, err := getModuleVersions(deps)
	if err != nil {
		log.Error("failed to get modules versions", zap.Error(err))
		return err
	}

	err = executeProtocCommand(ctx, deps, getModulePaths(moduleToVersion))
	if err != nil {
		log.Error("failed to execute protoc command", zap.Error(err))
		return err
	}

	return nil
}

func getModuleVersions(deps build.DepsFunc) (map[string]string, error) {
	var moduleToVersion = map[string]string{
		cosmosSdkModule:  "",
		tenderMintModule: "",
		cosmWasmModule:   "",
	}

	// Get versions for specific modules from coreum/go.mod.
	var version string
	var err error
	for moduleName := range moduleToVersion {
		version, err = golang.GetModuleVersion(deps, coreum.RepoPath, moduleName)
		if err != nil {
			return nil, err
		}

		moduleToVersion[moduleName] = version
	}

	return moduleToVersion, nil
}

// getModulePaths copies third-party proto files to coreum/third_party/proto dir.
func getModulePaths(modulesMap map[string]string) map[string]string {
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

	return modulesMap
}

func executeProtocCommand(ctx context.Context, deps build.DepsFunc, moduleToPath map[string]string) error {
	err := golang.EnsureProtoc(ctx, deps)
	if err != nil {
		return err
	}

	err = golang.EnsureProtocGenDoc(ctx, deps)
	if err != nil {
		return err
	}

	command := []string{
		`protoc`,
	}

	command = append(command, fmt.Sprintf("%s=%s", protoDocsOutKey, "docs"))
	command = append(command, fmt.Sprintf("%s=%s,api.md", protoDOcsOptKey, filepath.Join("docs", "protodoc-markdown.tmpl")))

	pathList, err := collectAllPaths(moduleToPath)
	if err != nil {
		return err
	}

	for _, path := range pathList {
		command = append(command, protoPathKey, fmt.Sprintf("\"%s\"", path))
	}

	allProtoFiles, err := findAllProtoFiles(pathList)
	if err != nil {
		return err
	}

	command = append(command, allProtoFiles...)

	cmd := exec.Command("sh", "-c", strings.Join(command, " "))
	cmd.Dir = coreum.RepoPath

	return libexec.Exec(ctx, cmd)
}

func collectAllPaths(moduleToPath map[string]string) ([]string, error) {
	absPath, err := filepath.Abs(coreum.RepoPath)
	if err != nil {
		return nil, err
	}

	pathList := []string{
		filepath.Join(absPath, "proto"),
		filepath.Join(absPath, "third_party", "proto"),
	}

	for _, path := range moduleToPath {
		pathList = append(pathList, path)
	}

	return pathList, nil
}

func findAllProtoFiles(pathList []string) (finalResult []string, err error) {
	var iterationResult []string
	for _, path := range pathList {
		iterationResult, err = tools.ListFilesByPath(path, protoExtension)
		if err != nil {
			return nil, err
		}
		finalResult = append(finalResult, iterationResult...)
	}

	return finalResult, nil
}
