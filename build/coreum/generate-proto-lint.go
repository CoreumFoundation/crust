package coreum

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/tools"
)

func lintProto(ctx context.Context, deps build.DepsFunc) error {
	deps(Tidy)

	//  We need versions to derive paths to protoc for given modules installed by `go mod tidy`
	moduleDirs, err := moduleDirectories(ctx, deps)
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
		moduleDirs[gogoProtobufModule],
		filepath.Join(moduleDirs[grpcGatewayModule], "third_party", "googleapis"),
	}

	generateDirs := []string{
		filepath.Join(absPath, "proto"),
	}

	err = executeLintProtocCommand(ctx, deps, includeDirs, generateDirs)
	if err != nil {
		return err
	}

	return nil
}

func executeLintProtocCommand(ctx context.Context, deps build.DepsFunc, includeDirs, generateDirs []string) error {
	deps(tools.EnsureProtoc, tools.EnsureProtocGenBufLint)

	// Linting rule descriptions might be found here: https://buf.build/docs/lint/rules

	args := []string{
		"--buf-lint_out=.",
		`--buf-lint_opt={
			"input_config": {
				"version": "v1",
				"lint": {
					"use": [
						"BASIC"
					],
					"except": [
						"ENUM_VALUE_UPPER_SNAKE_CASE",
						"FIELD_LOWER_SNAKE_CASE"
					]
				}
			},
			"error_format": "json"
		}`,
		"--plugin", must.String(filepath.Abs("bin/protoc-gen-buf-lint")),
	}

	for _, path := range includeDirs {
		args = append(args, "--proto_path", path)
	}

	allProtoFiles, err := findAllProtoFiles(generateDirs)
	if err != nil {
		return err
	}
	packages := map[string][]string{}
	for _, pf := range allProtoFiles {
		pkg, err := goPackage(pf)
		if err != nil {
			return err
		}
		packages[pkg] = append(packages[pkg], pf)
	}

	for _, files := range packages {
		args := append([]string{}, args...)
		args = append(args, files...)
		cmd := exec.Command(tools.Path("bin/protoc", tools.PlatformLocal), args...)
		if err := libexec.Exec(ctx, cmd); err != nil {
			return err
		}
	}

	return nil
}
