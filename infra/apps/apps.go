package apps

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum/pkg/config"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/infra/apps/blockexplorer"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/grafana"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/infra/apps/osmosis"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/infra/apps/prometheus"
	"github.com/CoreumFoundation/crust/infra/apps/relayercosmos"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
)

// NewFactory creates new app factory.
func NewFactory(config infra.Config, spec *infra.Spec, networkConfig config.NetworkConfig) *Factory {
	return &Factory{
		config:        config,
		spec:          spec,
		networkConfig: networkConfig,
	}
}

// Factory produces apps from config.
type Factory struct {
	config        infra.Config
	spec          *infra.Spec
	networkConfig config.NetworkConfig
}

// CoredNetwork creates new network of cored nodes.
func (f *Factory) CoredNetwork(
	namePrefix string,
	firstPorts cored.Ports,
	validatorsCount, sentriesCount int,
	binaryVersion string,
) (cored.Cored, []cored.Cored, error) {
	network := config.NewNetwork(f.networkConfig)

	wallet := cored.NewFundedWallet(&network)

	if validatorsCount > wallet.GetStakersMnemonicsCount() {
		return cored.Cored{}, nil, errors.Errorf("unsupported validators count: %d, max: %d", validatorsCount, wallet.GetStakersMnemonicsCount())
	}

	nodes := make([]cored.Cored, 0, validatorsCount+sentriesCount)
	var node0 *cored.Cored
	var lastNode cored.Cored
	var name string
	for i := 0; i < cap(nodes); i++ {
		name = namePrefix + fmt.Sprintf("-%02d", i)
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
					return wallet.GetStakersMnemonic(i)
				}
				return ""
			}(),
			StakerBalance: wallet.GetStakerMnemonicsBalance(),
			RootNode:      node0,
			ImportedMnemonics: map[string]string{
				"alice":   cored.AliceMnemonic,
				"bob":     cored.BobMnemonic,
				"charlie": cored.CharlieMnemonic,
			},
			FundingMnemonic: cored.FundingMnemonic,
			FaucetMnemonic:  cored.FaucetMnemonic,
			RelayerMnemonic: cored.RelayerMnemonic,
			BinaryVersion:   binaryVersion,
		})
		if node0 == nil {
			node0 = &node
		}
		lastNode = node
		nodes = append(nodes, node)
	}
	return lastNode, nodes, nil
}

// Faucet creates new faucet.
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

// BlockExplorer returns set of applications required to run block explorer.
func (f *Factory) BlockExplorer(name string, coredApp cored.Cored) blockexplorer.Explorer {
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
		TelemetryPort:  blockexplorer.DefaultPorts.BDJunoTelemetry,
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

	return blockexplorer.Explorer{
		Postgres:  postgresApp,
		BDJuno:    bdjunoApp,
		Hasura:    hasuraApp,
		BigDipper: bigDipperApp,
	}
}

// IBC creates set of applications required to test IBC.
func (f *Factory) IBC(name string, coredApp cored.Cored) infra.AppSet {
	nameGaia := name + "-gaia"
	nameOsmosis := name + "-osmosis"
	nameRelayerGaia := name + "-relayer-gaia"
	nameRelayerOsmosis := name + "-relayer-osmosis"

	gaiaApp := gaiad.New(cosmoschain.AppConfig{
		Name:            nameGaia,
		HomeDir:         filepath.Join(f.config.AppDir, nameGaia),
		ChainID:         gaiad.DefaultChainID,
		AppInfo:         f.spec.DescribeApp(gaiad.AppType, nameGaia),
		Ports:           gaiad.DefaultPorts,
		RelayerMnemonic: gaiad.RelayerMnemonic,
	})

	osmosisApp := osmosis.New(cosmoschain.AppConfig{
		Name:            nameOsmosis,
		HomeDir:         filepath.Join(f.config.AppDir, nameOsmosis),
		ChainID:         osmosis.DefaultChainID,
		AppInfo:         f.spec.DescribeApp(osmosis.AppType, nameOsmosis),
		Ports:           osmosis.DefaultPorts,
		RelayerMnemonic: osmosis.RelayerMnemonic,
	})

	relayerGaiaApp := relayercosmos.New(relayercosmos.Config{
		Name:        nameRelayerGaia,
		HomeDir:     filepath.Join(f.config.AppDir, nameRelayerGaia),
		AppInfo:     f.spec.DescribeApp(relayercosmos.AppType, nameRelayerGaia),
		DebugPort:   relayercosmos.DefaultDebugPort,
		Cored:       coredApp,
		PeeredChain: gaiaApp,
	})

	relayerOsmosisApp := relayercosmos.New(relayercosmos.Config{
		Name:        nameRelayerOsmosis,
		HomeDir:     filepath.Join(f.config.AppDir, nameRelayerOsmosis),
		AppInfo:     f.spec.DescribeApp(relayercosmos.AppType, nameRelayerOsmosis),
		DebugPort:   relayercosmos.DefaultDebugPort + 1,
		Cored:       coredApp,
		PeeredChain: osmosisApp,
	})

	return infra.AppSet{
		gaiaApp,
		osmosisApp,
		relayerGaiaApp,
		relayerOsmosisApp,
	}
}

// Monitoring returns set of applications required to run monitoring.
func (f *Factory) Monitoring(name string, coredNodes []cored.Cored, bdJuno bdjuno.BDJuno) infra.AppSet {
	namePrometheus := name + "-prometheus"
	nameGrafana := name + "-grafana"

	prometheusApp := prometheus.New(prometheus.Config{
		Name:       namePrometheus,
		HomeDir:    filepath.Join(f.config.AppDir, namePrometheus),
		Port:       prometheus.DefaultPort,
		AppInfo:    f.spec.DescribeApp(prometheus.AppType, namePrometheus),
		CoredNodes: coredNodes,
		BDJuno:     bdJuno,
	})

	grafanaApp := grafana.New(grafana.Config{
		Name:       nameGrafana,
		HomeDir:    filepath.Join(f.config.AppDir, nameGrafana),
		AppInfo:    f.spec.DescribeApp(grafana.AppType, nameGrafana),
		CoredNodes: coredNodes,
		Port:       grafana.DefaultPort,
		Prometheus: prometheusApp,
	})

	return infra.AppSet{
		prometheusApp,
		grafanaApp,
	}
}
