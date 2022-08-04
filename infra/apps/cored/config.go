package cored

import (
	"math/big"
	"path/filepath"
	"time"

	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum/app"
)

// SaveConfig sets specific tendermint config and saves it alongside private keys
func SaveConfig(nodeConfig app.NodeConfig, homeDir string) {
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

	must.OK(app.WriteTendermintConfigToFile(filepath.Join(homeDir, app.DefaultNodeConfigPath), cfg))
}

// CustomZNetNetworkConfig is the network config used by znet
var CustomZNetNetworkConfig = app.NetworkConfig{
	ChainID:       app.Devnet,
	Enabled:       true,
	GenesisTime:   time.Now(),
	AddressPrefix: "devcore",
	TokenSymbol:   app.TokenSymbolDev,
	Fee: app.FeeConfig{
		InitialGasPrice:       big.NewInt(1500),
		MinDiscountedGasPrice: big.NewInt(1000),
		DeterministicGas: app.DeterministicGasConfig{
			BankSend: 120000,
		},
	},
}
