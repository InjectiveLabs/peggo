package orchestrator

import (
	"context"
	"sync"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	gravitytypes "github.com/umee-network/Gravity-Bridge/module/x/gravity/types"

	sidechain "github.com/umee-network/peggo/orchestrator/cosmos"
	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/keystore"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"
	"github.com/umee-network/peggo/orchestrator/relayer"
)

type GravityOrchestrator interface {
	Start(ctx context.Context) error
	CheckForEvents(ctx context.Context, startingBlock, ethBlockConfirmationDelay uint64) (currentBlock uint64, err error)
	GetLastCheckedBlock(ctx context.Context, ethBlockConfirmationDelay uint64) (uint64, error)
	EthOracleMainLoop(ctx context.Context) error
	EthSignerMainLoop(ctx context.Context) error
	BatchRequesterLoop(ctx context.Context) error
	RelayerMainLoop(ctx context.Context) error
}

type gravityOrchestrator struct {
	logger                     zerolog.Logger
	cosmosQueryClient          gravitytypes.QueryClient
	gravityBroadcastClient     sidechain.GravityBroadcastClient
	gravityContract            gravity.Contract
	ethProvider                provider.EVMProvider
	ethFrom                    ethcmn.Address
	ethSignerFn                keystore.SignerFn
	ethPersonalSignFn          keystore.PersonalSignFn
	relayer                    relayer.GravityRelayer
	cosmosBlockTime            time.Duration
	ethereumBlockTime          time.Duration
	batchRequesterLoopDuration time.Duration
	ethBlocksPerLoop           uint64
	bridgeStartHeight          uint64

	mtx             sync.Mutex
	erc20DenomCache map[string]string
}

func NewGravityOrchestrator(
	logger zerolog.Logger,
	cosmosQueryClient gravitytypes.QueryClient,
	gravityBroadcastClient sidechain.GravityBroadcastClient,
	gravityContract gravity.Contract,
	ethFrom ethcmn.Address,
	ethSignerFn keystore.SignerFn,
	ethPersonalSignFn keystore.PersonalSignFn,
	relayer relayer.GravityRelayer,
	cosmosBlockTime time.Duration,
	ethereumBlockTime time.Duration,
	batchRequesterLoopDuration time.Duration,
	ethBlocksPerLoop int64,
	bridgeStartHeight int64,
	options ...func(GravityOrchestrator),
) GravityOrchestrator {

	orch := &gravityOrchestrator{
		logger:                     logger.With().Str("module", "orchestrator").Logger(),
		cosmosQueryClient:          cosmosQueryClient,
		gravityBroadcastClient:     gravityBroadcastClient,
		gravityContract:            gravityContract,
		ethProvider:                gravityContract.Provider(),
		ethFrom:                    ethFrom,
		ethSignerFn:                ethSignerFn,
		ethPersonalSignFn:          ethPersonalSignFn,
		relayer:                    relayer,
		cosmosBlockTime:            cosmosBlockTime,
		ethereumBlockTime:          ethereumBlockTime,
		batchRequesterLoopDuration: batchRequesterLoopDuration,
		ethBlocksPerLoop:           uint64(ethBlocksPerLoop),
		bridgeStartHeight:          uint64(bridgeStartHeight),
	}

	for _, option := range options {
		option(orch)
	}

	return orch
}
