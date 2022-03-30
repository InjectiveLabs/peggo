package relayer

import (
	"context"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"

	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"

	gravitytypes "github.com/umee-network/Gravity-Bridge/module/x/gravity/types"
)

// ValsetRelayMode defines an enumerated validator set relay mode.
type ValsetRelayMode int64

// Allowed validator set relay modes
const (
	ValsetRelayModeNone ValsetRelayMode = iota
	ValsetRelayModeMinimum
	ValsetRelayModeAll
)

// String gets the string representation of the validator set relay mode.
func (d ValsetRelayMode) String() string {
	return [...]string{"none", "minimum", "all"}[d]
}

type GravityRelayer interface {
	Start(ctx context.Context) error

	FindLatestValset(ctx context.Context) (*gravitytypes.Valset, error)

	RelayBatches(
		ctx context.Context,
		currentValset gravitytypes.Valset,
		possibleBatches map[ethcmn.Address][]SubmittableBatch,
	) error

	RelayValsets(ctx context.Context, currentValset gravitytypes.Valset) error

	// SetSymbolRetriever sets the Symbol Retriever api to get the symbol from contract address.
	SetSymbolRetriever(SymbolRetriever)

	// SetOracle sets the oracle for price feeder used when performing profitable
	// batch calculations.
	SetOracle(Oracle)

	GetProfitMultiplier() float64
}

type gravityRelayer struct {
	logger            zerolog.Logger
	cosmosQueryClient gravitytypes.QueryClient
	gravityContract   gravity.Contract
	ethProvider       provider.EVMProvider
	valsetRelayMode   ValsetRelayMode
	batchRelayEnabled bool
	loopDuration      time.Duration
	pendingTxWait     time.Duration
	profitMultiplier  float64
	symbolRetriever   SymbolRetriever
	oracle            Oracle

	// Store locally the last tx this validator made to avoid sending duplicates
	// or invalid txs.
	lastSentBatchNonce  uint64
	lastSentValsetNonce uint64
}

func NewGravityRelayer(
	logger zerolog.Logger,
	gravityQueryClient gravitytypes.QueryClient,
	gravityContract gravity.Contract,
	valsetRelayMode ValsetRelayMode,
	batchRelayEnabled bool,
	loopDuration time.Duration,
	pendingTxWait time.Duration,
	profitMultiplier float64,
	options ...func(GravityRelayer),
) GravityRelayer {
	relayer := &gravityRelayer{
		logger:            logger.With().Str("module", "gravity_relayer").Logger(),
		cosmosQueryClient: gravityQueryClient,
		gravityContract:   gravityContract,
		ethProvider:       gravityContract.Provider(),
		valsetRelayMode:   valsetRelayMode,
		batchRelayEnabled: batchRelayEnabled,
		loopDuration:      loopDuration,
		pendingTxWait:     pendingTxWait,
		profitMultiplier:  profitMultiplier,
	}

	for _, option := range options {
		option(relayer)
	}

	return relayer
}

func (s *gravityRelayer) GetProfitMultiplier() float64 {
	return s.profitMultiplier
}
