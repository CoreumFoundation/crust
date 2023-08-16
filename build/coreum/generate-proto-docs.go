package coreum

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	cosmosSdkModule = "github.com/cosmos/cosmos-sdk"
	cosmWasmModule  = "github.com/CosmWasm/wasmd"
)

// generateProtoDocs collects cosmos-sdk, cosmwasm and tendermint proto files from coreum go.mod,
// generates documentation using above proto files + coreum/proto, and places the result to docs/src/api.md.
func generateProtoDocs(ctx context.Context, deps build.DepsFunc) error {
	deps(Tidy)

	//  We need versions to derive paths to protoc for given modules installed by `go mod tidy`
	moduleDirs, err := golang.ModuleDirs(ctx, deps, repoPath, cosmosSdkModule, cosmWasmModule)
	if err != nil {
		return err
	}

	protoPathList, err := getProtoDirs(moduleDirs)
	if err != nil {
		return err
	}

	err = executeProtocCommand(ctx, deps, protoPathList)
	if err != nil {
		return err
	}

	return nil
}

// getProtoDirs returns a list of absolute paths to needed proto directories.
func getProtoDirs(moduleDirs []string) ([]string, error) {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	result := []string{}

	// This is the list of subdirectories scanned in each module for proto files.
	protoDirs := []string{
		"proto",
		"third_party/proto",
	}

	// In this loop all the proto directories from requested modules are collected.
	for _, modDir := range moduleDirs {
		// Iterate over defined well-known proto dirs. If dir exists, add it to the results.
		for _, dir := range protoDirs {
			dir = filepath.Join(modDir, dir)
			info, err := os.Stat(dir)
			switch {
			case err == nil:
				result = append(result, dir)
			case errors.Is(err, os.ErrNotExist) || !info.IsDir():
				continue
			default:
				return nil, errors.WithStack(err)
			}
		}
	}

	result = append(result, filepath.Join(absPath, "proto"))

	return result, nil
}

// executeProtocCommand ensures needed dependencies, composes the protoc command and executes it.
func executeProtocCommand(ctx context.Context, deps build.DepsFunc, pathList []string) error {
	deps(tools.EnsureProtoc, tools.EnsureProtocGenDoc)

	args := []string{
		fmt.Sprintf("%s=%s", "--doc_out", "docs"),
		fmt.Sprintf("%s=%s,api.md", "--doc_opt", filepath.Join("docs", "api.tmpl.md")),
	}

	for _, path := range pathList {
		args = append(args, "--proto_path", path)
	}

	allProtoFiles, err := findAllProtoFiles(pathList)
	if err != nil {
		return err
	}
	args = append(args, allProtoFiles...)

	cmd := exec.Command(tools.Path("bin/protoc", tools.PlatformLocal), args...)
	cmd.Dir = repoPath

	return libexec.Exec(ctx, cmd)
}

// findAllProtoFiles returns a list of absolute paths to each proto file within the given directories.
func findAllProtoFiles(pathList []string) (finalResult []string, err error) {
	var iterationResult []string
	for _, path := range pathList {
		iterationResult, err = listFilesByPath(path, ".proto")
		if err != nil {
			return nil, err
		}
		finalResult = append(finalResult, iterationResult...)
	}

	return finalResult, nil
}

// listFilesByPath returns the array of files with the specific extension within the given path.
func listFilesByPath(path, extension string) (fileList []string, err error) {
	err = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, extension) {
			fileList = append(fileList, path)
		}

		return nil
	})

	return fileList, err
}
