package apps

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/app"
	"github.com/CoreumFoundation/coreum/pkg/types"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/infra/apps/blockexplorer"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/infra/testing"
)

// NewFactory creates new app factory
func NewFactory(config infra.Config, spec *infra.Spec, networkConfig app.NetworkConfig) *Factory {
	return &Factory{
		config:        config,
		spec:          spec,
		networkConfig: networkConfig,
	}
}

// Factory produces apps from config
type Factory struct {
	config        infra.Config
	spec          *infra.Spec
	networkConfig app.NetworkConfig
}

// CoredNetwork creates new network of cored nodes
func (f *Factory) CoredNetwork(name string, validatorMnemonics []string, numOfSentryNodes int) (cored.Cored, infra.Mode, error) {
	network := app.NewNetwork(f.networkConfig)
	initialBalance := "500000000000000" + network.TokenSymbol()

	alicePrivKey, err := cored.PrivateKeyFromMnemonic(cored.AliceMnemonic)
	if err != nil {
		return cored.Cored{}, nil, errors.WithStack(err)
	}
	bobPrivKey, err := cored.PrivateKeyFromMnemonic(cored.BobMnemonic)
	if err != nil {
		return cored.Cored{}, nil, errors.WithStack(err)
	}
	charliePrivKey, err := cored.PrivateKeyFromMnemonic(cored.CharlieMnemonic)
	if err != nil {
		return cored.Cored{}, nil, errors.WithStack(err)
	}
	faucetPrivKey, err := cored.PrivateKeyFromMnemonic(faucet.PrivateKeyMnemonic)
	if err != nil {
		return cored.Cored{}, nil, errors.WithStack(err)
	}
	testsFundingPrivKey, err := cored.PrivateKeyFromMnemonic(testing.FundingMnemonic)
	if err != nil {
		return cored.Cored{}, nil, errors.WithStack(err)
	}

	must.OK(network.FundAccount(alicePrivKey.PubKey(), initialBalance))
	must.OK(network.FundAccount(bobPrivKey.PubKey(), initialBalance))
	must.OK(network.FundAccount(charliePrivKey.PubKey(), initialBalance))
	must.OK(network.FundAccount(faucetPrivKey.PubKey(), initialBalance))
	must.OK(network.FundAccount(testsFundingPrivKey.PubKey(), initialBalance))

	wallets := map[string]types.Secp256k1PrivateKey{
		"alice":   alicePrivKey,
		"bob":     bobPrivKey,
		"charlie": charliePrivKey,
	}

	nodes := make(infra.Mode, 0, len(validatorMnemonics)+numOfSentryNodes)
	var node0 *cored.Cored
	var lastNode cored.Cored
	for i := 0; i < cap(nodes); i++ {
		name := name + fmt.Sprintf("-%02d", i)
		portDelta := i * 100
		node := cored.New(cored.Config{
			Name:       name,
			HomeDir:    filepath.Join(f.config.AppDir, name, string(network.ChainID())),
			BinDir:     f.config.BinDir,
			WrapperDir: f.config.WrapperDir,
			Network:    &network,
			AppInfo:    f.spec.DescribeApp(cored.AppType, name),
			Ports: cored.Ports{
				RPC:        cored.DefaultPorts.RPC + portDelta,
				P2P:        cored.DefaultPorts.P2P + portDelta,
				GRPC:       cored.DefaultPorts.GRPC + portDelta,
				GRPCWeb:    cored.DefaultPorts.GRPCWeb + portDelta,
				PProf:      cored.DefaultPorts.PProf + portDelta,
				Prometheus: cored.DefaultPorts.Prometheus + portDelta,
			},
			IsValidator:       i < len(validatorMnemonics),
			ValidatorMnemonic: validatorMnemonics[i],
			RootNode:          node0,
			Wallets:           wallets,
		})
		if node0 == nil {
			node0 = &node
		}
		lastNode = node
		nodes = append(nodes, node)
	}
	return lastNode, nodes, nil
}

// Faucet creates new faucet
func (f *Factory) Faucet(name string, coredApp cored.Cored) (faucet.Faucet, error) {
	privKey, err := cored.PrivateKeyFromMnemonic(faucet.PrivateKeyMnemonic)
	if err != nil {
		return faucet.Faucet{}, errors.WithStack(err)
	}

	return faucet.New(faucet.Config{
		Name:       name,
		HomeDir:    filepath.Join(f.config.AppDir, name),
		BinDir:     f.config.BinDir,
		ChainID:    f.networkConfig.ChainID,
		AppInfo:    f.spec.DescribeApp(faucet.AppType, name),
		Port:       faucet.DefaultPort,
		PrivateKey: privKey,
		Cored:      coredApp,
	}), nil
}

// BlockExplorer returns set of applications required to run block explorer
func (f *Factory) BlockExplorer(name string, coredApp cored.Cored) infra.Mode {
	namePostgres := name + "-postgres"
	nameHasura := name + "-hasura"
	nameBDJuno := name + "-bdjuno"
	nameBigDipper := name + "-bigdipper"

	postgresApp := postgres.New(postgres.Config{
		Name:             namePostgres,
		AppInfo:          f.spec.DescribeApp(postgres.AppType, namePostgres),
		Port:             blockexplorer.DefaultPorts.Postgres,
		SchemaLoaderFunc: blockexplorer.LoadPostgresSchema,
	})
	hasuraApp := hasura.New(hasura.Config{
		Name:     nameHasura,
		AppInfo:  f.spec.DescribeApp(hasura.AppType, nameHasura),
		Port:     blockexplorer.DefaultPorts.Hasura,
		Postgres: postgresApp,
	})
	bdjunoApp := bdjuno.New(bdjuno.Config{
		Name:           nameBDJuno,
		HomeDir:        filepath.Join(f.config.AppDir, nameBDJuno),
		AppInfo:        f.spec.DescribeApp(bdjuno.AppType, nameBDJuno),
		Port:           blockexplorer.DefaultPorts.BDJuno,
		ConfigTemplate: blockexplorer.BDJunoConfigTemplate,
		Cored:          coredApp,
		Postgres:       postgresApp,
	})
	bigDipperApp := bigdipper.New(bigdipper.Config{
		Name:    nameBigDipper,
		AppInfo: f.spec.DescribeApp(bigdipper.AppType, nameBigDipper),
		Port:    blockexplorer.DefaultPorts.BigDipper,
		Cored:   coredApp,
		Hasura:  hasuraApp,
	})

	return infra.Mode{
		postgresApp,
		hasuraApp,
		bdjunoApp,
		bigDipperApp,
	}
}
