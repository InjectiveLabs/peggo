package cosmos

import (
	"fmt"

	clientcommon "github.com/InjectiveLabs/sdk-go/client/common"
	log "github.com/xlab/suplog"
)

type NetworkConfig struct {
	ChainID,
	ValidatorAddress,
	CosmosGRPC,
	TendermintRPC,
	GasPrice string
}

func (cfg NetworkConfig) loadClientConfig() clientcommon.Network {
	if custom := cfg.CosmosGRPC != "" && cfg.TendermintRPC != ""; custom {
		return customEndpoints(cfg)
	}

	return loadBalancedEndpoints(cfg)
}

func customEndpoints(cfg NetworkConfig) clientcommon.Network {
	c := clientcommon.LoadNetwork("devnet", "")
	c.Name = "custom"
	c.ChainId = cfg.ChainID
	c.Fee_denom = "inj"
	c.TmEndpoint = cfg.TendermintRPC
	c.ChainGrpcEndpoint = cfg.CosmosGRPC
	c.ExplorerGrpcEndpoint = ""
	c.LcdEndpoint = ""
	c.ExplorerGrpcEndpoint = ""

	log.Infoln("using custom endpoints for Injective")

	return c
}

func loadBalancedEndpoints(cfg NetworkConfig) clientcommon.Network {
	var networkName string
	switch cfg.ChainID {
	case "injective-1":
		networkName = "mainnet"
	case "injective-777":
		networkName = "devnet"
	case "injective-888":
		networkName = "testnet"
	default:
		panic(fmt.Errorf("no provider for chain id %s", cfg.ChainID))
	}

	log.Infoln("using load balanced endpoints for Injective")

	return clientcommon.LoadNetwork(networkName, "lb")
}
