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
	cosmosSDKModule    = "github.com/cosmos/cosmos-sdk"
	cosmosIBCModule    = "github.com/cosmos/ibc-go/v7"
	cosmosProtoModule  = "github.com/cosmos/cosmos-proto"
	cosmWASMModule     = "github.com/CosmWasm/wasmd"
	gogoProtobufModule = "github.com/cosmos/gogoproto"
	grpcGatewayModule  = "github.com/grpc-ecosystem/grpc-gateway"
)

// generateProtoDocs collects cosmos-sdk, cosmwasm and tendermint proto files from coreum go.mod,
// generates documentation using above proto files + coreum/proto, and places the result to docs/src/api.md.
func generateProtoDocs(ctx context.Context, deps build.DepsFunc) error {
	deps(Tidy)

	//  We need versions to derive paths to protoc for given modules installed by `go mod tidy`
	moduleDirs, err := golang.ModuleDirs(ctx, deps, repoPath,
		cosmosSDKModule,
		cosmosProtoModule,
		cosmWASMModule,
		gogoProtobufModule,
		grpcGatewayModule,
	)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return errors.WithStack(err)
	}

	includeDirs := []string{
		filepath.Join(absPath, "proto"),
		filepath.Join(absPath, "third_party", "proto"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto"),
		filepath.Join(moduleDirs[cosmosProtoModule], "proto"),
		filepath.Join(moduleDirs[cosmWASMModule], "proto"),
		moduleDirs[gogoProtobufModule],
		filepath.Join(moduleDirs[grpcGatewayModule], "third_party", "googleapis"),
	}

	generateDirs := []string{
		filepath.Join(absPath, "proto"),
		// FIXME(v47-generators) we must switch to cosmos sdk nft module before uncommenting this
		// filepath.Join(moduleDirs[cosmosSDKModule], "proto"),
		filepath.Join(moduleDirs[cosmWASMModule], "proto"),
	}

	err = executeProtocCommand(ctx, deps, includeDirs, generateDirs)
	if err != nil {
		return err
	}

	return nil
}

// executeProtocCommand ensures needed dependencies, composes the protoc command and executes it.
func executeProtocCommand(ctx context.Context, deps build.DepsFunc, includeDirs, generateDirs []string) error {
	deps(tools.EnsureProtoc, tools.EnsureProtocGenDoc)

	args := []string{
		fmt.Sprintf("%s=%s", "--doc_out", "docs"),
		fmt.Sprintf("%s=%s,api.md", "--doc_opt", filepath.Join("docs", "api.tmpl.md")),
	}

	for _, path := range includeDirs {
		args = append(args, "--proto_path", path)
	}

	allProtoFiles, err := findAllProtoFiles(generateDirs)
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
			return errors.WithStack(err)
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
