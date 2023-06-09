package orchestrator

import (
	"context"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"

	ethcmn "github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	sidechain "github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/InjectiveLabs/peggo/orchestrator/relayer"
)

type PriceFeed interface {
	QueryUSDPrice(address ethcmn.Address) (float64, error)
}

type InjectiveNetwork interface {
	PeggyParams(ctx context.Context) (*peggytypes.Params, error)

	UnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error)
	SendRequestBatch(ctx context.Context, denom string) error

	LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error)
	SendEthereumClaims(
		ctx context.Context,
		lastClaimEvent uint64,
		oldDeposits []*peggyevents.PeggySendToCosmosEvent,
		deposits []*peggyevents.PeggySendToInjectiveEvent,
		withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
		erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
		valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
	) error
}

type EthereumNetwork interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)

	GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)
}

type PeggyOrchestrator struct {
	svcTags   metrics.Tags
	pricefeed PriceFeed
	injective InjectiveNetwork
	ethereum  EthereumNetwork

	cosmosQueryClient       sidechain.PeggyQueryClient
	peggyBroadcastClient    sidechain.PeggyBroadcastClient
	peggyContract           peggy.PeggyContract
	ethProvider             provider.EVMProvider
	ethFrom                 ethcmn.Address
	erc20ContractMapping    map[ethcmn.Address]string
	relayer                 relayer.PeggyRelayer
	minBatchFeeUSD          float64
	priceFeeder             *coingecko.CoingeckoPriceFeed
	periodicBatchRequesting bool
}

func NewPeggyOrchestrator(
	cosmosQueryClient sidechain.PeggyQueryClient,
	peggyBroadcastClient sidechain.PeggyBroadcastClient,
	peggyContract peggy.PeggyContract,
	ethFrom ethcmn.Address,
	erc20ContractMapping map[ethcmn.Address]string,
	relayer relayer.PeggyRelayer,
	minBatchFeeUSD float64,
	priceFeeder *coingecko.CoingeckoPriceFeed,
	periodicBatchRequesting bool,
) *PeggyOrchestrator {
	return &PeggyOrchestrator{
		cosmosQueryClient:       cosmosQueryClient,
		peggyBroadcastClient:    peggyBroadcastClient,
		peggyContract:           peggyContract,
		ethProvider:             peggyContract.Provider(),
		ethFrom:                 ethFrom,
		erc20ContractMapping:    erc20ContractMapping,
		relayer:                 relayer,
		minBatchFeeUSD:          minBatchFeeUSD,
		priceFeeder:             priceFeeder,
		periodicBatchRequesting: periodicBatchRequesting,
		svcTags: metrics.Tags{
			"svc": "peggy_orchestrator",
		},
	}
}
