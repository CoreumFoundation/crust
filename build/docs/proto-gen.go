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
	cosmosSdkModule = "github.com/cosmos/cosmos-sdk"
	cosmWasmModule  = "github.com/CosmWasm/wasmd"

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

	//  We need versions to derive paths to protoc for given modules installed by `go mod tidy`
	moduleToVersion, err := getModuleVersions(deps, []string{
		cosmosSdkModule,
		cosmWasmModule,
	})
	if err != nil {
		log.Error("failed to get modules versions", zap.Error(err))
		return err
	}

	protoPathList, err := getProtoDirs(moduleToVersion)
	if err != nil {
		log.Error("failed to get paths to proto dirs", zap.Error(err))
		return err
	}

	err = executeProtocCommand(ctx, deps, protoPathList)
	if err != nil {
		log.Error("failed to execute protoc command", zap.Error(err))
		return err
	}

	return nil
}

// getModuleVersions returns a map[moduleName]version.
func getModuleVersions(deps build.DepsFunc, modules []string) (map[string]string, error) {
	moduleToVersion := make(map[string]string)

	var version string
	var err error
	for _, module := range modules {
		version, err = golang.GetModuleVersion(deps, coreum.RepoPath, module)
		if err != nil {
			return nil, err
		}

		moduleToVersion[module] = version
	}

	return moduleToVersion, nil
}

// getProtoDirs returns a list of absolute path to needed proto directories.
func getProtoDirs(modulesMap map[string]string) ([]string, error) {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(must.String(os.UserHomeDir()), "go")
	}

	absPath, err := filepath.Abs(coreum.RepoPath)
	if err != nil {
		return nil, err
	}

	result := []string{
		filepath.Join(absPath, "proto"),
	}

	for module, version := range modulesMap {
		switch module {
		case cosmosSdkModule:
			// path example: $GOPATH/pkg/mod/github.com/cosmos/cosmos-sdk@v0.45.16/proto.
			result = append(result, filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", module, version), "proto"))
			result = append(result, filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", module, version), "third_party", "proto"))
		case cosmWasmModule:
			result = append(result, filepath.Join(goPath, "pkg", "mod", "github.com", "!cosm!wasm", fmt.Sprintf("wasmd@%s", version), "proto"))
		}
	}

	return result, nil
}

// executeProtocCommand ensures needed dependencies, composes the protoc command and executes it.
func executeProtocCommand(ctx context.Context, deps build.DepsFunc, pathList []string) error {
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

// findAllProtoFiles returns a list of absolute paths to each proto file within the given directories.
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
