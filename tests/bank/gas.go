package bank

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

var maxMemo = strings.Repeat("-", 256) // cosmos sdk is configured to accept maximum memo of 256 characters by default

// TestTransferMaximumGas checks that transfer does not take more gas than assumed
func TestTransferMaximumGas(chain cored.Cored, numOfTransactions int) (testing.PrepareFunc, testing.RunFunc) {
	const margin = 1.5
	const maxGasAssumed = 100000 // set it to 50%+ higher than maximum observed value

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
			for i := numOfTransactions / 2; i >= 0; i-- {
				gasUsed, err := sendAndReturnGasUsed(ctx, client, wallet1, wallet2, toSend)
				require.NoError(t, err)

				if gasUsed > maxGasUsed {
					maxGasUsed = gasUsed
				}

				gasUsed, err = sendAndReturnGasUsed(ctx, client, wallet2, wallet1, toSend)
				require.NoError(t, err)

				if gasUsed > maxGasUsed {
					maxGasUsed = gasUsed
				}

				wallet1.AccountSequence++
				wallet2.AccountSequence++
			}
			assert.LessOrEqual(t, margin*float64(maxGasUsed), float64(maxGasAssumed))
			logger.Get(ctx).Info("Maximum gas used", zap.Int64("maxGasUsed", maxGasUsed))
		}
}

func sendAndReturnGasUsed(ctx context.Context, client cored.Client, sender, receiver cored.Wallet, toSend cored.Balance) (int64, error) {
	txBytes, err := client.PrepareTxBankSend(ctx, cored.TxBankSendData{
		Sender:   sender,
		Receiver: receiver,
		Balance:  toSend,
		Memo:     maxMemo, // memo is set to max length here to charge as much gas as possible
	})
	if err != nil {
		return 0, err
	}
	result, err := client.Broadcast(ctx, txBytes)
	if err != nil {
		return 0, err
	}
	return result.GasUsed, nil
}
