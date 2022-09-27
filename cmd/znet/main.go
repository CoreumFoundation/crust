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
	"github.com/CoreumFoundation/coreum/integration-tests/testing"
	"github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/pkg/znet"
)

func main() {
	run.Tool("znet", func(ctx context.Context) error {
		configF := infra.NewConfigFactory()
		cmdF := znet.NewCmdFactory(configF)
		config.NewNetwork(testing.NetworkConfig).SetupPrefixes()

		rootCmd := rootCmd(ctx, configF, cmdF)
		rootCmd.AddCommand(startCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(stopCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(removeCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(testCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(specCmd(configF, cmdF))
		rootCmd.AddCommand(consoleCmd(ctx, configF, cmdF))
		rootCmd.AddCommand(pingPongCmd(ctx, configF, cmdF))

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
	addModeFlag(rootCmd, configF)
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
	addModeFlag(startCmd, configF)
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
			configF.ModeName = "test"
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			return znet.Test(ctx, config, spec)
		}),
	}
	addTestRepoFlag(testCmd, configF)
	addBinDirFlag(testCmd, configF)
	addFilterFlag(testCmd, configF)
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

func pingPongCmd(ctx context.Context, configF *infra.ConfigFactory, cmdF *znet.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "ping-pong",
		Short: "Sends tokens back and forth to generate transactions",
		RunE: cmdF.Cmd(func() error {
			spec := infra.NewSpec(configF)
			config := znet.NewConfig(configF, spec)
			appF := apps.NewFactory(config, spec, testing.NetworkConfig)
			mode, err := znet.Mode(appF, config.ModeName)
			if err != nil {
				return err
			}
			return znet.PingPong(ctx, mode)
		}),
	}
}

func addTestRepoFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringSliceVar(
		&configF.TestRepos,
		"repos",
		[]string{},
		"Repositories to run integration test for,empty means all repositories,e.g. --repos=faucet,coreum or --repos=faucet --repos=coreum",
	)
}

func addBinDirFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(&configF.BinDir, "bin-dir", defaultString("CRUST_ZNET_BIN_DIR",
		filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable())))))),
		"Path to directory where executables exist")
}

func addModeFlag(cmd *cobra.Command, configF *infra.ConfigFactory) {
	cmd.Flags().StringVar(&configF.ModeName, "mode", defaultString("CRUST_ZNET_MODE", "dev"), "List of applications to deploy: "+strings.Join(znet.Modes(), " | "))
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
