package zstress

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/parallel"
	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/tx"
	"github.com/CoreumFoundation/coreum/pkg/types"
)

// StressConfig contains config for benchmarking the blockchain
type StressConfig struct {
	// ChainID is the ID of the chain to connect to
	ChainID string

	// NodeAddress is the address of a cored node RPC endpoint, in the form of host:port, to connect to
	NodeAddress string

	// Accounts is the list of private keys used to send transactions during benchmark
	Accounts []types.Secp256k1PrivateKey

	// NumOfTransactions to send from each account
	NumOfTransactions int
}

type txRequest struct {
	AccountIndex int
	TxIndex      int
	From         types.Wallet
	To           types.Wallet
	TxBytes      []byte
}

// Stress runs a benchmark test
func Stress(ctx context.Context, config StressConfig, network *app.Network) error {
	log := logger.Get(ctx)
	coredClient := client.New(app.ChainID(config.ChainID), config.NodeAddress)

	log.Info("Preparing signed transactions...")
	signedTxs, initialAccountSequences, err := prepareTransactions(ctx, config, coredClient, *network)
	if err != nil {
		return err
	}
	log.Info("Transactions prepared")

	log.Info("Broadcasting transactions...")
	err = parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		const period = 10

		var mu sync.Mutex
		var txNum uint32
		var minGasUsed int64 = math.MaxInt64
		var maxGasUsed int64
		spawn("stats", parallel.Fail, func(ctx context.Context) error {
			log := logger.Get(ctx)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(period * time.Second):
					mu.Lock()
					txNumLocal := txNum
					txNum = 0
					minGasUsedLocal := minGasUsed
					maxGasUsedLocal := maxGasUsed
					mu.Unlock()

					log.Info("Stress stats",
						zap.Float32("txRate", float32(txNumLocal)/period),
						zap.Int64("minGasUsed", minGasUsedLocal),
						zap.Int64("maxGasUsed", maxGasUsedLocal))
				}
			}
		})
		spawn("accounts", parallel.Exit, func(ctx context.Context) error {
			return parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
				for i, accountTxs := range signedTxs {
					accountTxs := accountTxs
					initialSequence := initialAccountSequences[i]
					spawn(fmt.Sprintf("account-%d", i), parallel.Continue, func(ctx context.Context) error {
						for txIndex := 0; txIndex < config.NumOfTransactions; {
							tx := accountTxs[txIndex]
							result, err := coredClient.Broadcast(ctx, tx)
							if err != nil {
								if errors.Is(err, ctx.Err()) {
									return err
								}
								expectedAccSeq, ok, err2 := client.ExpectedSequenceFromError(err)
								if err2 != nil {
									return err2
								}
								if ok {
									log.Warn("Broadcasting failed, retrying with fresh account sequence...", zap.Error(err),
										zap.Uint64("accountSequence", expectedAccSeq))
									txIndex = int(expectedAccSeq - initialSequence)
								} else {
									log.Warn("Broadcasting failed, retrying...", zap.Error(err))
								}
								continue
							}
							log.Debug("Transaction broadcasted", zap.String("txHash", result.TxHash),
								zap.Int64("gasUsed", result.GasUsed))
							txIndex++

							mu.Lock()
							txNum++
							if result.GasUsed < minGasUsed {
								minGasUsed = result.GasUsed
							}
							if result.GasUsed > maxGasUsed {
								maxGasUsed = result.GasUsed
							}
							mu.Unlock()
						}
						return nil
					})
				}
				return nil
			})
		})
		return nil
	})
	if err != nil {
		return err
	}
	log.Info("Benchmark finished")
	return nil
}

func prepareTransactions(ctx context.Context, config StressConfig, coredClient client.Client, network app.Network) ([][][]byte, []uint64, error) {
	numOfAccounts := len(config.Accounts)
	var signedTxs [][][]byte
	initialAccountSequences := make([]uint64, 0, numOfAccounts)
	err := parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		queue := make(chan txRequest)
		results := make(chan txRequest)

		for i := 0; i < runtime.NumCPU(); i++ {
			spawn(fmt.Sprintf("signer-%d", i), parallel.Continue, signWorkerTask(coredClient, network, queue, results))
		}
		spawn("enqueue", parallel.Continue, enqueueTransactionsToSignTask(initialAccountSequences, config.Accounts,
			config.NumOfTransactions, coredClient, queue))
		spawn("integrate", parallel.Exit, func(ctx context.Context) error {
			signedTxs = make([][][]byte, numOfAccounts)
			for i := 0; i < numOfAccounts; i++ {
				signedTxs[i] = make([][]byte, config.NumOfTransactions)
			}
			for i := 0; i < numOfAccounts; i++ {
				for j := 0; j < config.NumOfTransactions; j++ {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case result := <-results:
						signedTxs[result.AccountIndex][result.TxIndex] = result.TxBytes
					}
				}
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return signedTxs, initialAccountSequences, nil
}

func signWorkerTask(coredClient client.Client, network app.Network, queue <-chan txRequest, results chan<- txRequest) parallel.Task {
	return func(ctx context.Context) error {
		// FIXME (wojtek): set to `network.FeeModel().InitialGasPrice` once fee model is merged
		initialGasPrice := big.NewInt(1500)
		gasPrice, err := types.NewCoin(initialGasPrice, network.TokenSymbol())
		if err != nil {
			return err
		}
		amount, err := types.NewCoin(big.NewInt(1), network.TokenSymbol())
		if err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case txReq, ok := <-queue:
				if !ok {
					return nil
				}
				txReq.TxBytes = must.Bytes(coredClient.PrepareTxBankSend(ctx, client.TxBankSendInput{
					Base: tx.BaseInput{
						Signer:   txReq.From,
						GasLimit: network.DeterministicGas().BankSend,
						GasPrice: gasPrice,
					},
					Sender:   txReq.From,
					Receiver: txReq.To,
					Amount:   amount,
				}))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case results <- txReq:
				}
			}
		}
	}
}

func enqueueTransactionsToSignTask(initialAccountSequences []uint64, privateKeys []types.Secp256k1PrivateKey,
	numOfTransactions int, coredClient client.Client, queue chan<- txRequest) parallel.Task {
	return func(ctx context.Context) error {
		numOfAccounts := len(privateKeys)

		for i := 0; i < numOfAccounts; i++ {
			fromPrivateKey := privateKeys[i]
			toPrivateKeyIndex := i + 1
			if toPrivateKeyIndex >= numOfAccounts {
				toPrivateKeyIndex = 0
			}
			toPrivateKey := privateKeys[toPrivateKeyIndex]

			accNum, accSeq, err := getAccountNumberSequence(ctx, coredClient, fromPrivateKey.Address())
			if err != nil {
				return errors.WithStack(fmt.Errorf("fetching account number and sequence failed: %w", err))
			}
			initialAccountSequences = append(initialAccountSequences, accSeq)

			tx := txRequest{
				AccountIndex: i,
				From:         types.Wallet{Name: "sender", Key: fromPrivateKey, AccountNumber: accNum, AccountSequence: accSeq},
				To:           types.Wallet{Name: "receiver", Key: toPrivateKey},
			}

			for j := 0; j < numOfTransactions; j++ {
				tx.TxIndex = j
				select {
				case <-ctx.Done():
					return ctx.Err()
				case queue <- tx:
				}
				tx.From.AccountSequence++
			}
		}
		return nil
	}
}

func getAccountNumberSequence(ctx context.Context, coredClient client.Client, accountAddress string) (uint64, uint64, error) {
	var accNum, accSeq uint64
	err := retry.Do(ctx, time.Second, func() error {
		var err error
		accNum, accSeq, err = coredClient.GetNumberSequence(ctx, accountAddress)
		if err != nil {
			return retry.Retryable(errors.Wrap(err, "querying for account number and sequence failed"))
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return accNum, accSeq, nil
}
