package znet

import (
	"context"
	"fmt"
	"math/big"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"time"

	cosmosclient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/ioc"
	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/parallel"
	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/tx"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
	"github.com/CoreumFoundation/crust/pkg/znet/tmux"
)

var exe = must.String(filepath.EvalSymlinks(must.String(os.Executable())))

// Activate starts preconfigured shell environment
func Activate(ctx context.Context, configF *infra.ConfigFactory, config infra.Config) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.WithStack(err)
	}
	defer watcher.Close()

	// To be notified about directory being removed we must observe parent directory
	if err := watcher.Add(filepath.Dir(config.HomeDir)); err != nil {
		return errors.WithStack(err)
	}

	saveWrapper(config.WrapperDir, "start", "start")
	saveWrapper(config.WrapperDir, "stop", "stop")
	saveWrapper(config.WrapperDir, "remove", "remove")
	// `test` can't be used here because it is a reserved keyword in bash
	saveWrapper(config.WrapperDir, "tests", "test")
	saveWrapper(config.WrapperDir, "spec", "spec")
	saveWrapper(config.WrapperDir, "console", "console")
	saveWrapper(config.WrapperDir, "ping-pong", "ping-pong")
	saveLogsWrapper(config.WrapperDir, config.EnvName, "logs")

	shell, promptVar, err := shellConfig(config.EnvName)
	if err != nil {
		return err
	}
	shellCmd := osexec.Command(shell)
	shellCmd.Env = append(os.Environ(),
		"PATH="+config.WrapperDir+":"+os.Getenv("PATH"),
		"CRUST_ZNET_ENV="+configF.EnvName,
		"CRUST_ZNET_MODE="+configF.ModeName,
		"CRUST_ZNET_HOME="+configF.HomeDir,
		"CRUST_ZNET_BIN_DIR="+configF.BinDir,
		"CRUST_ZNET_FILTER="+configF.TestFilter,
	)
	if promptVar != "" {
		shellCmd.Env = append(shellCmd.Env, promptVar)
	}
	shellCmd.Dir = config.HomeDir
	shellCmd.Stdin = os.Stdin

	return parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		spawn("session", parallel.Exit, func(ctx context.Context) error {
			err = libexec.Exec(ctx, shellCmd)
			if shellCmd.ProcessState != nil && shellCmd.ProcessState.ExitCode() != 0 {
				// shell returns non-exit code if command executed in the shell failed
				return nil
			}
			return err
		})
		spawn("fsnotify", parallel.Exit, func(ctx context.Context) error {
			defer func() {
				if shellCmd.Process != nil {
					// Shell exits only if SIGHUP is received. All the other signals are caught and passed to process
					// running inside the shell.
					_ = shellCmd.Process.Signal(syscall.SIGHUP)
				}
			}()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case event := <-watcher.Events:
					// Rename is here because on some OSes removing is done by moving file to trash
					if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 && event.Name == config.HomeDir {
						return nil
					}
				case err := <-watcher.Errors:
					return errors.WithStack(err)
				}
			}
		})
		return nil
	})
}

// Start starts environment
func Start(ctx context.Context, target infra.Target, mode infra.Mode, spec *infra.Spec) (retErr error) {
	if err := spec.Verify(); err != nil {
		return err
	}
	return target.Deploy(ctx, mode)
}

// Stop stops environment
func Stop(ctx context.Context, target infra.Target, spec *infra.Spec) (retErr error) {
	defer func() {
		for _, app := range spec.Apps {
			app.SetInfo(infra.DeploymentInfo{Status: infra.AppStatusStopped})
		}
		if err := spec.Save(); retErr == nil {
			retErr = err
		}
	}()
	return target.Stop(ctx)
}

// Remove removes environment
func Remove(ctx context.Context, config infra.Config, target infra.Target) (retErr error) {
	if err := target.Remove(ctx); err != nil {
		return err
	}

	// It may happen that some files are flushed to disk even after processes are terminated
	// so let's try to delete dir a few times
	var err error
	for i := 0; i < 3; i++ {
		if err = os.RemoveAll(config.HomeDir); err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.WithStack(err)
}

// Test runs integration tests
func Test(c *ioc.Container, configF *infra.ConfigFactory) error {
	configF.ModeName = "test"
	var err error
	c.Call(func(ctx context.Context, config infra.Config, target infra.Target, mode infra.Mode, spec *infra.Spec) (retErr error) {
		if err := spec.Verify(); err != nil {
			return err
		}
		for _, app := range spec.Apps {
			if app.Info().Status == infra.AppStatusStopped {
				return errors.New("tests can't be executed on top of stopped environment, start it first")
			}
		}

		return testing.Run(ctx, target, mode, config)
	}, &err)
	return err
}

// Spec prints specification of running environment
func Spec(spec *infra.Spec) error {
	fmt.Println(spec)
	return nil
}

// Console starts tmux session on top of running environment
func Console(ctx context.Context, config infra.Config, spec *infra.Spec) error {
	if err := tmux.Kill(ctx, config.EnvName); err != nil {
		return err
	}

	containers := map[string]string{}
	for appName, app := range spec.Apps {
		if app.Info().Status == infra.AppStatusRunning {
			containers[appName] = app.Info().Container
		}
	}
	if len(containers) == 0 {
		logger.Get(ctx).Info("There are no running applications to show in tmux console")
		return nil
	}

	appNames := make([]string, 0, len(containers))
	for appName := range containers {
		appNames = append(appNames, appName)
	}
	sort.Strings(appNames)

	for _, appName := range appNames {
		if err := tmux.ShowContainerLogs(ctx, config.EnvName, appName, containers[appName]); err != nil {
			return err
		}
	}
	if err := tmux.Attach(ctx, config.EnvName); err != nil {
		return err
	}
	return tmux.Kill(ctx, config.EnvName)
}

// PingPong connects to cored node and sends transactions back and forth from one account to another to generate
// transactions on the blockchain
func PingPong(ctx context.Context, mode infra.Mode) error {
	coredApp := mode.FindAnyRunningApp(cored.AppType)
	if coredApp == nil {
		return errors.New("no running cored app found")
	}
	coredNode := coredApp.(cored.Cored)
	client := coredNode.Client()

	alicePrivKey, err := cored.PrivateKeyFromMnemonic(cored.AliceMnemonic)
	if err != nil {
		return err
	}
	bobPrivKey, err := cored.PrivateKeyFromMnemonic(cored.BobMnemonic)
	if err != nil {
		return err
	}
	charliePrivKey, err := cored.PrivateKeyFromMnemonic(cored.CharlieMnemonic)
	if err != nil {
		return err
	}

	alice := types.Wallet{Name: "alice", Key: alicePrivKey}
	bob := types.Wallet{Name: "bob", Key: bobPrivKey}
	charlie := types.Wallet{Name: "charlie", Key: charliePrivKey}

	for {
		if err := sendTokens(ctx, client.Context(), alice, bob, *coredNode.Network()); err != nil {
			return err
		}
		if err := sendTokens(ctx, client.Context(), bob, charlie, *coredNode.Network()); err != nil {
			return err
		}
		if err := sendTokens(ctx, client.Context(), charlie, alice, *coredNode.Network()); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func sendTokens(ctx context.Context, clientCtx cosmosclient.Context, from, to types.Wallet, network app.Network) error {
	log := logger.Get(ctx)

	amount, err := types.NewCoin(big.NewInt(1), network.TokenSymbol())
	if err != nil {
		return err
	}
	gasPrice, err := types.NewCoin(network.FeeModel().InitialGasPrice.BigInt(), network.TokenSymbol())
	if err != nil {
		return err
	}
	senderPrivateKey := secp256k1.PrivKey{Key: from.Key}
	fromAddress := sdk.AccAddress(senderPrivateKey.PubKey().Address())

	toPrivateKey := secp256k1.PrivKey{Key: to.Key}
	toAddress := sdk.AccAddress(toPrivateKey.PubKey().Address())
	msg := &banktypes.MsgSend{
		FromAddress: fromAddress.String(),
		ToAddress:   toAddress.String(),
		Amount: []sdk.Coin{
			sdk.NewCoin(amount.Denom, sdk.NewIntFromBigInt(amount.Amount)),
		},
	}
	signInput := tx.SignInput{
		PrivateKey: senderPrivateKey,
		GasLimit:   network.DeterministicGas().BankSend,
		GasPrice:   sdk.NewCoin(gasPrice.Denom, sdk.NewIntFromBigInt(gasPrice.Amount)),
	}
	result, err := tx.BroadcastSync(ctx, clientCtx, signInput, msg)
	if err != nil {
		return err
	}

	log.Info("Sent tokens", zap.Stringer("from", from), zap.Stringer("to", to),
		zap.Stringer("amount", amount), zap.String("txHash", result.Hash.String()),
		zap.Int64("gasUsed", result.TxResult.GasUsed))

	bankQueryClient := banktypes.NewQueryClient(clientCtx)
	fromBalance, err := bankQueryClient.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{Address: fromAddress.String()})
	if err != nil {
		return err
	}
	toBalance, err := bankQueryClient.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{Address: toAddress.String()})
	if err != nil {
		return err
	}

	log.Info("Current balance", zap.Stringer("wallet", from), zap.Stringer("balance", fromBalance.GetBalances().AmountOf(network.TokenSymbol())))
	log.Info("Current balance", zap.Stringer("wallet", to), zap.Stringer("balance", toBalance.GetBalances().AmountOf(network.TokenSymbol())))

	return nil
}

func saveWrapper(dir, file, command string) {
	must.OK(os.WriteFile(dir+"/"+file, []byte(`#!/bin/bash
exec "`+exe+`" "`+command+`" "$@"
`), 0o700))
}

func saveLogsWrapper(dir, envName, file string) {
	must.OK(os.WriteFile(dir+"/"+file, []byte(`#!/bin/bash
if [ "$1" == "" ]; then
  echo "Provide the name of application"
  exit 1
fi
exec docker logs -f "`+envName+`-$1"
`), 0o700))
}

var supportedShells = map[string]func(envName string) string{
	"bash": func(envName string) string {
		return "PS1=(" + envName + `) [\u@\h \W]\$ `
	},
	"zsh": func(envName string) string {
		return "PROMPT=(" + envName + `) [%n@%m %1~]%# `
	},
}

func shellConfig(envName string) (string, string, error) {
	shell := os.Getenv("SHELL")
	if _, exists := supportedShells[filepath.Base(shell)]; !exists {
		var shells []string
		switch runtime.GOOS {
		case "darwin":
			shells = []string{"zsh", "bash"}
		default:
			shells = []string{"bash", "zsh"}
		}
		for _, s := range shells {
			if shell2, err := osexec.LookPath(s); err == nil {
				shell = shell2
				break
			}
		}
	}
	if shell == "" {
		return "", "", errors.New("custom shell not defined and supported shell not found")
	}

	var promptVar string
	if promptVarFn, exists := supportedShells[filepath.Base(shell)]; exists {
		promptVar = promptVarFn(envName)
	}
	return shell, promptVar, nil
}
