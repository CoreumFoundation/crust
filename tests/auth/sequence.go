package auth

import (
	"context"
	"math/big"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// TestUnexpectedSequenceNumber test verifies that we correctly handle error reporting invalid account sequence number
// used to sign transaction
func TestUnexpectedSequenceNumber(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var sender cored.Wallet

	return func(ctx context.Context) error {
			sender = chain.AddWallet("180000010core")
			return nil
		},
		func(ctx context.Context, t *testing.T) {
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			client := chain.Client()

			accNum, accSeq, err := client.GetNumberSequence(ctx, sender.Key.Address())
			require.NoError(t, err)

			sender.AccountNumber = accNum
			sender.AccountSequence = accSeq + 1 // Intentionally set incorrect sequence number

			// Broadcast a transaction using incorrect sequence number
			txBytes, err := client.PrepareTxBankSend(ctx, cored.TxBankSendInput{
				Base: cored.BaseInput{
					Signer: sender,
					// FIXME (wojtek): Take this value from Network.TxBankSendGas() once Milad integrates it into crust
					GasLimit: 120000,
					// FIXME (wojtek): Take this value from Network.InitialGasPrice() once Milad integrates it into crust
					GasPrice: cored.Coin{Amount: big.NewInt(1500), Denom: "core"},
				},
				Sender:   sender,
				Receiver: sender,
				Amount:   cored.Coin{Denom: "core", Amount: big.NewInt(1)},
			})
			require.NoError(t, err)
			_, err = client.Broadcast(ctx, txBytes)
			require.Error(t, err) // We expect error

			// We expect that we get an error saying what the correct sequence number should be
			expectedSeq, ok, err2 := cored.ExpectedSequenceFromError(err)
			require.NoError(t, err2)
			if !ok {
				require.Fail(t, "Unexpected error", err.Error())
			}
			require.Equal(t, accSeq, expectedSeq)
		}
}
