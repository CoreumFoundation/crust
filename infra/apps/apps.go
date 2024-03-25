package apps

import (
	"fmt"
	"path/filepath"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum/v4/pkg/config/constant"
	"github.com/CoreumFoundation/crust/infra"
	"github.com/CoreumFoundation/crust/infra/apps/bdjuno"
	"github.com/CoreumFoundation/crust/infra/apps/bigdipper"
	"github.com/CoreumFoundation/crust/infra/apps/blockexplorer"
	"github.com/CoreumFoundation/crust/infra/apps/bridgexrpl"
	"github.com/CoreumFoundation/crust/infra/apps/cored"
	"github.com/CoreumFoundation/crust/infra/apps/faucet"
	"github.com/CoreumFoundation/crust/infra/apps/gaiad"
	"github.com/CoreumFoundation/crust/infra/apps/grafana"
	"github.com/CoreumFoundation/crust/infra/apps/hasura"
	"github.com/CoreumFoundation/crust/infra/apps/hermes"
	"github.com/CoreumFoundation/crust/infra/apps/osmosis"
	"github.com/CoreumFoundation/crust/infra/apps/postgres"
	"github.com/CoreumFoundation/crust/infra/apps/prometheus"
	"github.com/CoreumFoundation/crust/infra/apps/xrpl"
	"github.com/CoreumFoundation/crust/infra/cosmoschain"
)

// NewFactory creates new app factory.
func NewFactory(config infra.Config, spec *infra.Spec) *Factory {
	return &Factory{
		config: config,
		spec:   spec,
	}
}

// Factory produces apps from config.
type Factory struct {
	config infra.Config
	spec   *infra.Spec
}

// CoredNetwork creates new network of cored nodes.
//
//nolint:funlen // breaking down this function will make it less readable.
func (f *Factory) CoredNetwork(
	genesisConfig cored.GenesisInitConfig,
	namePrefix string,
	firstPorts cored.Ports,
	validatorCount, sentryCount, seedCount, fullCount int,
	binaryVersion string,
) (cored.Cored, []cored.Cored, error) {
	config := sdk.GetConfig()
	addressPrefix := genesisConfig.AddressPrefix

	// Set address & public key prefixes
	config.SetBech32PrefixForAccount(addressPrefix, addressPrefix+"pub")
	config.SetBech32PrefixForValidator(addressPrefix+"valoper", addressPrefix+"valoperpub")
	config.SetBech32PrefixForConsensusNode(addressPrefix+"valcons", addressPrefix+"valconspub")
	config.SetCoinType(constant.CoinType)

	wallet, genesisConfig := cored.NewFundedWallet(genesisConfig)

	if validatorCount > wallet.GetStakersMnemonicsCount() {
		return cored.Cored{}, nil, errors.Errorf(
			"unsupported validators count: %d, max: %d",
			validatorCount,
			wallet.GetStakersMnemonicsCount(),
		)
	}

	nodes := make([]cored.Cored, 0, validatorCount+seedCount+sentryCount+fullCount)
	valNodes := make([]cored.Cored, 0, validatorCount)
	seedNodes := make([]cored.Cored, 0, seedCount)
	var lastNode cored.Cored
	var name string
	for i := 0; i < cap(nodes); i++ {
		portDelta := i * 100
		isValidator := i < validatorCount
		isSeed := !isValidator && i < validatorCount+seedCount
		isSentry := !isValidator && !isSeed && i < validatorCount+seedCount+sentryCount
		isFull := !isValidator && !isSeed && !isSentry

		name = namePrefix + fmt.Sprintf("-%02d", i)
		switch {
		case isValidator:
			name += "-val"
		case isSentry:
			name += "-sentry"
		case isSeed:
			name += "-seed"
		default:
			name += "-full"
		}

		node := cored.New(cored.Config{
			Name:              name,
			HomeDir:           filepath.Join(f.config.AppDir, name, string(genesisConfig.ChainID)),
			BinDir:            filepath.Join(f.config.RootDir, "coreum", "bin"),
			WrapperDir:        f.config.WrapperDir,
			GenesisInitConfig: &genesisConfig,
			AppInfo:           f.spec.DescribeApp(cored.AppType, name),
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
			ValidatorNodes: func() []cored.Cored {
				if isSentry || sentryCount == 0 {
					return valNodes
				}

				return nil
			}(),
			SeedNodes: func() []cored.Cored {
				if isSentry || isFull {
					return seedNodes
				}

				return nil
			}(),
			ImportedMnemonics: map[string]string{
				"alice":      cored.AliceMnemonic,
				"bob":        cored.BobMnemonic,
				"charlie":    cored.CharlieMnemonic,
				"xrplbridge": bridgexrpl.CoreumAdminMnemonic,
			},
			FundingMnemonic: cored.FundingMnemonic,
			FaucetMnemonic:  cored.FaucetMnemonic,
			GasPriceStr:     cored.DefaultGasPriceStr,
			BinaryVersion:   binaryVersion,
			TimeoutCommit:   f.spec.TimeoutCommit,
		})
		if isValidator {
			valNodes = append(valNodes, node)
		}
		if isSeed {
			seedNodes = append(seedNodes, node)
		}

		lastNode = node
		nodes = append(nodes, node)
	}
	return lastNode, nodes, nil
}

// Faucet creates new faucet.
func (f *Factory) Faucet(name string, coredApp cored.Cored) faucet.Faucet {
	return faucet.New(faucet.Config{
		Name:           name,
		HomeDir:        filepath.Join(f.config.AppDir, name),
		AppInfo:        f.spec.DescribeApp(faucet.AppType, name),
		Port:           faucet.DefaultPort,
		MonitoringPort: faucet.DefaultMonitoringPort,
		Cored:          coredApp,
	})
}

// BlockExplorer returns set of applications required to run block explorer.
func (f *Factory) BlockExplorer(prefix string, coredApp cored.Cored) blockexplorer.Explorer {
	namePostgres := BuildPrefixedAppName(prefix, string(postgres.AppType))
	nameHasura := BuildPrefixedAppName(prefix, string(hasura.AppType))
	nameBDJuno := BuildPrefixedAppName(prefix, string(bdjuno.AppType))
	nameBigDipper := BuildPrefixedAppName(prefix, string(bigdipper.AppType))

	postgresApp := postgres.New(postgres.Config{
		Name:    namePostgres,
		AppInfo: f.spec.DescribeApp(postgres.AppType, namePostgres),
		Port:    blockexplorer.DefaultPorts.Postgres,
	})
	bdjunoApp := bdjuno.New(bdjuno.Config{
		Name:           nameBDJuno,
		HomeDir:        filepath.Join(f.config.AppDir, nameBDJuno),
		RepoDir:        filepath.Join(f.config.RootDir, "bdjuno"),
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
func (f *Factory) IBC(prefix string, coredApp cored.Cored) infra.AppSet {
	nameGaia := BuildPrefixedAppName(prefix, string(gaiad.AppType))
	nameOsmosis := BuildPrefixedAppName(prefix, string(osmosis.AppType))
	nameRelayerHermesGaia := BuildPrefixedAppName(prefix, string(hermes.AppType), string(gaiad.AppType))
	nameRelayerHermesOsmosis := BuildPrefixedAppName(prefix, string(hermes.AppType), string(osmosis.AppType))

	gaiaApp := gaiad.New(cosmoschain.AppConfig{
		Name:            nameGaia,
		HomeDir:         filepath.Join(f.config.AppDir, nameGaia),
		ChainID:         gaiad.DefaultChainID,
		HomeName:        gaiad.DefaultHomeName,
		AppInfo:         f.spec.DescribeApp(gaiad.AppType, nameGaia),
		Ports:           gaiad.DefaultPorts,
		RelayerMnemonic: gaiad.RelayerMnemonic,
		FundingMnemonic: gaiad.FundingMnemonic,
		TimeoutCommit:   f.config.TimeoutCommit,
		WrapperDir:      f.config.WrapperDir,
		GasPriceStr:     gaiad.DefaultGasPriceStr,
	})

	osmosisApp := osmosis.New(cosmoschain.AppConfig{
		Name:            nameOsmosis,
		HomeDir:         filepath.Join(f.config.AppDir, nameOsmosis),
		ChainID:         osmosis.DefaultChainID,
		HomeName:        osmosis.DefaultHomeName,
		AppInfo:         f.spec.DescribeApp(osmosis.AppType, nameOsmosis),
		Ports:           osmosis.DefaultPorts,
		RelayerMnemonic: osmosis.RelayerMnemonic,
		FundingMnemonic: osmosis.FundingMnemonic,
		TimeoutCommit:   f.config.TimeoutCommit,
		WrapperDir:      f.config.WrapperDir,
		GasPriceStr:     osmosis.DefaultGasPriceStr,
	})

	relayerHermesGaiaApp := hermes.New(hermes.Config{
		Name:                  nameRelayerHermesGaia,
		HomeDir:               filepath.Join(f.config.AppDir, nameRelayerHermesGaia),
		AppInfo:               f.spec.DescribeApp(hermes.AppType, nameRelayerHermesGaia),
		TelemetryPort:         hermes.DefaultTelemetryPort,
		Cored:                 coredApp,
		CoreumRelayerMnemonic: cored.RelayerMnemonicGaia,
		PeeredChain:           gaiaApp,
	})

	relayerHermesOsmosisApp := hermes.New(hermes.Config{
		Name:                  nameRelayerHermesOsmosis,
		HomeDir:               filepath.Join(f.config.AppDir, nameRelayerHermesOsmosis),
		AppInfo:               f.spec.DescribeApp(hermes.AppType, nameRelayerHermesOsmosis),
		TelemetryPort:         hermes.DefaultTelemetryPort + 100, // avoid conflicts with another Hermes.
		Cored:                 coredApp,
		CoreumRelayerMnemonic: cored.RelayerMnemonicOsmosis,
		PeeredChain:           osmosisApp,
	})

	return infra.AppSet{
		gaiaApp,
		osmosisApp,
		relayerHermesGaiaApp,
		relayerHermesOsmosisApp,
	}
}

// Monitoring returns set of applications required to run monitoring.
func (f *Factory) Monitoring(
	prefix string,
	coredNodes []cored.Cored,
	faucet faucet.Faucet,
	bdJuno bdjuno.BDJuno,
	hermesApps []hermes.Hermes,
) infra.AppSet {
	namePrometheus := BuildPrefixedAppName(prefix, string(prometheus.AppType))
	nameGrafana := BuildPrefixedAppName(prefix, string(grafana.AppType))

	prometheusApp := prometheus.New(prometheus.Config{
		Name:       namePrometheus,
		HomeDir:    filepath.Join(f.config.AppDir, namePrometheus),
		Port:       prometheus.DefaultPort,
		AppInfo:    f.spec.DescribeApp(prometheus.AppType, namePrometheus),
		CoredNodes: coredNodes,
		Faucet:     faucet,
		BDJuno:     bdJuno,
		HermesApps: hermesApps,
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

// XRPL returns xrpl node app set.
func (f *Factory) XRPL(prefix string) xrpl.XRPL {
	nameXRPL := BuildPrefixedAppName(prefix, string(xrpl.AppType))

	return xrpl.New(xrpl.Config{
		Name:       nameXRPL,
		HomeDir:    filepath.Join(f.config.AppDir, nameXRPL),
		AppInfo:    f.spec.DescribeApp(xrpl.AppType, nameXRPL),
		RPCPort:    xrpl.DefaultRPCPort,
		WSPort:     xrpl.DefaultWSPort,
		FaucetSeed: xrpl.DefaultFaucetSeed,
	})
}

// BridgeXRPLRelayers returns a set of XRPL relayer apps.
func (f *Factory) BridgeXRPLRelayers(
	prefix string,
	coredApp cored.Cored,
	xrplApp xrpl.XRPL,
	relayerCount int,
) (infra.AppSet, error) {
	if relayerCount > len(bridgexrpl.RelayerMnemonics) {
		return nil, errors.Errorf(
			"unsupported relayer count: %d, max: %d",
			relayerCount,
			len(bridgexrpl.RelayerMnemonics),
		)
	}

	var leader *bridgexrpl.Bridge
	relayers := make(infra.AppSet, 0, relayerCount)
	ports := bridgexrpl.DefaultPorts
	for i := 0; i < relayerCount; i++ {
		name := fmt.Sprintf("%s-%02d", BuildPrefixedAppName(prefix, string(bridgexrpl.AppType)), i)
		relayer := bridgexrpl.New(bridgexrpl.Config{
			Name:    name,
			HomeDir: filepath.Join(f.config.AppDir, name),
			ContractPath: filepath.Join(f.config.RootDir, "coreumbridge-xrpl", "contract", "artifacts",
				"coreumbridge_xrpl.wasm"),
			Mnemonics: bridgexrpl.RelayerMnemonics[i],
			Quorum:    uint32(relayerCount),
			AppInfo:   f.spec.DescribeApp(bridgexrpl.AppType, name),
			Ports:     ports,
			Leader:    leader,
			Cored:     coredApp,
			XRPL:      xrplApp,
		})
		ports.Metrics++
		if leader == nil {
			leader = &relayer
		}

		relayers = append(relayers, relayer)
	}

	return relayers, nil
}

// BuildPrefixedAppName builds the app name based on its prefix and name.
func BuildPrefixedAppName(prefix string, names ...string) string {
	return strings.Join(append([]string{prefix}, names...), "-")
}
