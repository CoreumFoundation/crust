package auth

import (
	"context"
	"math/big"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// TestTooLowGasPrice verifies that transaction does not enter mempool if offered gas price is below minimum level
// specified by the fee model of the network
func TestTooLowGasPrice(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var sender cored.Wallet

	return func(ctx context.Context) error {
			sender = chain.AddWallet("180000100core")
			return nil
		},
		func(ctx context.Context, t *testing.T) {
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			client := chain.Client()

			txBytes, err := client.PrepareTxBankSend(ctx, cored.TxBankSendInput{
				Base: cored.BaseInput{
					Signer: sender,
					// FIXME (wojtek): Take this value from Network.TxBankSendGas() once Milad integrates it into crust
					GasLimit: 120000,
					// FIXME (wojtek): Take this value from Network.MinDiscountedGasPrice()-1 once Milad integrates it into crust
					// Offer gas price lower by exactly 1 unit than the minimum which should be accepted into mempool
					GasPrice: cored.Coin{Amount: big.NewInt(999), Denom: "core"},
				},
				Sender:   sender,
				Receiver: sender,
				Amount:   cored.Coin{Denom: "core", Amount: big.NewInt(10)},
			})
			require.NoError(t, err)

			// Broadcast should fail because gas price is too low for transaction to enter mempool
			_, err = client.Broadcast(ctx, txBytes)
			require.True(t, cored.IsInsufficientFeeError(err))

		}
}
