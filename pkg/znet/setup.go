package znet

import (
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/coreum-tools/pkg/ioc"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/integration-tests/testing"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps"
	"github.com/CoreumFoundation/crust/infra/targets"
)

// IoC configures IoC container
func IoC(c *ioc.Container) {
	c.Singleton(func() config.NetworkConfig {
		// FIXME (wojtek): this is only a temporary hack until we develop our own address encoder.
		config.NewNetwork(testing.NetworkConfig).SetupPrefixes()
		return testing.NetworkConfig
	})
	c.Singleton(NewCmdFactory)
	c.Singleton(infra.NewConfigFactory)
	c.Singleton(infra.NewSpec)
	c.Transient(NewConfig)
	c.Transient(apps.NewFactory)
	c.TransientNamed("dev", DevMode)
	c.TransientNamed("test", TestMode)
	c.Transient(func(c *ioc.Container, config infra.Config) infra.Mode {
		var mode infra.Mode
		c.ResolveNamed(config.ModeName, &mode)
		return mode
	})
	c.Transient(targets.NewDocker)
}

// NewCmdFactory returns new CmdFactory
func NewCmdFactory(c *ioc.Container, configF *infra.ConfigFactory) *CmdFactory {
	return &CmdFactory{
		c:       c,
		configF: configF,
	}
}

// CmdFactory is a wrapper around cobra RunE
type CmdFactory struct {
	c       *ioc.Container
	configF *infra.ConfigFactory
}

// Cmd returns function compatible with RunE
func (f *CmdFactory) Cmd(cmdFunc interface{}) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		f.configF.VerboseLogging = cmd.Flags().Lookup("verbose").Value.String() == "true"
		f.configF.LogFormat = cmd.Flags().Lookup("log-format").Value.String()
		var err error
		f.c.Call(cmdFunc, &err)
		return err
	}
}

// NewConfig produces final config
func NewConfig(configF *infra.ConfigFactory, spec *infra.Spec) infra.Config {
	must.OK(os.MkdirAll(configF.HomeDir, 0o700))
	homeDir := must.String(filepath.Abs(must.String(filepath.EvalSymlinks(configF.HomeDir)))) + "/" + configF.EnvName
	if err := os.Mkdir(homeDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}

	config := infra.Config{
		EnvName:        configF.EnvName,
		ModeName:       spec.Mode,
		HomeDir:        homeDir,
		AppDir:         homeDir + "/app",
		WrapperDir:     homeDir + "/bin",
		BinDir:         must.String(filepath.Abs(must.String(filepath.EvalSymlinks(configF.BinDir)))),
		TestFilter:     configF.TestFilter,
		VerboseLogging: configF.VerboseLogging,
		LogFormat:      configF.LogFormat,
	}

	// we use append to make a copy of the original list, so it is not passed by reference
	config.TestRepos = append([]string{}, configF.TestRepos...)

	createDirs(config)

	return config
}

func createDirs(config infra.Config) {
	must.OK(os.MkdirAll(config.AppDir, 0o700))
	must.OK(os.MkdirAll(config.WrapperDir, 0o700))
}
