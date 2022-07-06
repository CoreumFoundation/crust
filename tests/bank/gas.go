package bank

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// TestTransferMaximumGas checks that transfer does not take more gas than assumed
func TestTransferMaximumGas(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	const margin = 1.5
	const maxGasAssumed = 100000 // set it to 50%+ higher than maximum observed value
	const numOfTransactions = 20
	amount, ok := big.NewInt(0).SetString("100000000000000000000", 10)
	if !ok {
		panic("invalid amount")
	}

	var wallet1, wallet2 cored.Wallet

	return func(ctx context.Context) error {
			wallet1 = chain.AddWallet(fmt.Sprintf("%score", amount))
			wallet2 = chain.AddWallet("1core")
			return nil
		},
		func(ctx context.Context, t *testing.T) {
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			client := chain.Client()

			var err error
			wallet1.AccountNumber, wallet1.AccountSequence, err = client.GetNumberSequence(ctx, wallet1.Key.Address())
			require.NoError(t, err)
			wallet2.AccountNumber, wallet2.AccountSequence, err = client.GetNumberSequence(ctx, wallet2.Key.Address())
			require.NoError(t, err)

			var maxGasUsed int64
			toSend := cored.Balance{Denom: "core", Amount: amount}
			send := func(sender *cored.Wallet, receiver cored.Wallet) {
				txBytes, err := client.PrepareTxBankSend(ctx, *sender, receiver, toSend)
				require.NoError(t, err)
				result, err := client.Broadcast(ctx, txBytes)
				require.NoError(t, err)

				if result.GasUsed > maxGasUsed {
					maxGasUsed = result.GasUsed
				}
				sender.AccountSequence++
			}
			for i := numOfTransactions / 2; i >= 0; i-- {
				send(&wallet1, wallet2)
				send(&wallet2, wallet1)
			}
			assert.LessOrEqual(t, margin*float64(maxGasUsed), float64(maxGasAssumed))
			logger.Get(ctx).Info("Maximum gas used", zap.Int64("maxGasUsed", maxGasUsed))
		}
}
