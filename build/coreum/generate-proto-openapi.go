package coreum

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/golang"
	"github.com/CoreumFoundation/crust/build/tools"
)

type swaggerDoc struct {
	Swagger     string                                           `json:"swagger"`
	Info        swaggerInfo                                      `json:"info"`
	Consumes    []string                                         `json:"consumes"`
	Produces    []string                                         `json:"produces"`
	Paths       map[string]map[string]map[string]json.RawMessage `json:"paths"`
	Definitions map[string]json.RawMessage                       `json:"definitions"`
}

type swaggerInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

func generateProtoOpenAPI(ctx context.Context, deps build.DepsFunc) error {
	deps(Tidy)

	//  We need versions to derive paths to protoc for given modules installed by `go mod tidy`
	moduleDirs, err := golang.ModuleDirs(ctx, deps, repoPath,
		cosmosSDKModule,
		cosmosIBCModule,
		cosmWASMModule,
		cosmosProtoModule,
		gogoProtobufModule,
		grpcGatewayModule,
	)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(filepath.Join(repoPath, "proto"))
	if err != nil {
		return errors.WithStack(err)
	}

	includeDirs := []string{
		absPath,
		filepath.Join(moduleDirs[cosmosSDKModule], "proto"),
		filepath.Join(moduleDirs[cosmosIBCModule], "proto"),
		filepath.Join(moduleDirs[cosmWASMModule], "proto"),
		filepath.Join(moduleDirs[cosmosProtoModule], "proto"),
		moduleDirs[gogoProtobufModule],
		filepath.Join(moduleDirs[grpcGatewayModule], "third_party", "googleapis"),
	}

	generateDirs := []string{
		filepath.Join(absPath, "coreum", "asset", "ft", "v1"),
		filepath.Join(absPath, "coreum", "asset", "nft", "v1"),
		filepath.Join(absPath, "coreum", "customparams", "v1"),
		filepath.Join(absPath, "coreum", "feemodel", "v1"),
		filepath.Join(absPath, "coreum", "nft", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "auth", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "authz", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "bank", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "consensus", "v1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "distribution", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "evidence", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "feegrant", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "gov", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "gov", "v1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "mint", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "slashing", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "staking", "v1beta1"),
		filepath.Join(moduleDirs[cosmosSDKModule], "proto", "cosmos", "upgrade", "v1beta1"),
		filepath.Join(moduleDirs[cosmosIBCModule], "proto", "ibc", "core", "channel", "v1"),
		filepath.Join(moduleDirs[cosmosIBCModule], "proto", "ibc", "core", "client", "v1"),
		filepath.Join(moduleDirs[cosmosIBCModule], "proto", "ibc", "core", "connection", "v1"),
		filepath.Join(moduleDirs[cosmWASMModule], "proto", "cosmwasm", "wasm", "v1"),
	}

	err = executeOpenAPIProtocCommand(ctx, deps, includeDirs, generateDirs)
	if err != nil {
		return err
	}

	return nil
}

// executeGoProtocCommand generates go code from proto files.
func executeOpenAPIProtocCommand(ctx context.Context, deps build.DepsFunc, includeDirs, generateDirs []string) error {
	deps(tools.EnsureProtoc, tools.EnsureProtocGenGRPCGateway, tools.EnsureProtocGenGoCosmos)

	outDir, err := os.MkdirTemp("", "")
	if err != nil {
		return errors.WithStack(err)
	}

	defer os.RemoveAll(outDir) //nolint:errcheck // we don't care

	args := []string{
		"--openapiv2_out=logtostderr=true,allow_merge=true,json_names_for_fields=false,fqn_for_openapi_name=true,simple_operation_ids=true,Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:.",
		"--plugin", must.String(filepath.Abs("bin/protoc-gen-openapiv2")),
	}

	for _, path := range includeDirs {
		args = append(args, "--proto_path", path)
	}

	finalDoc := swaggerDoc{
		Swagger: "2.0",
		Info: swaggerInfo{
			Title:   "title goes here",
			Version: "version goes here",
		},
		Consumes:    []string{"application/json"},
		Produces:    []string{"application/json"},
		Paths:       map[string]map[string]map[string]json.RawMessage{},
		Definitions: map[string]json.RawMessage{},
	}

	for _, dir := range generateDirs {
		pf := filepath.Join(dir, "query.proto")
		pkg, err := goPackage(pf)
		if err != nil {
			return err
		}

		dir := filepath.Join(outDir, pkg)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		args := append([]string{}, args...)
		args = append(args, pf)
		cmd := exec.Command(tools.Path("bin/protoc", tools.PlatformLocal), args...)
		cmd.Dir = dir
		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}

		err = func() error {
			var sd swaggerDoc
			f, err := os.Open(filepath.Join(dir, "apidocs.swagger.json"))
			if err != nil {
				return errors.WithStack(err)
			}
			defer f.Close()

			if err := json.NewDecoder(f).Decode(&sd); err != nil {
				return errors.WithStack(err)
			}

			for k, v := range sd.Paths {
				for opK, opV := range v {
					var opID string
					if err := json.Unmarshal(opV["operationId"], &opID); err != nil {
						return errors.WithStack(err)
					}
					v[opK]["operationId"] = json.RawMessage(fmt.Sprintf(`"%s%s"`, strcase.ToCamel(strings.ReplaceAll(pkg, "/", ".")), opID))
				}
				finalDoc.Paths[k] = v
			}
			for k, v := range sd.Definitions {
				finalDoc.Definitions[k] = v
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	f, err := os.OpenFile(filepath.Join(repoPath, "docs", "static", "openapi.json"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.WithStack(err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return errors.WithStack(encoder.Encode(finalDoc))
}