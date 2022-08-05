package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/cmd"
	"github.com/CoreumFoundation/crust/infra/apps/cored/keyring"
	"github.com/CoreumFoundation/crust/pkg/contracts"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
		addNetworkFlags(deployCmd, &deployConfig.Network)
		addTxKeyringFlags(deployCmd, &keyringParams)
		rootCmd.AddCommand(deployCmd)

		var execConfig contracts.ExecuteConfig
		execCmd := &cobra.Command{
			Use:     "exec [contract-address] [payload-json]",
			Aliases: []string{"e", "transact", "tx"},
			Short:   "Executes a command on a WASM contract, with the payload provided.",
			Args:    cobra.MaximumNArgs(2),
			RunE:    executeRunE(ctx, &execConfig, &keyringParams),
		}
		addExecuteFlags(execCmd, &execConfig)
		addNetworkFlags(execCmd, &execConfig.Network)
		addTxKeyringFlags(execCmd, &keyringParams)
		rootCmd.AddCommand(execCmd)

		var queryConfig contracts.QueryConfig
		queryCmd := &cobra.Command{
			Use:     "query [contract-address] [payload-json]",
			Aliases: []string{"q", "call"},
			Short:   "Calls contract with given address with query data and prints the returned result.",
			Args:    cobra.MaximumNArgs(2),
			RunE:    queryRunE(ctx, &queryConfig),
		}
		addNetworkFlags(queryCmd, &queryConfig.Network)
		rootCmd.AddCommand(queryCmd)

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

		_, err := contracts.Build(ctx, targetDir, *buildConfig)
		return err
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

		mainAcc, kb, err := keyring.NewCosmosKeyring(keyringParams.Opts()...)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize Cosmos keyring")
			return err
		}
		deployConfig.From, err = types.NewWalletFromKeyring(kb, mainAcc)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize coreum.Wallet from provided keyring")
			return err
		}

		out, err := contracts.Deploy(ctx, *deployConfig)
		if out != nil {
			outMsg, err := json.Marshal(out)
			must.OK(err)
			fmt.Println(string(outMsg))
		}

		return err
	}
}

func executeRunE(
	ctx context.Context,
	execConfig *contracts.ExecuteConfig,
	keyringParams *KeyringParams,
) RunE {
	return func(cmd *cobra.Command, args []string) error {
		var contractAddress string

		switch len(args) {
		case 2:
			contractAddress = args[0]
			execConfig.ExecutePayload = args[1]
		case 1:
			contractAddress = args[0]
			execConfig.ExecutePayload = "{}"
		case 0:
			err := errors.New("at least 1 argument with contract address must be provided")
			return err
		}

		mainAcc, kb, err := keyring.NewCosmosKeyring(keyringParams.Opts()...)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize Cosmos keyring")
			return err
		}
		execConfig.From, err = types.NewWalletFromKeyring(kb, mainAcc)
		if err != nil {
			err = errors.Wrap(err, "failed to initialize coreum.Wallet from provided keyring")
			return err
		}

		out, err := contracts.Execute(ctx, contractAddress, *execConfig)
		if out != nil {
			outMsg, err := json.Marshal(out)
			must.OK(err)
			fmt.Println(string(outMsg))
		}

		return err
	}
}

func queryRunE(
	ctx context.Context,
	queryConfig *contracts.QueryConfig,
) RunE {
	return func(cmd *cobra.Command, args []string) error {
		var contractAddress string

		switch len(args) {
		case 2:
			contractAddress = args[0]
			queryConfig.QueryPayload = args[1]
		case 1:
			contractAddress = args[0]
			queryConfig.QueryPayload = "{}"
		case 0:
			err := errors.New("at least 1 argument with contract address must be provided")
			return err
		}

		out, err := contracts.Query(ctx, contractAddress, *queryConfig)
		if out != nil {
			outMsg, err := json.Marshal(out)
			must.OK(err)
			fmt.Println(string(outMsg))
		}

		return err
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
		"Enable code coverage report using tarpaulin (Linux x64 / MacOS x64 / M1).",
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

func addNetworkFlags(cmd *cobra.Command, networkConfig *contracts.ChainConfig) {
	cmd.Flags().StringVar(
		&networkConfig.ChainID,
		"chain-id",
		defaultString("CRUST_CONTRACTS_NETWORK_CHAIN_ID", "coredev"),
		"ChainID used to sign transactions.",
	)
	cmd.Flags().StringVar(
		&networkConfig.RPCEndpoint,
		"rpc-endpoint",
		defaultString("CRUST_CONTRACTS_NETWORK_RPC_ENDPOINT", "http://localhost:26657"),
		"Specify the Tendermint RPC endpoint for the chain client",
	)
	cmd.Flags().StringVar(
		&networkConfig.MinGasPrice,
		"min-gas-price",
		defaultString("CRUST_CONTRACTS_NETWORK_MIN_GAS_PRICE", "1500core"),
		"Sets the minimum gas price required to be paid to get the transaction included in a block.",
	)
}

func addDeployFlags(deployCmd *cobra.Command, deployConfig *contracts.DeployConfig) {
	deployCmd.Flags().BoolVar(
		&deployConfig.NeedRebuild,
		"rebuild",
		toBool("CRUST_CONTRACTS_DEPLOY_NEED_REBUILD", false),
		"Forces an optimized rebuild of the WASM artefact, even if it exists. Requires WorkspaceDir to be present and valid.",
	)
	deployCmd.Flags().BoolVar(
		&deployConfig.InstantiationConfig.NeedInstantiation,
		"init",
		toBool("CRUST_CONTRACTS_DEPLOY_NEED_INSTANTIATION", true),
		"Enables 2nd stage (contract instantiation) to be executed after code has been stored on chain.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.InstantiatePayload,
		"init-payload",
		defaultString("CRUST_CONTRACTS_DEPLOY_INIT_PAYLOAD", ""),
		"Path to a file containing JSON-encoded contract instantiate args, or JSON-encoded body itself.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.AccessType,
		"init-access-type",
		defaultString("CRUST_CONTRACTS_DEPLOY_INIT_ACCESS_TYPE", string(contracts.AccessTypeUnspecified)),
		`Sets the permission flag, affecting who can instantiate this contract. Expects "nobody", "address", "unrestricted".`,
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.AccessAddress,
		"init-access-address",
		defaultString("CRUST_CONTRACTS_DEPLOY_INIT_ACCESS_TYPE", ""),
		`Sets the address when CRUST_CONTRACTS_DEPLOY_INIT_ACCESS_TYPE is "address".`,
	)
	deployCmd.Flags().BoolVar(
		&deployConfig.InstantiationConfig.NeedAdmin,
		"need-admin",
		toBool("CRUST_CONTRACTS_DEPLOY_NEED_ADMIN", true),
		"Set the admin address explicitly. If false, there will be no admin.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.AdminAddress,
		"admin-address",
		defaultString("CRUST_CONTRACTS_DEPLOY_ADMIN_ADDRESS", ""),
		"Sets the address when CRUST_CONTRACTS_DEPLOY_NEED_ADMIN is true. If empty, sender will be set as an admin.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.Amount,
		"amount",
		defaultString("CRUST_CONTRACTS_DEPLOY_AMOUNT", ""),
		"Specifies Coins to send to the contract during instantiation.",
	)
	deployCmd.Flags().StringVar(
		&deployConfig.InstantiationConfig.Label,
		"label",
		defaultString("CRUST_CONTRACTS_DEPLOY_LABEL", ""),
		"Sets the human-readable label for the contract instance during instantiation.",
	)
	deployCmd.Flags().Uint64Var(
		&deployConfig.CodeID,
		"code-id",
		defaultUInt64("CRUST_CONTRACTS_DEPLOY_CODE_ID", 0),
		"Specify existing program Code ID to skip the store stage.",
	)
}

func addExecuteFlags(execCmd *cobra.Command, execConfig *contracts.ExecuteConfig) {
	execCmd.Flags().StringVar(
		&execConfig.Amount,
		"amount",
		defaultString("CRUST_CONTRACTS_EXEC_AMOUNT", ""),
		"Specifies Coins to send to the contract during execution.",
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

func addTxKeyringFlags(txCmd *cobra.Command, keyringParams *KeyringParams) {
	txCmd.Flags().StringVar(
		&keyringParams.KeyringDir,
		"keyring-dir",
		defaultString("CRUST_KEYRING_DIR", ""),
		`Sets keyring path in the filesystem, useful when CRUST_KEYRING_BACKEND is "file".`,
	)
	txCmd.Flags().StringVar(
		&keyringParams.KeyringAppName,
		"keyring-app-name",
		defaultString("CRUST_KEYRING_APP_NAME", "cored"),
		"Sets keyring application name (used by Cosmos to separate keyrings)",
	)
	txCmd.Flags().StringVar(
		&keyringParams.KeyringBackend,
		"keyring-backend",
		defaultString("CRUST_KEYRING_BACKEND", "test"),
		"Sets the keyring backend. Expected values: test, file, os.",
	)
	txCmd.Flags().StringVar(
		&keyringParams.KeyFrom,
		"key-from",
		defaultString("CRUST_KEYRING_KEY_FROM", ""),
		"Sets the key name to use for signing. Must exist in the provided keyring.",
	)
	txCmd.Flags().StringVar(
		&keyringParams.KeyPassphrase,
		"key-passphrase",
		defaultString("CRUST_KEYRING_KEY_PASSPHRASE", ""),
		"Sets the passphrase for keyring files. Insecure option, use for testing only. If not set, will read from stdin.",
	)
	txCmd.Flags().StringVar(
		&keyringParams.PrivKeyHex,
		"key-priv-hex",
		defaultString("CRUST_KEYRING_PRIVKEY_HEX", ""),
		"Specify a private key as plaintext hex. Insecure option, use for testing only.",
	)
	txCmd.Flags().BoolVar(
		&keyringParams.UseLedger,
		"use-ledger",
		toBool("CRUST_KEYRING_USE_LEDGER", false),
		"Set the option to use hardware wallet, if available on the system.",
	)
}

func (k *KeyringParams) Opts() (opts []keyring.ConfigOpt) {
	if len(k.KeyringDir) > 0 {
		opts = append(opts, keyring.WithKeyringDir(k.KeyringDir))
	}
	if len(k.KeyringAppName) > 0 {
		opts = append(opts, keyring.WithKeyringAppName(k.KeyringAppName))
	}
	if len(k.KeyringBackend) > 0 {
		keyringBackend := keyring.Backend(k.KeyringBackend)
		switch keyringBackend {
		case keyring.BackendFile, keyring.BackendOS, keyring.BackendTest:
			opts = append(opts, keyring.WithKeyringBackend(keyringBackend))
		default:
			must.OK(errors.Errorf("unsupported keyring backend provided: %s", keyringBackend))
			return nil
		}
	}
	if len(k.KeyFrom) > 0 {
		opts = append(opts, keyring.WithKeyFrom(k.KeyFrom))
	}
	if len(k.KeyPassphrase) > 0 {
		opts = append(opts, keyring.WithKeyPassphrase(k.KeyPassphrase))
	}
	if len(k.PrivKeyHex) > 0 {
		opts = append(opts, keyring.WithPrivKeyHex(k.PrivKeyHex))
	}
	if k.UseLedger {
		opts = append(opts, keyring.WithUseLedger(k.UseLedger))
	}

	return opts
}

func defaultString(env, def string) string {
	val := os.Getenv(env)
	if val == "" {
		val = def
	}
	return val
}

func defaultUInt64(env string, def uint64) uint64 {
	strVal := os.Getenv(env)
	if strVal == "" {
		return def
	}

	v, err := strconv.ParseUint(strVal, 10, 64)
	must.OK(err)

	return v
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
