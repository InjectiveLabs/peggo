package orchestrator

import (
	"context"
	"math/big"
	"time"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	tmctypes "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/InjectiveLabs/metrics"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
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

	OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	SendValsetConfirm(ctx context.Context, peggyID ethcmn.Hash, valset *peggytypes.Valset, ethFrom ethcmn.Address) error

	OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error)
	SendBatchConfirm(ctx context.Context, peggyID ethcmn.Hash, batch *peggytypes.OutgoingTxBatch, ethFrom ethcmn.Address) error

	GetBlock(ctx context.Context, height int64) (*tmctypes.ResultBlock, error)

	LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error)
	ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error)

	LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error)
	TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract ethcmn.Address) ([]*peggytypes.MsgConfirmBatch, error)
}

type EthereumNetwork interface {
	FromAddress() ethcmn.Address
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	GetPeggyID(ctx context.Context) (ethcmn.Hash, error)

	GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)

	GetValsetNonce(ctx context.Context) (*big.Int, error)
	SendEthValsetUpdate(
		ctx context.Context,
		oldValset *peggytypes.Valset,
		newValset *peggytypes.Valset,
		confirms []*peggytypes.MsgValsetConfirm,
	) (*ethcmn.Hash, error)

	GetTxBatchNonce(
		ctx context.Context,
		erc20ContractAddress ethcmn.Address,
	) (*big.Int, error)

	SendTransactionBatch(
		ctx context.Context,
		currentValset *peggytypes.Valset,
		batch *peggytypes.OutgoingTxBatch,
		confirms []*peggytypes.MsgConfirmBatch,
	) (*ethcmn.Hash, error)
}

type PeggyOrchestrator struct {
	svcTags   metrics.Tags
	injective InjectiveNetwork
	ethereum  EthereumNetwork
	pricefeed PriceFeed

	erc20ContractMapping map[ethcmn.Address]string
	minBatchFeeUSD       float64
	maxRetries           uint

	relayValsetOffsetDur,
	relayBatchOffsetDur time.Duration

	valsetRelayEnabled bool
	batchRelayEnabled  bool

	periodicBatchRequesting bool
}

func NewPeggyOrchestrator(
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
	priceFeed PriceFeed,
	erc20ContractMapping map[ethcmn.Address]string,
	minBatchFeeUSD float64,
	periodicBatchRequesting,
	valsetRelayingEnabled,
	batchRelayingEnabled bool,
	valsetRelayingOffset,
	batchRelayingOffset string,
) (*PeggyOrchestrator, error) {
	orch := &PeggyOrchestrator{
		svcTags:                 metrics.Tags{"svc": "peggy_orchestrator"},
		injective:               injective,
		ethereum:                ethereum,
		pricefeed:               priceFeed,
		erc20ContractMapping:    erc20ContractMapping,
		minBatchFeeUSD:          minBatchFeeUSD,
		periodicBatchRequesting: periodicBatchRequesting,
		valsetRelayEnabled:      valsetRelayingEnabled,
		batchRelayEnabled:       batchRelayingEnabled,
		maxRetries:              10, // default is 10 for retry pkg
	}

	if valsetRelayingEnabled {
		dur, err := time.ParseDuration(valsetRelayingOffset)
		if err != nil {
			return nil, err
		}

		orch.relayValsetOffsetDur = dur
	}

	if batchRelayingEnabled {
		dur, err := time.ParseDuration(batchRelayingOffset)
		if err != nil {
			return nil, err
		}

		orch.relayBatchOffsetDur = dur
	}

	return orch, nil
}
