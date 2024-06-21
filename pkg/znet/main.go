package znet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
)

// Main is the main function of znet.
func Main() {
	run.Tool("znet", func(ctx context.Context) error {
		configF := infra.NewConfigFactory()
		cmdF := NewCmdFactory(configF)

		rootCmd := rootCmd(ctx, configF, cmdF)
		rootCmd.AddCommand(startCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(stopCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(removeCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(specCmd(configF, cmdF))
		rootCmd.AddCommand(consoleCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(coverageConvertCmd(ctx, configF, cmdF))

		return rootCmd.Execute()
	})
}

func rootCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Creates preconfigured session for environment",
		RunE: cmdF.Cmd(func() error {
			return Activate(ctx, configF)
		}),
	}
	logger.AddFlags(logger.ToolDefaultConfig, rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().StringVar(
		&configF.EnvName,
		"env",
		defaultString("CRUST_ZNET_ENV", "znet"),
		"Name of the environment to run in",
	)
	rootCmd.PersistentFlags().StringVar(
		&configF.HomeDir,
		"home",
		defaultString("CRUST_ZNET_HOME", must.String(os.UserCacheDir())+"/crust/znet"),
		"Directory where all files created automatically by znet are stored",
	)
	addRootDirFlag(rootCmd, configF)
	addProfileFlag(rootCmd, configF)
	addCoredVersionFlag(rootCmd, configF)
	return rootCmd
}

func startCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Starts environment",
		RunE: cmdF.Cmd(func() error {
			return Start(ctx, configF)
		}),
	}
	addRootDirFlag(startCmd, configF)
	addProfileFlag(startCmd, configF)
	addCoredVersionFlag(startCmd, configF)
	addTimeoutCommitFlag(startCmd, configF)

	return startCmd
}

func stopCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops environment",
		RunE: cmdF.Cmd(func() error {
			return Stop(ctx, configF)
		}),
	}
}

func removeCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Removes environment",
		RunE: cmdF.Cmd(func() error {
			return Remove(ctx, configF)
		}),
	}
}

func specCmd(configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Prints specification of running environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			return Spec(spec)
		}),
	}
}

func consoleCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "console",
		Short: "Starts tmux console on top of running environment",
		RunE: cmdF.Cmd(func() error {
			return Console(ctx, configF)
		}),
	}
}

func coverageConvertCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage-convert",
		Short: "Converts codecoverage report from binary to text format and stores in folder specified by flag",
		RunE: cmdF.Cmd(func() error {
			return CoverageConvert(ctx, configF)
		}),
	}

	addCoverageOutputFlag(cmd, configF)
	return cmd
}

func addRootDirFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(
		&configF.RootDir,
		"root-dir",
		defaultString("CRUST_ZNET_ROOT_DIR", repoRoot()),
		"Path to directory where current repository exists",
	)
}

func addProfileFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringSliceVar(
		&configF.Profiles,
		"profiles",
		defaultStrings("CRUST_ZNET_PROFILES", apps.DefaultProfiles()),
		"List of application profiles to deploy: "+strings.Join(apps.Profiles(), " | "),
	)
}

func addTimeoutCommitFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	defaultTimeoutCommitString := defaultString("CRUST_ZNET_TIMEOUT_COMMIT", "0s")
	defaultTimeoutCommit, err := time.ParseDuration(defaultTimeoutCommitString)
	if err != nil {
		panic(errors.Errorf("failed to covert default timeout commit to duration, err:%s", err))
	}
	cmd.Flags().DurationVar(&configF.TimeoutCommit, "timeout-commit", defaultTimeoutCommit, "Chains timeout commit.")
}

func addCoredVersionFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(
		&configF.CoredVersion,
		"cored-version",
		defaultString("CRUST_ZNET_CORED_VERSION", ""),
		"The version of the binary to be used for deployment",
	)
}

func addCoverageOutputFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(
		&configF.CoverageOutputFile,
		"coverage-output",
		defaultString("CRUST_ZNET_COVERAGE_OUTPUT",
			filepath.Clean(filepath.Join(configF.RootDir, "coverage/coreum-integration-tests-modules"))),
		"Output path for coverage data in text format",
	)
}

func repoRoot() string {
	currentBinaryPath := must.String(filepath.EvalSymlinks(must.String(os.Executable())))

	// to detect crust repo root we go 3 levels up.
	return filepath.Clean(filepath.Join(currentBinaryPath, "../../.."))
}

func defaultString(env, def string) string {
	val := os.Getenv(env)
	if val == "" {
		val = def
	}
	return val
}

func defaultStrings(env string, def []string) []string {
	val := os.Getenv(env)
	if val == "" {
		return def
	}

	parts := strings.Split(val, ",")
	def = make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			def = append(def, p)
		}
	}
	return def
}
