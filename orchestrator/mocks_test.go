package orchestrator

import (
	"context"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	eth "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

type mockPriceFeed struct {
	queryFn func(eth.Address) (float64, error)
}

func (p mockPriceFeed) QueryUSDPrice(address eth.Address) (float64, error) {
	return p.queryFn(address)
}

type mockInjective struct {
	unbatchedTokenFeesFn        func(context.Context) ([]*peggytypes.BatchFees, error)
	unbatchedTokenFeesCallCount int

	sendRequestBatchFn        func(context.Context, string) error
	sendRequestBatchCallCount int

	peggyParamsFn func(context.Context) (*peggytypes.Params, error)

	lastClaimEventFn func(context.Context) (*peggytypes.LastClaimEvent, error)

	sendEthereumClaimsFn func(
		ctx context.Context,
		lastClaimEvent uint64,
		oldDeposits []*peggyevents.PeggySendToCosmosEvent,
		deposits []*peggyevents.PeggySendToInjectiveEvent,
		withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
		erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
		valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
	) error
	sendEthereumClaimsCallCount int
}

func (i *mockInjective) UnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	i.unbatchedTokenFeesCallCount++
	return i.unbatchedTokenFeesFn(ctx)
}

func (i *mockInjective) SendRequestBatch(ctx context.Context, denom string) error {
	i.sendRequestBatchCallCount++
	return i.sendRequestBatchFn(ctx, denom)
}

func (i *mockInjective) PeggyParams(ctx context.Context) (*peggytypes.Params, error) {
	return i.peggyParamsFn(ctx)
}

func (i *mockInjective) LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error) {
	return i.lastClaimEventFn(ctx)
}

func (i *mockInjective) SendEthereumClaims(
	ctx context.Context,
	lastClaimEvent uint64,
	oldDeposits []*peggyevents.PeggySendToCosmosEvent,
	deposits []*peggyevents.PeggySendToInjectiveEvent,
	withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
	erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
	valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
) error {
	i.sendEthereumClaimsCallCount++
	return i.sendEthereumClaimsFn(
		ctx,
		lastClaimEvent,
		oldDeposits,
		deposits,
		withdraws,
		erc20Deployed,
		valsetUpdates,
	)
}

type mockEthereum struct {
	headerByNumberFn                    func(context.Context, *big.Int) (*ethtypes.Header, error)
	getSendToCosmosEventsFn             func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	getSendToInjectiveEventsFn          func(uint64, uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	getPeggyERC20DeployedEventsFn       func(uint64, uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	getValsetUpdatedEventsFn            func(uint64, uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	getTransactionBatchExecutedEventsFn func(uint64, uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)
}

func (e mockEthereum) HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
	return e.headerByNumberFn(ctx, number)
}

func (e mockEthereum) GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
	return e.getSendToCosmosEventsFn(startBlock, endBlock)
}

func (e mockEthereum) GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
	return e.getSendToInjectiveEventsFn(startBlock, endBlock)
}

func (e mockEthereum) GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
	return e.getPeggyERC20DeployedEventsFn(startBlock, endBlock)
}

func (e mockEthereum) GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
	return e.getValsetUpdatedEventsFn(startBlock, endBlock)
}

func (e mockEthereum) GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
	return e.getTransactionBatchExecutedEventsFn(startBlock, endBlock)
}
