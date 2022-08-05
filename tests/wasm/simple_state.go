package wasm

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/testing"
	"github.com/CoreumFoundation/crust/pkg/contracts"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	_ "embed"
)

var (
	//go:embed test_fixtures/simple-state/artifacts/simple_state.wasm
	simpleStateWASM []byte
)

// TestSimpleStateContract runs a contract deployment flow and tries to modify the state after deployment.
// This is a E2E check for the WASM integration, to ensure it works for a simple state contract (Counter).
func TestSimpleStateContract(chain cored.Cored) (testing.PrepareFunc, testing.RunFunc) {
	var adminWallet types.Wallet
	var networkConfig contracts.ChainConfig
	var stagedContractPath string

	minGasPrice := chain.Network().InitialGasPrice()
	nativeDenom := chain.Network().TokenSymbol()
	nativeTokens := func(v string) string {
		return v + nativeDenom
	}

	initTestState := func(ctx context.Context) error {
		adminWallet = chain.AddWallet(nativeTokens("100000000000000000000000000000000000"))
		networkConfig = contracts.ChainConfig{
			ChainID: string(chain.Network().ChainID()),
			// FIXME: Take this value from Network.InitialGasPrice() once Milad integrates it into crust
			MinGasPrice: nativeTokens(minGasPrice.String()),
			RPCEndpoint: infra.JoinNetAddr("", chain.Info().HostFromHost, chain.Ports().RPC),
		}

		// FIXME: if workdir for the test is fixed, we can avoid embedding & staging
		// the artefacts. Should be just referencing the local file.

		stagedContractsDir := filepath.Join(os.TempDir(), "crust", "wasm", "artifacts")
		if err := os.MkdirAll(stagedContractsDir, 0700); err != nil {
			err = errors.Wrap(err, "failed to init the WASM staging dig")
			return err
		}

		stagedContractPath = filepath.Join(stagedContractsDir, "simple_state.wasm")
		if err := ioutil.WriteFile(stagedContractPath, simpleStateWASM, 0600); err != nil {
			err = errors.Wrap(err, "failed to stage the WASM contract for the test")
			return err
		}

		return nil
	}

	runTestFunc := func(ctx context.Context, t *testing.T) {
		expect := require.New(t)

		testing.WaitUntilHealthy(ctx, t, 20*time.Second, chain)

		deployOut, err := contracts.Deploy(ctx, contracts.DeployConfig{
			Network: networkConfig,
			From:    adminWallet,

			ArtefactPath: stagedContractPath,
			NeedRebuild:  false,
		})
		expect.NoError(err)
		expect.NotEmpty(deployOut.StoreTxHash)

		deployOut, err = contracts.Deploy(ctx, contracts.DeployConfig{
			Network: networkConfig,
			From:    adminWallet,

			ArtefactPath: stagedContractPath,
			InstantiationConfig: contracts.ContractInstanceConfig{
				NeedInstantiation:  true,
				InstantiatePayload: `{"count": 1337}`,
			},
		})
		expect.NoError(err)
		expect.NotEmpty(deployOut.InitTxHash)
		expect.NotEmpty(deployOut.ContractAddr)

		queryOut, err := contracts.Query(ctx, deployOut.ContractAddr, contracts.QueryConfig{
			Network:      networkConfig,
			QueryPayload: `{"get_count": {}}`,
		})
		expect.NoError(err)

		response := simpleStateQueryResponse{}
		err = json.Unmarshal(queryOut.Result, &response)
		expect.NoError(err)
		expect.Equal(1337, response.Count)

		execOut, err := contracts.Execute(ctx, deployOut.ContractAddr, contracts.ExecuteConfig{
			Network:        networkConfig,
			From:           adminWallet,
			ExecutePayload: `{"increment": {}}`,
		})
		expect.NoError(err)
		expect.NotEmpty(execOut.ExecuteTxHash)
		expect.Equal(deployOut.ContractAddr, execOut.ContractAddress)
		expect.Equal("try_increment", execOut.MethodExecuted)

		queryOut, err = contracts.Query(ctx, deployOut.ContractAddr, contracts.QueryConfig{
			Network:      networkConfig,
			QueryPayload: `{"get_count": {}}`,
		})
		expect.NoError(err)

		response = simpleStateQueryResponse{}
		err = json.Unmarshal(queryOut.Result, &response)
		expect.NoError(err)
		expect.Equal(1338, response.Count)
	}

	return initTestState, runTestFunc
}

type simpleStateQueryResponse struct {
	Count int `json:"count"`
}
