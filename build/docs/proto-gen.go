package docs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/build"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/crust/build/coreum"
	"github.com/CoreumFoundation/crust/build/git"
	"github.com/CoreumFoundation/crust/build/tools"
)

const (
	repoURL = "git@github.com:CoreumFoundation/docs.git"

	cosmosSdkModule  = "github.com/cosmos/cosmos-sdk"
	cosmWasmModule   = "github.com/CosmWasm/wasmd"
	tenderMintModule = "github.com/tendermint/tendermint"
	cometBftModule   = "github.com/cometbft/cometbft"
)

// Proto generates documentation from proto and puts it to docs/src/api.md.
func Proto(ctx context.Context, deps build.DepsFunc) error {
	log := logger.Get(ctx)

	err := coreum.Tidy(ctx, deps)
	if err != nil {
		return err
	}

	moduleToVersion, err := getModulesVersions()
	if err != nil {
		log.Error("failed to get modules versions", zap.Error(err))
		return err
	}

	err = copyProtoFiles(log, moduleToVersion)
	if err != nil {
		log.Error("failed to copy proto files", zap.Error(err))
		return err
	}

	err = git.EnsureRepo(ctx, repoURL)
	if err != nil {
		log.Error("failed to ensure repo", zap.String("repo", repoURL), zap.Error(err))
		return err
	}

	err = executeProtocCommand()
	if err != nil {
		log.Error("failed to execute protoc command", zap.Error(err))
		return err
	}

	return nil
}

func getModulesVersions() (map[string]string, error) {
	var moduleToVersion = map[string]string{
		cosmosSdkModule:  "",
		cosmWasmModule:   "",
		tenderMintModule: "",
	}

	// Go to coreum dir.
	err := os.Chdir("../coreum")
	if err != nil {
		return nil, err
	}

	// Get dependencies from coreum/go.mod.
	var cmd *exec.Cmd
	var output []byte
	for moduleName := range moduleToVersion {
		cmd = exec.Command("go", "list", "-m", moduleName)
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}

		parts := strings.Fields(string(output))
		if len(parts) >= 2 {
			moduleToVersion[moduleName] = parts[1]
		} else {
			return nil, errors.New("no module version")
		}
	}

	return moduleToVersion, nil
}

// copyProtoFiles copies third-party proto files to coreum/third_party/proto dir.
func copyProtoFiles(log *zap.Logger, repos map[string]string) error {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		return errors.New("empty GOPATH")
	}

	var sourceDir, destinationDir string
	for repo, version := range repos {
		switch repo {
		case cosmosSdkModule:
			// example: $GOPATH/pkg/mod/github.com/cosmos/cosmos-sdk@v0.45.16/proto.
			sourceDir = fmt.Sprintf("%s/pkg/mod/%s@%s/proto/cosmos", goPath, repo, version)
			destinationDir = "./third_party/proto/cosmos"
		case cosmWasmModule:
			sourceDir = fmt.Sprintf("%s/pkg/mod/github.com/!cosm!wasm/wasmd@%s/proto/cosmwasm", goPath, version)
			destinationDir = "./third_party/proto/cosmwasm"
		// TODO tune it when we complete migration from tendermint to cometbft
		case tenderMintModule:
			sourceDir = fmt.Sprintf("%s/pkg/mod/%s@%s/proto/tendermint", goPath, cometBftModule, version)
			destinationDir = "./third_party/proto/tendermint"
		}

		err := tools.Dir(sourceDir, destinationDir)
		if err != nil {
			return err
		}

		log.Info("Proto files are copied successfully:", zap.String("module", repo))
	}

	return nil
}

func executeProtocCommand() error {
	cmd := exec.Command("sh", "-c", `
		protoc \
			--proto_path "proto" \
			--proto_path "third_party/proto" \
			--doc_out=../docs/src/api \
			--doc_opt=../docs/proto/protodoc-markdown.tmpl,api.md \
			$(find "proto/coreum" -maxdepth 5 -name '*.proto') \
			$(find "third_party/proto" -maxdepth 5 -name '*.proto')
	`)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
