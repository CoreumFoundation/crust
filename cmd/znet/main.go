package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/pkg/znet"
)

func main() {
	run.Tool("znet", func(ctx context.Context) error {
		configF := infra.NewConfigFactory()
		cmdF := znet.NewCmdFactory(configF)

		rootCmd := rootCmd(ctx, configF, cmdF)
		rootCmd.AddCommand(startCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(stopCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(removeCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(testCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(specCmd(configF, cmdF))
		rootCmd.AddCommand(consoleCmd(ctx, configF, cmdF))

		return rootCmd.Execute()
	})
}

func rootCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Short:         "Creates preconfigured session for environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Activate(ctx, configF, config)
		}),
	}
	logger.AddFlags(logger.ToolDefaultConfig, rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().StringVar(&configF.EnvName, "env", defaultString("CRUST_ZNET_ENV", "znet"), "Name of the environment to run in")
	rootCmd.PersistentFlags().StringVar(&configF.HomeDir, "home", defaultString("CRUST_ZNET_HOME", must.String(os.UserCacheDir())+"/crust/znet"), "Directory where all files created automatically by znet are stored")
	addBinDirFlag(rootCmd, configF)
	addProfileFlag(rootCmd, configF)
	addCoredVersionFlag(rootCmd, configF)
	addFilterFlag(rootCmd, configF)
	return rootCmd
}

func startCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Starts environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Start(ctx, config, spec)
		}),
	}
	addBinDirFlag(startCmd, configF)
	addProfileFlag(startCmd, configF)
	addCoredVersionFlag(startCmd, configF)

	return startCmd
}

func stopCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stops environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Stop(ctx, config, spec)
		}),
	}
}

func removeCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Removes environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Remove(ctx, config, spec)
		}),
	}
}

func testCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Runs integration tests for all repos",
		RunE: cmdF.Cmd(func() error {
			configF.Profiles = apps.IntegrationTestsProfiles()
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Test(ctx, config, spec)
		}),
	}
	addTestGroupFlag(testCmd, configF)
	addBinDirFlag(testCmd, configF)
	addFilterFlag(testCmd, configF)
	addCoredVersionFlag(testCmd, configF)
	return testCmd
}

func specCmd(configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "spec",
		Short: "Prints specification of running environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			return znet.Spec(spec)
		}),
	}
}

func consoleCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "console",
		Short: "Starts tmux console on top of running environment",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Console(ctx, config, spec)
		}),
	}
}

func addTestGroupFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringSliceVar(
		&configF.TestGroups,
		"test-groups",
		[]string{},
		"Test groups in supported repositories to run integration test for,empty means all repositories all test groups ,e.g. --test-groups=faucet,coreum-modules or --test-groups=faucet --test-groups=coreum-modules",
	)
}

func addBinDirFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(&configF.BinDir, "bin-dir", defaultString("CRUST_ZNET_BIN_DIR",
		filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable())))))),
		"Path to directory where executables exist")
}

func addProfileFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringSliceVar(&configF.Profiles, "profiles", defaultStrings("CRUST_ZNET_PROFILES", apps.DefaultProfiles()), "List of application profiles to deploy: "+strings.Join(apps.Profiles(), " | "))
}

func addCoredVersionFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(&configF.CoredVersion, "cored-version", defaultString("CRUST_ZNET_CORED_VERSION", ""), "The version of the binary to be used for deployment")
}

func addFilterFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(&configF.TestFilter, "filter", defaultString("CRUST_ZNET_FILTER", ""), "Regular expression used to filter tests to run")
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
