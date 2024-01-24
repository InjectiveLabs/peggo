package cosmos

import (
	"context"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
)

type Network interface {
	GetBlockTime(ctx context.Context, height int64) (time.Time, error)

	peggy.QueryClient
	peggy.BroadcastClient
}

type NetworkConfig struct {
	ChainID,
	ValidatorAddress,
	CosmosGRPC,
	TendermintRPC,
	GasPrice string
}

func NewCosmosNetwork(k keyring.Keyring, ethSignFn keystore.PersonalSignFn, cfg NetworkConfig) (Network, error) {
	if isCustom := cfg.CosmosGRPC != "" && cfg.TendermintRPC != ""; isCustom {
		return newCustomNetwork(cfg, k, ethSignFn)
	}

	return newProviderNetwork(cfg, k, ethSignFn)
}
