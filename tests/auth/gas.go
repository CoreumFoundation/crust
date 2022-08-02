package auth

import (
	"context"
	"math/big"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// TestTooLowGasPrice verifies that transaction does not enter mempool if offered gas price is below minimum level
// specified by the fee model of the network
func TestTooLowGasPrice(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var sender types.Wallet

	return func(ctx context.Context) error {
			sender = chain.AddWallet("180000100" + chain.Network().TokenSymbol())
			return nil
		},
		func(ctx context.Context, t *testing.T) {
			testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

			client := chain.Client()

			gasPrice := big.NewInt(0).Sub(chain.Network().MinDiscountedGasPrice(), big.NewInt(1))
			txBytes, err := client.PrepareTxBankSend(ctx, cored.TxBankSendInput{
				Base: cored.BaseInput{
					Signer:   sender,
					GasLimit: chain.Network().DeterministicGas().BankSend,
					GasPrice: types.Coin{Amount: gasPrice, Denom: chain.Network().TokenSymbol()},
				},
				Sender:   sender,
				Receiver: sender,
				Amount:   types.Coin{Denom: chain.Network().TokenSymbol(), Amount: big.NewInt(10)},
			})
			require.NoError(t, err)

			// Broadcast should fail because gas price is too low for transaction to enter mempool
			_, err = client.Broadcast(ctx, txBytes)
			require.True(t, cored.IsInsufficientFeeError(err))
		}
}
