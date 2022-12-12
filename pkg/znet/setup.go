package znet

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/crust/infra"
)

// NewCmdFactory returns new CmdFactory
func NewCmdFactory(configF *infra.ConfigFactory) *CmdFactory {
	return &CmdFactory{
		configF: configF,
	}
}

// CmdFactory is a wrapper around cobra RunE
type CmdFactory struct {
	configF *infra.ConfigFactory
}

// Cmd returns function compatible with RunE
func (f *CmdFactory) Cmd(cmdFunc func() error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		f.configF.VerboseLogging = cmd.Flags().Lookup("verbose").Value.String() == "true"
		f.configF.LogFormat = cmd.Flags().Lookup("log-format").Value.String()
		return cmdFunc()
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
		Profiles:       spec.Profiles,
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
