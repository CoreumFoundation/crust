package zstress

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"
	tmed25519 "github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/client"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
)

// GenerateConfig contains config for generating the blockchain
type GenerateConfig struct {
	// ChainID is the ID of the chain to generate
	ChainID string

	// NumOfValidators is the number of validators present on the blockchain
	NumOfValidators int

	// NumOfSentryNodes is the number of sentry nodes to generate config for
	NumOfSentryNodes int

	// NumOfInstances is the maximum number of application instances used in the future during benchmarking
	NumOfInstances int

	// NumOfAccountsPerInstance is the maximum number of funded accounts per each instance used in the future during benchmarking
	NumOfAccountsPerInstance int

	// BinDirectory is the path to the directory where binaries exist
	BinDirectory string

	// OutDirectory is the path to the directory where generated files are stored
	OutDirectory string

	// Network is the cored network config
	Network app.Network
}

func nodeIDFromPubKey(pubKey ed25519.PublicKey) string {
	return hex.EncodeToString(tmed25519.PubKey(pubKey).Address())
}

// Generate generates all the files required to deploy blockchain used for benchmarking
func Generate(cfg GenerateConfig) error {
	outDir := cfg.OutDirectory + "/zstress-deployment"
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	if err := generateDocker(outDir, cfg.BinDirectory+"/cored"); err != nil {
		return err
	}
	if err := generateDocker(outDir, cfg.BinDirectory+"/zstress"); err != nil {
		return err
	}

	network := cfg.Network
	clientCtx := app.NewDefaultClientContext().WithChainID(string(network.ChainID()))
	nodeIDs := make([]string, 0, cfg.NumOfValidators)
	for i := 0; i < cfg.NumOfValidators; i++ {
		nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		nodeIDs = append(nodeIDs, cored.NodeID(nodePublicKey))
		validatorPublicKey, validatorPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		stakerPublicKey, stakerPrivateKey := types.GenerateSecp256k1Key()

		valDir := fmt.Sprintf("%s/validators/%d", outDir, i)

		nodeConfig := app.NodeConfig{
			Name:           fmt.Sprintf("validator-%d", i),
			PrometheusPort: cored.DefaultPorts.Prometheus,
			NodeKey:        nodePrivateKey,
			ValidatorKey:   validatorPrivateKey,
		}
		cored.SaveConfig(nodeConfig, valDir)
		must.OK(err)

		err = network.FundAccount(stakerPublicKey, "100000000000000000000000"+network.TokenSymbol())
		must.OK(err)
		tx, err := client.PrepareTxStakingCreateValidator(clientCtx, validatorPublicKey, stakerPrivateKey, "100000000"+network.TokenSymbol())
		must.OK(err)
		network.AddGenesisTx(tx)
	}
	must.OK(ioutil.WriteFile(outDir+"/validators/ids.json", must.Bytes(json.Marshal(nodeIDs)), 0o600))

	for i := 0; i < cfg.NumOfInstances; i++ {
		accounts := make([]types.Secp256k1PrivateKey, 0, cfg.NumOfAccountsPerInstance)
		for j := 0; j < cfg.NumOfAccountsPerInstance; j++ {
			accountPublicKey, accountPrivateKey := types.GenerateSecp256k1Key()
			accounts = append(accounts, accountPrivateKey)
			err := network.FundAccount(accountPublicKey, "10000000000000000000000000000"+network.TokenSymbol())
			must.OK(err)
		}

		instanceDir := fmt.Sprintf("%s/instances/%d", outDir, i)
		must.OK(os.MkdirAll(instanceDir, 0o700))
		must.OK(ioutil.WriteFile(instanceDir+"/accounts.json", must.Bytes(json.Marshal(accounts)), 0o600))
	}

	for i := 0; i < cfg.NumOfValidators; i++ {
		err := network.SaveGenesis(fmt.Sprintf("%s/validators/%d", outDir, i))
		must.OK(err)
	}

	nodeIDs = make([]string, 0, cfg.NumOfSentryNodes)
	for i := 0; i < cfg.NumOfSentryNodes; i++ {
		nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		nodeIDs = append(nodeIDs, nodeIDFromPubKey(nodePublicKey))

		nodeDir := fmt.Sprintf("%s/sentry-nodes/%d", outDir, i)

		nodeConfig := app.NodeConfig{
			Name:           fmt.Sprintf("sentry-node-%d", i),
			PrometheusPort: cored.DefaultPorts.Prometheus,
			NodeKey:        nodePrivateKey,
		}
		cored.SaveConfig(nodeConfig, nodeDir)
		must.OK(err)

		err = network.SaveGenesis(nodeDir)
		must.OK(err)
	}
	must.OK(ioutil.WriteFile(outDir+"/sentry-nodes/ids.json", must.Bytes(json.Marshal(nodeIDs)), 0o600))
	return nil
}

func generateDocker(outDir, toolPath string) error {
	toolName := filepath.Base(toolPath)
	dockerDir := outDir + "/docker-" + toolName
	dockerDirBin := dockerDir + "/bin"
	must.OK(os.MkdirAll(dockerDirBin, 0o700))
	if err := os.Link(toolPath, dockerDirBin+"/"+toolName); err != nil {
		return errors.Wrapf(err, `can't find %[1]s binary, run "crust build/%[1]s" to build it`, toolName)
	}

	must.OK(ioutil.WriteFile(dockerDir+"/Dockerfile", []byte(`FROM alpine:3.16.0
COPY . .
ENTRYPOINT ["`+toolName+`"]
`), 0o600))

	return nil
}
