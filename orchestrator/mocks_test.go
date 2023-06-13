package orchestrator

import (
	"context"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	eth "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	tmctypes "github.com/tendermint/tendermint/rpc/core/types"
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

	peggyParamsFn        func(context.Context) (*peggytypes.Params, error)
	lastClaimEventFn     func(context.Context) (*peggytypes.LastClaimEvent, error)
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

	oldestUnsignedValsetsFn func(context.Context) ([]*peggytypes.Valset, error)
	sendValsetConfirmFn     func(
		ctx context.Context,
		peggyID eth.Hash,
		valset *peggytypes.Valset,
	) error

	oldestUnsignedTransactionBatchFn func(context.Context) (*peggytypes.OutgoingTxBatch, error)
	sendBatchConfirmFn               func(context.Context, eth.Hash, *peggytypes.OutgoingTxBatch) error

	latestValsetsFn func(context.Context) ([]*peggytypes.Valset, error)
	getBlockFn      func(context.Context, int64) (*tmctypes.ResultBlock, error)

	allValsetConfirmsFn func(context.Context, uint64) ([]*peggytypes.MsgValsetConfirm, error)
	valsetAtFn          func(context.Context, uint64) (*peggytypes.Valset, error)

	latestTransactionBatchesFn   func(context.Context) ([]*peggytypes.OutgoingTxBatch, error)
	transactionBatchSignaturesFn func(context.Context, uint64, eth.Address) ([]*peggytypes.MsgConfirmBatch, error)
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

func (i *mockInjective) OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return i.oldestUnsignedValsetsFn(ctx)
}

func (i *mockInjective) SendValsetConfirm(
	ctx context.Context,
	peggyID eth.Hash,
	valset *peggytypes.Valset,
) error {
	return i.sendValsetConfirmFn(ctx, peggyID, valset)
}

func (i *mockInjective) OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error) {
	return i.oldestUnsignedTransactionBatchFn(ctx)
}

func (i *mockInjective) GetBlock(ctx context.Context, height int64) (*tmctypes.ResultBlock, error) {
	return i.getBlockFn(ctx, height)
}

func (i *mockInjective) LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return i.latestValsetsFn(ctx)
}

func (i *mockInjective) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error) {
	return i.allValsetConfirmsFn(ctx, nonce)
}

func (i *mockInjective) SendBatchConfirm(
	ctx context.Context,
	peggyID eth.Hash,
	batch *peggytypes.OutgoingTxBatch,
) error {
	return i.sendBatchConfirmFn(ctx, peggyID, batch)
}

func (i *mockInjective) ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error) {
	return i.valsetAtFn(ctx, nonce)
}

func (i *mockInjective) LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
	return i.latestTransactionBatchesFn(ctx)
}

func (i *mockInjective) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract eth.Address) ([]*peggytypes.MsgConfirmBatch, error) {
	return i.transactionBatchSignaturesFn(ctx, nonce, tokenContract)
}

type mockEthereum struct {
	headerByNumberFn                    func(context.Context, *big.Int) (*ethtypes.Header, error)
	getSendToCosmosEventsFn             func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	getSendToInjectiveEventsFn          func(uint64, uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	getPeggyERC20DeployedEventsFn       func(uint64, uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	getValsetUpdatedEventsFn            func(uint64, uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	getTransactionBatchExecutedEventsFn func(uint64, uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)
	getPeggyIDFn                        func(context.Context) (eth.Hash, error)
	getValsetNonceFn                    func(context.Context) (*big.Int, error)
	sendEthValsetUpdateFn               func(context.Context, *peggytypes.Valset, *peggytypes.Valset, []*peggytypes.MsgValsetConfirm) (*eth.Hash, error)
	getTxBatchNonceFn                   func(context.Context, eth.Address) (*big.Int, error)
	sendTransactionBatchFn              func(context.Context, *peggytypes.Valset, *peggytypes.OutgoingTxBatch, []*peggytypes.MsgConfirmBatch) (*eth.Hash, error)
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

func (e mockEthereum) GetPeggyID(ctx context.Context) (eth.Hash, error) {
	return e.getPeggyIDFn(ctx)
}

func (e mockEthereum) GetValsetNonce(ctx context.Context) (*big.Int, error) {
	return e.getValsetNonceFn(ctx)
}

func (e mockEthereum) SendEthValsetUpdate(
	ctx context.Context,
	oldValset *peggytypes.Valset,
	newValset *peggytypes.Valset,
	confirms []*peggytypes.MsgValsetConfirm,
) (*eth.Hash, error) {
	return e.sendEthValsetUpdateFn(ctx, oldValset, newValset, confirms)
}

func (e mockEthereum) GetTxBatchNonce(
	ctx context.Context,
	erc20ContractAddress eth.Address,
) (*big.Int, error) {
	return e.getTxBatchNonceFn(ctx, erc20ContractAddress)
}

func (e mockEthereum) SendTransactionBatch(
	ctx context.Context,
	currentValset *peggytypes.Valset,
	batch *peggytypes.OutgoingTxBatch,
	confirms []*peggytypes.MsgConfirmBatch,
) (*eth.Hash, error) {
	return e.sendTransactionBatchFn(ctx, currentValset, batch, confirms)
}
