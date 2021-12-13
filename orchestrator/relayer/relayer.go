package relayer

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"

	peggytypes "github.com/umee-network/umee/x/peggy/types"
)

type PeggyRelayer interface {
	Start(ctx context.Context) error

	FindLatestValset(ctx context.Context) (*peggytypes.Valset, error)

	RelayBatches(
		ctx context.Context,
		currentValset *peggytypes.Valset,
		possibleBatches map[common.Address][]SubmittableBatch,
	) error

	RelayValsets(ctx context.Context, currentValset *peggytypes.Valset) error

	// SetPriceFeeder sets the (optional) price feeder used when performing profitable
	// batch calculations.
	SetPriceFeeder(*coingecko.PriceFeed)
}

type peggyRelayer struct {
	logger             zerolog.Logger
	cosmosQueryClient  peggytypes.QueryClient
	peggyContract      peggy.Contract
	ethProvider        provider.EVMProvider
	valsetRelayEnabled bool
	batchRelayEnabled  bool
	loopDuration       time.Duration
	priceFeeder        *coingecko.PriceFeed
	pendingTxWait      time.Duration
	profitMultiplier   float64

	// Store locally the last tx this validator made to avoid sending duplicates
	// or invalid txs.
	lastSentBatchNonce  uint64
	lastSentValsetNonce uint64
}

func NewPeggyRelayer(
	logger zerolog.Logger,
	peggyQueryClient peggytypes.QueryClient,
	peggyContract peggy.Contract,
	valsetRelayEnabled bool,
	batchRelayEnabled bool,
	loopDuration time.Duration,
	pendingTxWait time.Duration,
	profitMultiplier float64,
	options ...func(PeggyRelayer),
) PeggyRelayer {
	relayer := &peggyRelayer{
		logger:             logger.With().Str("module", "peggy_relayer").Logger(),
		cosmosQueryClient:  peggyQueryClient,
		peggyContract:      peggyContract,
		ethProvider:        peggyContract.Provider(),
		valsetRelayEnabled: valsetRelayEnabled,
		batchRelayEnabled:  batchRelayEnabled,
		loopDuration:       loopDuration,
		pendingTxWait:      pendingTxWait,
		profitMultiplier:   profitMultiplier,
	}

	for _, option := range options {
		option(relayer)
	}

	return relayer
}
