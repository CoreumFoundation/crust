package cored

import (
	"path/filepath"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/v2/pkg/config"
	"github.com/CoreumFoundation/coreum/v2/pkg/config/constant"
)

func saveTendermintConfig(nodeConfig config.NodeConfig, homeDir string) {
	err := nodeConfig.SavePrivateKeys(homeDir)
	must.OK(err)
	cfg := nodeConfig.TendermintNodeConfig(nil)
	// set addr_book_strict to false so nodes connecting from non-routable hosts are added to address book
	cfg.P2P.AddrBookStrict = false
	cfg.P2P.AllowDuplicateIP = true
	cfg.P2P.MaxNumOutboundPeers = 100
	cfg.P2P.MaxNumInboundPeers = 100
	cfg.RPC.MaxSubscriptionClients = 10000
	cfg.RPC.MaxOpenConnections = 10000
	cfg.RPC.GRPCMaxOpenConnections = 10000
	cfg.RPC.MaxSubscriptionsPerClient = 10000
	cfg.Mempool.Size = 50000
	cfg.Mempool.MaxTxsBytes = 5368709120

	must.OK(config.WriteTendermintConfigToFile(filepath.Join(homeDir, config.DefaultNodeConfigPath), cfg))
}

// NetworkConfig returns the network config used by crust.
func NetworkConfig(genesisTemplate string) (config.NetworkConfig, error) {
	networkConfig := config.NetworkConfig{
		Provider: config.DynamicConfigProvider{
			AddressPrefix:   constant.AddressPrefixDev,
			GenesisTemplate: genesisTemplate,
			ChainID:         constant.ChainIDDev,
			GenesisTime:     time.Now(),
			Denom:           constant.DenomDev,
			GovConfig: config.GovConfig{
				ProposalConfig: config.GovProposalConfig{
					MinDepositAmount: "1000",
					VotingPeriod:     (time.Second * 15).String(),
				},
			},
			CustomParamsConfig: config.CustomParamsConfig{
				Staking: config.CustomParamsStakingConfig{
					MinSelfDelegation: sdk.NewInt(10_000_000), // 10 core
				},
			},
		},
	}

	return networkConfig, nil
}
