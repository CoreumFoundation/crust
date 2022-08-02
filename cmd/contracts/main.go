package main

import (
	"context"
	"os"
	"strings"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/crust/cmd"
	"github.com/CoreumFoundation/crust/pkg/contracts"
)

const (
	defaultTemplateRepo    = "https://github.com/CoreumFoundation/smartcontract-template.git"
	defaultTemplateVersion = "1.0"
)

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
			RunE: func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					cmd.Help()
					os.Exit(0)
				}

				return nil
			},
		}
		logger.AddFlags(logger.ToolDefaultConfig, rootCmd.PersistentFlags())

		var initConfig contracts.InitConfig
		initCmd := &cobra.Command{
			Use:     "init <target-dir>",
			Aliases: []string{"i", "gen", "generate"},
			Short:   "Initializes a new WASM smart-contract project by cloning the remote template into target dir",
			Args:    cobra.ExactValidArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				initConfig.TargetDir = args[0]
				return contracts.Init(ctx, initConfig)
			},
		}
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
		rootCmd.AddCommand(initCmd)

		var testConfig contracts.TestConfig
		testCmd := &cobra.Command{
			Use:     "test [workspace-dir]",
			Aliases: []string{"t", "unit-test"},
			Short:   "Runs unit-test suite in the workspace dir. By default current dir is used.",
			Args:    cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var targetDir string
				if len(args) > 0 {
					targetDir = args[0]
				} else {
					cwd, err := os.Getwd()
					must.OK(err)
					targetDir = cwd
				}

				return contracts.Test(ctx, targetDir, testConfig)
			},
		}
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
		rootCmd.AddCommand(testCmd)

		var buildConfig contracts.BuildConfig
		buildCmd := &cobra.Command{
			Use:     "build [workspace-dir]",
			Aliases: []string{"b"},
			Short:   "Builds the WASM contract located in the workspace dir. By default current dir is used.",
			Args:    cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				var targetDir string
				if len(args) > 0 {
					targetDir = args[0]
				} else {
					cwd, err := os.Getwd()
					must.OK(err)
					targetDir = cwd
				}

				return contracts.Build(ctx, targetDir, buildConfig)
			},
		}
		buildCmd.Flags().BoolVar(
			&buildConfig.NeedOptimizedBuild,
			"optimized",
			toBool("CRUST_CONTRACTS_BUILD_OPTIMIZED", true),
			"Enables WASM optimized build using a special Docker image, ensuring minimum deployment size and predictable execution.",
		)
		rootCmd.AddCommand(buildCmd)

		return rootCmd.Execute()
	})
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
