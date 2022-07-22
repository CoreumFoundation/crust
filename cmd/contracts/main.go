package main

import (
	"context"
	"os"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/crust/cmd"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/cored/keyring"
	"github.com/CoreumFoundation/crust/pkg/contracts"
)

const (
	defaultTemplateRepo    = "https://github.com/CoreumFoundation/smartcontract-template.git"
	defaultTemplateVersion = "1.0"
)

type RunE func(cmd *cobra.Command, args []string) error

func main() {
	cmd.SetAccountPrefixes("core")
	run.Tool("contracts", nil, func(ctx context.Context) error {
		rootCmd := &cobra.Command{
			SilenceUsage:  true,
			SilenceErrors: true,
			Short:         "Tools for the WASM smart-contracts development on Coreum",
			Args:          cobra.NoArgs,
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: true,
			},
			RunE: contractsRootRunE,
		}
		logger.AddFlags(logger.ToolDefaultConfig, rootCmd.PersistentFlags())

		var initConfig contracts.InitConfig
		initCmd := &cobra.Command{
			Use:     "init <target-dir>",
			Aliases: []string{"i", "gen", "generate"},
			Short:   "Initializes a new WASM smart-contract project by cloning the remote template into target dir",
			Args:    cobra.ExactValidArgs(1),
			RunE:    initRunE(ctx, &initConfig),
		}
		addInitFlags(initCmd, &initConfig)
		rootCmd.AddCommand(initCmd)

		var testConfig contracts.TestConfig
		testCmd := &cobra.Command{
			Use:     "test [workspace-dir]",
			Aliases: []string{"t", "unit-test"},
			Short:   "Runs unit-test suite in the workspace dir. By default current dir is used.",
			Args:    cobra.MaximumNArgs(1),
			RunE:    testRunE(ctx, &testConfig),
		}
		addTestFlags(testCmd, &testConfig)
		rootCmd.AddCommand(testCmd)

		var buildConfig contracts.BuildConfig
		buildCmd := &cobra.Command{
			Use:     "build [workspace-dir]",
			Aliases: []string{"b"},
			Short:   "Builds the WASM contract located in the workspace dir. By default current dir is used.",
			Args:    cobra.MaximumNArgs(1),
			RunE:    buildRunE(ctx, &buildConfig),
		}
		addBuildFlags(buildCmd, &buildConfig)
		rootCmd.AddCommand(buildCmd)

		var deployConfig contracts.DeployConfig
		var keyringParams KeyringParams
		deployCmd := &cobra.Command{
			Use:     "deploy [wasm-artefact] [workspace-dir]",
			Aliases: []string{"d"},
			Short:   "Deploys a WASM artefact specified by path, or builds it on the fly using the workspace dir. By default current dir is used.",
			Args:    cobra.MaximumNArgs(2),
			RunE:    deployRunE(ctx, &deployConfig, &keyringParams),
		}
		addDeployFlags(deployCmd, &deployConfig)
		addDeployKeyringFlags(deployCmd, &keyringParams)
		rootCmd.AddCommand(deployCmd)

		return rootCmd.Execute()
	})
}

var contractsRootRunE RunE = func(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		cmd.Help()
		os.Exit(0)
	}

	return nil
}

func initRunE(ctx context.Context, initConfig *contracts.InitConfig) RunE {
	return func(cmd *cobra.Command, args []string) error {
		initConfig.TargetDir = args[0]
		return contracts.Init(ctx, *initConfig)
	}
}

func testRunE(ctx context.Context, testConfig *contracts.TestConfig) RunE {
	return func(cmd *cobra.Command, args []string) error {
		var targetDir string
		if len(args) > 0 {
			targetDir = args[0]
		} else {
			cwd, err := os.Getwd()
			must.OK(err)
			targetDir = cwd
		}

		return contracts.Test(ctx, targetDir, *testConfig)
	}
}

func buildRunE(ctx context.Context, buildConfig *contracts.BuildConfig) RunE {
	return func(cmd *cobra.Command, args []string) error {
		var targetDir string
		if len(args) > 0 {
			targetDir = args[0]
		} else {
			cwd, err := os.Getwd()
			must.OK(err)
			targetDir = cwd
		}

		return contracts.Build(ctx, targetDir, *buildConfig)
	}
}

func deployRunE(
	ctx context.Context,
	deployConfig *contracts.DeployConfig,
	keyringParams *KeyringParams,
) RunE {
	return func(cmd *cobra.Command, args []string) error {
		switch len(args) {
		case 2:
			if isDir(args[0]) {
				deployConfig.WorkspaceDir = args[0]
			} else if isFile(args[0]) {
				deployConfig.ArtefactPath = args[0]
			} else {
				err := errors.Errorf("path specified but not found: %s", args[0])
				return err
			}

			if isDir(args[1]) {
				if len(deployConfig.WorkspaceDir) > 0 {
					err := errors.Errorf("both paths specified are dirs, need only one")
					return err
				}

				deployConfig.WorkspaceDir = args[1]
			} else if isFile(args[1]) {
				if len(deployConfig.ArtefactPath) > 0 {
					err := errors.Errorf("both paths specified are files, need only one")
					return err
				}

				deployConfig.ArtefactPath = args[1]
			} else {
				err := errors.Errorf("path specified but not found: %s", args[1])
				return err
			}
		case 1:
			if isDir(args[0]) {
				deployConfig.WorkspaceDir = args[0]
			} else if isFile(args[0]) {
				deployConfig.ArtefactPath = args[0]
			} else {
				err := errors.Errorf("path specified but not found: %s", args[0])
				return err
			}
		case 0:
			cwd, err := os.Getwd()
			must.OK(err)
			deployConfig.WorkspaceDir = cwd
		}

		opts := []keyring.ConfigOpt{}
		if len(keyringParams.KeyringDir) > 0 {
			opts = append(opts, keyring.WithKeyringDir(keyringParams.KeyringDir))
		}
		if len(keyringParams.KeyringAppName) > 0 {
			opts = append(opts, keyring.WithKeyringAppName(keyringParams.KeyringAppName))
		}
		if len(keyringParams.KeyringBackend) > 0 {
			keyringBackend := keyring.Backend(keyringParams.KeyringBackend)
			switch keyringBackend {
			case keyring.BackendFile, keyring.BackendOS, keyring.BackendTest:
				opts = append(opts, keyring.WithKeyringBackend(keyringBackend))
			default:
				err := errors.Errorf("unsupported keyring backend provided: %s", keyringBackend)
				return err
			}
		}
		if len(keyringParams.KeyFrom) > 0 {
			opts = append(opts, keyring.WithKeyFrom(keyringParams.KeyFrom))
		}
		if len(keyringParams.KeyPassphrase) > 0 {
			opts = append(opts, keyring.WithKeyPassphrase(keyringParams.KeyPassphrase))
		}
		if len(keyringParams.PrivKeyHex) > 0 {
			opts = append(opts, keyring.WithPrivKeyHex(keyringParams.PrivKeyHex))
		}
		if keyringParams.UseLedger {
			opts = append(opts, keyring.WithUseLedger(keyringParams.UseLedger))
		}

		mainAcc, kb, err := keyring.NewCosmosKeyring(opts...)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize Cosmos keyring")
			return err
		}
		deployConfig.From, err = cored.NewWalletFromKeyring(kb, mainAcc)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize cored.Wallet from provided keyring")
			return err
		}

		return contracts.Deploy(ctx, *deployConfig)
	}
}

func addInitFlags(initCmd *cobra.Command, initConfig *contracts.InitConfig) {
	initCmd.Flags().StringVar(
		&initConfig.TemplateRepoURL,
		"template-repo",
		defaultString("CRUST_CONTRACTS_TEMPLATE_REPO", defaultTemplateRepo),
		"Public Git repo URL to clone smart-contract template from",
	)
	initCmd.Flags().StringVar(
		&initConfig.TemplateVersion,
		"template-version",
		defaultString("CRUST_CONTRACTS_TEMPLATE_VERSION", defaultTemplateVersion),
		"Specify the version of the template, e.g. 1.0, 1.0-minimal, 0.16",
	)
	initCmd.Flags().StringVar(
		&initConfig.TemplateSubdir,
		"template-subdir",
		defaultString("CRUST_CONTRACTS_TEMPLATE_SUBDIR", ""),
		"Specify a subfolder within the template repository to be used as the actual template",
	)
	initCmd.Flags().StringVar(
		&initConfig.ProjectName,
		"project-name",
		defaultString("CRUST_CONTRACTS_PROJECT_NAME", ""),
		"Specify smart-contract name for the scaffolded template",
	)
}

func addTestFlags(testCmd *cobra.Command, testConfig *contracts.TestConfig) {
	testCmd.Flags().BoolVar(
		&testConfig.NeedCoverageReport,
		"coverage",
		toBool("CRUST_CONTRACTS_COVERAGE_REPORT", false),
		"Enable code coverage report using tarpaulin - works only on Linux/amd64 targets.",
	)
	testCmd.Flags().BoolVar(
		&testConfig.HasIntegrationTests,
		"integration",
		toBool("CRUST_CONTRACTS_INTEGRATION_TEST", false),
		"Enables the integration tests stage.",
	)
}

func addBuildFlags(buildCmd *cobra.Command, buildConfig *contracts.BuildConfig) {
	buildCmd.Flags().BoolVar(
		&buildConfig.NeedOptimizedBuild,
		"optimized",
		toBool("CRUST_CONTRACTS_BUILD_OPTIMIZED", true),
		"Enables WASM optimized build using a special Docker image, ensuring minimum deployment size and predictable execution.",
	)
}

func addDeployFlags(deployCmd *cobra.Command, deployConfig *contracts.DeployConfig) {
	deployCmd.Flags().BoolVar(
		&deployConfig.NeedsRebuild,
		"rebuild",
		toBool("CRUST_CONTRACTS_DEPLOY_NEEDS_REBUILD", false),
		"Forces an optimized rebuild of the WASM artefact, even if it exists. Requires WorkspaceDir to be present and valid.",
	)
	deployCmd.Flags().BoolVar(
		&deployConfig.InstantiationConfig.NeedInstantiation,
		"init",
		toBool("CRUST_CONTRACTS_DEPLOY_NEEDS_INSTANTIATION", true),
		"Enables 2nd stage (contract instantiation) to be executed after code has been stored on chain.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.CodeID,
		"code-id",
		defaultString("CRUST_CONTRACTS_DEPLOY_CODE_ID", ""),
		"Specify existing program Code ID to skip the store stage.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.Network.ChainID,
		"chain-id",
		defaultString("CRUST_CONTRACTS_DEPLOY_CHAIN_ID", "coredev"),
		"ChainID used to sign transactions.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.Network.RPCEndpoint,
		"rpc-endpoint",
		defaultString("CRUST_CONTRACTS_DEPLOY_RPC_ENDPOINT", "http://localhost:26657"),
		"Specify the Tendermint RPC endpoint for the chain client",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.Network.MinGasPrice,
		"min-gas-price",
		defaultString("CRUST_CONTRACTS_DEPLOY_CHAIN_MIN_GAS_PRICE", "1core"),
		"Sets the minimum gas price required to be paid to get the transaction included in a block.",
	)
}

type KeyringParams struct {
	KeyringDir     string
	KeyringAppName string
	KeyringBackend string
	KeyFrom        string
	KeyPassphrase  string
	PrivKeyHex     string
	UseLedger      bool
}

func addDeployKeyringFlags(deployCmd *cobra.Command, keyringParams *KeyringParams) {
	deployCmd.Flags().StringVar(
		&keyringParams.KeyringDir,
		"keyring-dir",
		defaultString("CRUST_KEYRING_DIR", ""),
		"Sets keyring path in the filesystem, useful when CRUST_KEYRING_BACKEND is \"file\".",
	)
	deployCmd.Flags().StringVar(
		&keyringParams.KeyringAppName,
		"keyring-app-name",
		defaultString("CRUST_KEYRING_APP_NAME", "cored"),
		"Sets keyring application name (used by Cosmos to separate keyrings)",
	)
	deployCmd.Flags().StringVar(
		&keyringParams.KeyringBackend,
		"keyring-backend",
		defaultString("CRUST_KEYRING_BACKEND", "test"),
		"Sets the keyring backend. Expected values: test, file, os.",
	)
	deployCmd.Flags().StringVar(
		&keyringParams.KeyFrom,
		"key-from",
		defaultString("CRUST_KEYRING_KEY_FROM", ""),
		"Sets the key name to use for signing. Must exist in the provided keyring.",
	)
	deployCmd.Flags().StringVar(
		&keyringParams.KeyPassphrase,
		"key-passphrase",
		defaultString("CRUST_KEYRING_KEY_PASSPHRASE", ""),
		"Sets the passphrase for keyring files. Insecure option, use for testing only. If not set, will read from stdin.",
	)
	deployCmd.Flags().StringVar(
		&keyringParams.PrivKeyHex,
		"key-priv-hex",
		defaultString("CRUST_KEYRING_PRIVKEY_HEX", ""),
		"Specify a private key as plaintext hex. Insecure option, use for testing only.",
	)
	deployCmd.Flags().BoolVar(
		&keyringParams.UseLedger,
		"use-ledger",
		toBool("CRUST_KEYRING_USE_LEDGER", false),
		"Set the option to use hardware wallet, if available on the system.",
	)
}

func defaultString(env, def string) string {
	val := os.Getenv(env)
	if val == "" {
		val = def
	}
	return val
}

func toBool(env string, def bool) bool {
	switch strings.ToLower(os.Getenv(env)) {
	case "1", "y", "yes", "true":
		return true
	default:
		return false
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
