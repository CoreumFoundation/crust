package apps

import (
	"fmt"
	"path/filepath"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/infra/apps/blockexplorer"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/infra/apps/relayer"
)

// NewFactory creates new app factory
func NewFactory(config infra.Config, spec *infra.Spec, networkConfig config.NetworkConfig) *Factory {
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
	networkConfig config.NetworkConfig
}

// CoredNetwork creates new network of cored nodes
func (f *Factory) CoredNetwork(name string, firstPorts cored.Ports, validatorsCount int, sentriesCount int) (cored.Cored, infra.AppSet, error) {
	if validatorsCount > len(cored.StakerMnemonics) {
		return cored.Cored{}, nil, errors.Errorf("unsupported validators count: %d, max: %d", validatorsCount, len(cored.StakerMnemonics))
	}

	network := config.NewNetwork(f.networkConfig)
	initialBalance := sdk.NewCoins(sdk.NewInt64Coin(f.networkConfig.Denom, 500_000_000_000_000))

	for _, mnemonic := range []string{
		cored.AliceMnemonic,
		cored.BobMnemonic,
		cored.CharlieMnemonic,
		cored.FaucetMnemonic,
		cored.FundingMnemonic,
		cored.RelayerMnemonic,
	} {
		privKey, err := cored.PrivateKeyFromMnemonic(mnemonic)
		if err != nil {
			return cored.Cored{}, nil, errors.WithStack(err)
		}
		must.OK(network.FundAccount(sdk.AccAddress(privKey.PubKey().Address()), initialBalance))
	}

	nodes := make(infra.AppSet, 0, validatorsCount+sentriesCount)
	var node0 *cored.Cored
	var lastNode cored.Cored
	for i := 0; i < cap(nodes); i++ {
		name := name + fmt.Sprintf("-%02d", i)
		portDelta := i * 100
		isValidator := i < validatorsCount
		node := cored.New(cored.Config{
			Name:       name,
			HomeDir:    filepath.Join(f.config.AppDir, name, string(network.ChainID())),
			BinDir:     f.config.BinDir,
			WrapperDir: f.config.WrapperDir,
			Network:    &network,
			AppInfo:    f.spec.DescribeApp(cored.AppType, name),
			Ports: cored.Ports{
				RPC:        firstPorts.RPC + portDelta,
				P2P:        firstPorts.P2P + portDelta,
				GRPC:       firstPorts.GRPC + portDelta,
				GRPCWeb:    firstPorts.GRPCWeb + portDelta,
				API:        firstPorts.API + portDelta,
				PProf:      firstPorts.PProf + portDelta,
				Prometheus: firstPorts.Prometheus + portDelta,
			},
			IsValidator: isValidator,
			StakerMnemonic: func() string {
				if isValidator {
					return cored.StakerMnemonics[i]
				}
				return ""
			}(),
			RootNode: node0,
			ImportedMnemonics: map[string]string{
				"alice":   cored.AliceMnemonic,
				"bob":     cored.BobMnemonic,
				"charlie": cored.CharlieMnemonic,
			},
			FundingMnemonic: cored.FundingMnemonic,
			FaucetMnemonic:  cored.FaucetMnemonic,
			RelayerMnemonic: cored.RelayerMnemonic,
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
func (f *Factory) Faucet(name string, coredApp cored.Cored) faucet.Faucet {
	return faucet.New(faucet.Config{
		Name:    name,
		HomeDir: filepath.Join(f.config.AppDir, name),
		BinDir:  f.config.BinDir,
		ChainID: f.networkConfig.ChainID,
		AppInfo: f.spec.DescribeApp(faucet.AppType, name),
		Port:    faucet.DefaultPort,
		Cored:   coredApp,
	})
}

// BlockExplorer returns set of applications required to run block explorer
func (f *Factory) BlockExplorer(name string, coredApp cored.Cored) infra.AppSet {
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
	bdjunoApp := bdjuno.New(bdjuno.Config{
		Name:           nameBDJuno,
		HomeDir:        filepath.Join(f.config.AppDir, nameBDJuno),
		AppInfo:        f.spec.DescribeApp(bdjuno.AppType, nameBDJuno),
		Port:           blockexplorer.DefaultPorts.BDJuno,
		ConfigTemplate: blockexplorer.BDJunoConfigTemplate,
		Cored:          coredApp,
		Postgres:       postgresApp,
	})
	hasuraApp := hasura.New(hasura.Config{
		Name:     nameHasura,
		AppInfo:  f.spec.DescribeApp(hasura.AppType, nameHasura),
		Port:     blockexplorer.DefaultPorts.Hasura,
		Postgres: postgresApp,
		BDJuno:   bdjunoApp,
	})
	bigDipperApp := bigdipper.New(bigdipper.Config{
		Name:    nameBigDipper,
		AppInfo: f.spec.DescribeApp(bigdipper.AppType, nameBigDipper),
		Port:    blockexplorer.DefaultPorts.BigDipper,
		Cored:   coredApp,
		Hasura:  hasuraApp,
	})

	return infra.AppSet{
		postgresApp,
		hasuraApp,
		bdjunoApp,
		bigDipperApp,
	}
}

// Gaia creates new gaia.
func (f *Factory) Gaia(name string) gaiad.Gaia {
	return gaiad.New(gaiad.Config{
		Name:            name,
		HomeDir:         filepath.Join(f.config.AppDir, name),
		ChainID:         gaiad.DefaultChainID,
		AccountPrefix:   gaiad.DefaultAccountPrefix,
		AppInfo:         f.spec.DescribeApp(gaiad.AppType, name),
		Ports:           gaiad.DefaultPorts,
		RelayerMnemonic: gaiad.RelayerMnemonic,
	})
}

// Relayer creates new relayer.
func (f *Factory) Relayer(name string, coredApp cored.Cored, gaiaApp gaiad.Gaia) relayer.Relayer {
	return relayer.New(relayer.Config{
		Name:      name,
		HomeDir:   filepath.Join(f.config.AppDir, name),
		AppInfo:   f.spec.DescribeApp(relayer.AppType, name),
		DebugPort: relayer.DefaultDebugPort,
		Cored:     coredApp,
		Gaia:      gaiaApp,
	})
}
