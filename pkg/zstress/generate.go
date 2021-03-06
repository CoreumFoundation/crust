package zstress

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/pkg/errors"

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
}

// Generate generates all the files required to deploy blockchain used for benchmarking
func Generate(config GenerateConfig) error {
	outDir := config.OutDirectory + "/zstress-deployment"
	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	if err := generateDocker(outDir, config.BinDirectory+"/cored"); err != nil {
		return err
	}
	if err := generateDocker(outDir, config.BinDirectory+"/zstress"); err != nil {
		return err
	}

	genesis := cored.NewGenesis(config.ChainID)
	nodeIDs := make([]string, 0, config.NumOfValidators)
	for i := 0; i < config.NumOfValidators; i++ {
		nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		nodeIDs = append(nodeIDs, cored.NodeID(nodePublicKey))
		validatorPublicKey, validatorPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		stakerPublicKey, stakerPrivateKey := cored.GenerateSecp256k1Key()

		valDir := fmt.Sprintf("%s/validators/%d", outDir, i)

		cored.NodeConfig{
			Name:           fmt.Sprintf("validator-%d", i),
			PrometheusPort: cored.DefaultPorts.Prometheus,
			NodeKey:        nodePrivateKey,
			ValidatorKey:   validatorPrivateKey,
		}.Save(valDir)

		genesis.AddWallet(stakerPublicKey, "100000000000000000000000core")
		genesis.AddValidator(validatorPublicKey, stakerPrivateKey, "100000000core")
	}
	must.OK(ioutil.WriteFile(outDir+"/validators/ids.json", must.Bytes(json.Marshal(nodeIDs)), 0o600))

	for i := 0; i < config.NumOfInstances; i++ {
		accounts := make([]cored.Secp256k1PrivateKey, 0, config.NumOfAccountsPerInstance)
		for j := 0; j < config.NumOfAccountsPerInstance; j++ {
			accountPublicKey, accountPrivateKey := cored.GenerateSecp256k1Key()
			accounts = append(accounts, accountPrivateKey)
			genesis.AddWallet(accountPublicKey, "10000000000000000000000000000core")
		}

		instanceDir := fmt.Sprintf("%s/instances/%d", outDir, i)
		must.OK(os.MkdirAll(instanceDir, 0o700))
		must.OK(ioutil.WriteFile(instanceDir+"/accounts.json", must.Bytes(json.Marshal(accounts)), 0o600))
	}

	for i := 0; i < config.NumOfValidators; i++ {
		genesis.Save(fmt.Sprintf("%s/validators/%d", outDir, i))
	}

	nodeIDs = make([]string, 0, config.NumOfSentryNodes)
	for i := 0; i < config.NumOfSentryNodes; i++ {
		nodePublicKey, nodePrivateKey, err := ed25519.GenerateKey(rand.Reader)
		must.OK(err)
		nodeIDs = append(nodeIDs, cored.NodeID(nodePublicKey))

		nodeDir := fmt.Sprintf("%s/sentry-nodes/%d", outDir, i)

		cored.NodeConfig{
			Name:           fmt.Sprintf("sentry-node-%d", i),
			PrometheusPort: cored.DefaultPorts.Prometheus,
			NodeKey:        nodePrivateKey,
		}.Save(nodeDir)

		genesis.Save(nodeDir)
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
