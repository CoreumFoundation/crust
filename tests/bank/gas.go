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
	// FIXME (wojtek): Take this value from Network.TxBankSendGas() once Milad integrates it into crust
	const maxGasAssumed = 120000 // set it to 50%+ higher than maximum observed value

	amount, ok := big.NewInt(0).SetString("100000000000000000000000000000000000", 10)
	if !ok {
		panic("invalid amount")
	}

	// FIXME (wojciech): Compute fee based on Network once Milad integrates it into crust
	fees, ok := big.NewInt(0).SetString("180000000", 10)
	if !ok {
		panic("invalid amount")
	}
	fees.Mul(fees, big.NewInt(int64(numOfTransactions)))

	var wallet1, wallet2 cored.Wallet

	return func(ctx context.Context) error {
			wallet1 = chain.AddWallet(fmt.Sprintf("%score", big.NewInt(0).Add(fees, amount)))
			wallet2 = chain.AddWallet(fmt.Sprintf("%score", fees))
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
			toSend := cored.Coin{Denom: "core", Amount: amount}
			for i, sender, receiver := numOfTransactions, wallet1, wallet2; i >= 0; i, sender, receiver = i-1, receiver, sender {
				gasUsed, err := sendAndReturnGasUsed(ctx, client, sender, receiver, toSend, maxGasAssumed)
				if !assert.NoError(t, err) {
					break
				}

				if gasUsed > maxGasUsed {
					maxGasUsed = gasUsed
				}
				sender.AccountSequence++
			}
			assert.LessOrEqual(t, margin*float64(maxGasUsed), float64(maxGasAssumed))
			logger.Get(ctx).Info("Maximum gas used", zap.Int64("maxGasUsed", maxGasUsed))
		}
}

func sendAndReturnGasUsed(ctx context.Context, client cored.Client, sender, receiver cored.Wallet, toSend cored.Coin, gasLimit uint64) (int64, error) {
	txBytes, err := client.PrepareTxBankSend(ctx, cored.TxBankSendInput{
		Signing: cored.SignInput{
			Signer:   sender,
			GasLimit: gasLimit,
			// FIXME (wojtek): Take this value from Network.InitialGasPrice() once Milad integrates it into crust
			GasPrice: cored.Coin{Amount: big.NewInt(1500), Denom: "core"},
			Memo:     maxMemo, // memo is set to max length here to charge as much gas as possible
		},
		Sender:   sender,
		Receiver: receiver,
		Amount:   toSend,
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
