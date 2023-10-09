package coreum

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

func breakingProto(ctx context.Context, deps build.DepsFunc) error {
	deps(Tidy, tools.EnsureProtoc, tools.EnsureProtocGenBufBreaking)

	masterDir, err := os.MkdirTemp("", "coreum-master-*")
	if err != nil {
		return errors.WithStack(err)
	}
	defer os.RemoveAll(masterDir) //nolint:errcheck // error doesn't matter

	if err := git.Clone(ctx, masterDir, repoPath, "master"); err != nil {
		return err
	}

	_, masterIncludeDirs, err := protoCDirectories(ctx, masterDir, deps)
	if err != nil {
		return err
	}

	masterIncludeArgs := make([]string, 0, 2*len(masterIncludeDirs))
	for _, path := range masterIncludeDirs {
		masterIncludeArgs = append(masterIncludeArgs, "--proto_path", path)
	}

	imageFile := filepath.Join(os.TempDir(), "coreum.binpb")
	if err := os.MkdirAll(filepath.Dir(imageFile), 0o700); err != nil {
		return err
	}
	defer os.Remove(imageFile) //nolint:errcheck // error doesn't matter

	masterProtoFiles, err := findAllProtoFiles([]string{filepath.Join(masterDir, "proto")})
	if err != nil {
		return err
	}

	cmdImage := exec.Command(tools.Path("bin/protoc", tools.PlatformLocal),
		append(append([]string{"--include_imports", "--include_source_info", "-o", imageFile}, masterIncludeArgs...), masterProtoFiles...)...)

	if err := libexec.Exec(ctx, cmdImage); err != nil {
		return err
	}

	_, includeDirs, err := protoCDirectories(ctx, repoPath, deps)
	if err != nil {
		return err
	}

	includeArgs := make([]string, 0, 2*len(includeDirs))
	for _, path := range includeDirs {
		includeArgs = append(includeArgs, "--proto_path", path)
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return err
	}

	masterProtoFiles, err = findAllProtoFiles([]string{filepath.Join(absRepoPath, "proto")})
	if err != nil {
		return err
	}

	args := []string{
		"--buf-breaking_out=.",
		fmt.Sprintf(`--buf-breaking_opt={
			"against_input": "%s",
			"limit_to_input_files": true,
			"input_config": {"version": "v1", "breaking": {"use": ["FILE"]}},
			"against_input_config": {"version": "v1", "breaking": {"use": ["FILE"]}},
			"log_level": "debug"
		}`, imageFile),
		"--plugin", must.String(filepath.Abs("bin/protoc-gen-buf-breaking")),
	}

	args = append(args, includeArgs...)
	args = append(args, masterProtoFiles...)
	cmdBreaking := exec.Command(tools.Path("bin/protoc", tools.PlatformLocal), args...)
	return libexec.Exec(ctx, cmdBreaking)
}
