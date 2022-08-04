package bank

import (
	"context"
	"math/big"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/tx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum/pkg/types"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// TestInitialBalance checks that initial balance is set by genesis block
func TestInitialBalance(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var wallet types.Wallet

	// First function prepares initial well-known state
	return func(ctx context.Context) error {
			// Create new random wallet with predefined balance added to genesis block
			wallet = chain.AddWallet("100" + chain.Network().TokenSymbol())
			return nil
		},

		// Second function runs test
		func(ctx context.Context, t *testing.T) {
			// Wait until chain is healthy
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			// Create client so we can send transactions and query state
			coredClient := chain.Client()

			// Query for current balance available on the wallet
			balances, err := coredClient.QueryBankBalances(ctx, wallet)
			require.NoError(t, err)

			// Test that wallet owns expected balance
			assert.Equal(t, "100", balances[chain.Network().TokenSymbol()].Amount.String())
		}
}

// TestCoreTransfer checks that core is transferred correctly between wallets
func TestCoreTransfer(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var sender, receiver types.Wallet

	// First function prepares initial well-known state
	return func(ctx context.Context) error {
			// Create two random wallets with predefined amounts of core
			sender = chain.AddWallet("180000100" + chain.Network().TokenSymbol())
			receiver = chain.AddWallet("10" + chain.Network().TokenSymbol())
			return nil
		},

		// Second function runs test
		func(ctx context.Context, t *testing.T) {
			// Wait until chain is healthy
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			// Create client so we can send transactions and query state
			coredClient := chain.Client()

			// Transfer 10 cores from sender to receiver
			txBytes, err := coredClient.PrepareTxBankSend(ctx, client.TxBankSendInput{
				Base: tx.BaseInput{
					Signer:   sender,
					GasLimit: chain.Network().DeterministicGas().BankSend,
					GasPrice: types.Coin{Amount: chain.Network().InitialGasPrice(), Denom: chain.Network().TokenSymbol()},
				},
				Sender:   sender,
				Receiver: receiver,
				Amount:   types.Coin{Denom: chain.Network().TokenSymbol(), Amount: big.NewInt(10)},
			})
			require.NoError(t, err)
			result, err := coredClient.Broadcast(ctx, txBytes)
			require.NoError(t, err)

			logger.Get(ctx).Info("Transfer executed", zap.String("txHash", result.TxHash))

			// Query wallets for current balance
			balancesSender, err := coredClient.QueryBankBalances(ctx, sender)
			require.NoError(t, err)

			balancesReceiver, err := coredClient.QueryBankBalances(ctx, receiver)
			require.NoError(t, err)

			// Test that tokens disappeared from sender's wallet
			// - 10core were transferred to receiver
			// - 180000000core were taken as fee
			assert.Equal(t, "90", balancesSender[chain.Network().TokenSymbol()].Amount.String())

			// Test that tokens reached receiver's wallet
			assert.Equal(t, "20", balancesReceiver[chain.Network().TokenSymbol()].Amount.String())
		}
}
