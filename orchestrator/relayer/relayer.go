package relayer

import (
	"context"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"

	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

const ethBlocksValsetOutdated = uint64(2000)

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
	lastSentBatchNonce         uint64
	lastSentValsetNonce        uint64
	latestValsetEthBlockNumber uint64
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

// UpdateLatestValsetEthBlockNumber only updates the last valset eth block number
// if the number is bigger than the one already stored in memory
func (s *gravityRelayer) UpdateLatestValsetEthBlockNumber(lastestValsetEthBlockNumber uint64) {
	if s.latestValsetEthBlockNumber > lastestValsetEthBlockNumber {
		return
	}
	s.latestValsetEthBlockNumber = lastestValsetEthBlockNumber
}

// IsLastestValsetUpdateOutdated checks if the latest valset update was sent
// more than 2000 blocks than the current height
func (s *gravityRelayer) IsLastestValsetUpdateOutdated(ctx context.Context) bool {
	if s.latestValsetEthBlockNumber == 0 {
		// means that it wasn't update or didn't passed to `FindLatestValset`
		return false
	}

	latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		s.logger.
			Err(errors.Wrap(err, "failed to get latest header")).
			Msg("IsLastValsetUpdateOutdated")
		return false
	}
	currentBlock := latestHeader.Number.Uint64()

	if s.latestValsetEthBlockNumber > currentBlock {
		return false
	}

	return (currentBlock - s.latestValsetEthBlockNumber) > ethBlocksValsetOutdated
}
