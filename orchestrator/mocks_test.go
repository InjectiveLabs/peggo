package orchestrator

import (
	"context"
	"math/big"
	"time"

	cometrpc "github.com/cometbft/cometbft/rpc/core/types"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	log "github.com/xlab/suplog"

	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

type MockPriceFeed struct {
	QueryUSDPriceFn func(gethcommon.Address) (float64, error)
}

func (p MockPriceFeed) QueryUSDPrice(address gethcommon.Address) (float64, error) {
	return p.QueryUSDPriceFn(address)
}

type MockCosmosNetwork struct {
	PeggyParamsFn                      func(ctx context.Context) (*peggytypes.Params, error)
	LastClaimEventByAddrFn             func(ctx context.Context, address cosmostypes.AccAddress) (*peggytypes.LastClaimEvent, error)
	GetValidatorAddressFn              func(ctx context.Context, address gethcommon.Address) (cosmostypes.AccAddress, error)
	CurrentValsetFn                    func(ctx context.Context) (*peggytypes.Valset, error)
	ValsetAtFn                         func(ctx context.Context, uint642 uint64) (*peggytypes.Valset, error)
	OldestUnsignedValsetsFn            func(ctx context.Context, address cosmostypes.AccAddress) ([]*peggytypes.Valset, error)
	LatestValsetsFn                    func(ctx context.Context) ([]*peggytypes.Valset, error)
	AllValsetConfirmsFn                func(ctx context.Context, uint642 uint64) ([]*peggytypes.MsgValsetConfirm, error)
	OldestUnsignedTransactionBatchFn   func(ctx context.Context, address cosmostypes.AccAddress) (*peggytypes.OutgoingTxBatch, error)
	LatestTransactionBatchesFn         func(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error)
	UnbatchedTokensWithFeesFn          func(ctx context.Context) ([]*peggytypes.BatchFees, error)
	TransactionBatchSignaturesFn       func(ctx context.Context, uint642 uint64, address gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error)
	UpdatePeggyOrchestratorAddressesFn func(ctx context.Context, address gethcommon.Address, address2 cosmostypes.Address) error
	SendValsetConfirmFn                func(ctx context.Context, address gethcommon.Address, hash gethcommon.Hash, valset *peggytypes.Valset) error
	SendBatchConfirmFn                 func(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, batch *peggytypes.OutgoingTxBatch) error
	SendRequestBatchFn                 func(ctx context.Context, denom string) error
	SendToEthFn                        func(ctx context.Context, destination gethcommon.Address, amount, fee cosmostypes.Coin) error
	SendOldDepositClaimFn              func(ctx context.Context, deposit *peggyevents.PeggySendToCosmosEvent) error
	SendDepositClaimFn                 func(ctx context.Context, deposit *peggyevents.PeggySendToInjectiveEvent) error
	SendWithdrawalClaimFn              func(ctx context.Context, withdrawal *peggyevents.PeggyTransactionBatchExecutedEvent) error
	SendValsetClaimFn                  func(ctx context.Context, vs *peggyevents.PeggyValsetUpdatedEvent) error
	SendERC20DeployedClaimFn           func(ctx context.Context, erc20 *peggyevents.PeggyERC20DeployedEvent) error
	GetBlockFn                         func(ctx context.Context, height int64) (*cometrpc.ResultBlock, error)
	GetLatestBlockHeightFn             func(ctx context.Context) (int64, error)
}

func (n MockCosmosNetwork) PeggyParams(ctx context.Context) (*peggytypes.Params, error) {
	return n.PeggyParamsFn(ctx)
}

func (n MockCosmosNetwork) LastClaimEventByAddr(ctx context.Context, validatorAccountAddress cosmostypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
	return n.LastClaimEventByAddrFn(ctx, validatorAccountAddress)
}

func (n MockCosmosNetwork) GetValidatorAddress(ctx context.Context, addr gethcommon.Address) (cosmostypes.AccAddress, error) {
	return n.GetValidatorAddressFn(ctx, addr)
}

func (n MockCosmosNetwork) ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error) {
	return n.ValsetAtFn(ctx, nonce)
}

func (n MockCosmosNetwork) CurrentValset(ctx context.Context) (*peggytypes.Valset, error) {
	return n.CurrentValsetFn(ctx)
}

func (n MockCosmosNetwork) OldestUnsignedValsets(ctx context.Context, valAccountAddress cosmostypes.AccAddress) ([]*peggytypes.Valset, error) {
	return n.OldestUnsignedValsetsFn(ctx, valAccountAddress)
}

func (n MockCosmosNetwork) LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return n.LatestValsetsFn(ctx)
}

func (n MockCosmosNetwork) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error) {
	return n.AllValsetConfirmsFn(ctx, nonce)
}

func (n MockCosmosNetwork) OldestUnsignedTransactionBatch(ctx context.Context, valAccountAddress cosmostypes.AccAddress) (*peggytypes.OutgoingTxBatch, error) {
	return n.OldestUnsignedTransactionBatchFn(ctx, valAccountAddress)
}

func (n MockCosmosNetwork) LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
	return n.LatestTransactionBatchesFn(ctx)
}

func (n MockCosmosNetwork) UnbatchedTokensWithFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	return n.UnbatchedTokensWithFeesFn(ctx)
}

func (n MockCosmosNetwork) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
	return n.TransactionBatchSignaturesFn(ctx, nonce, tokenContract)
}

func (n MockCosmosNetwork) UpdatePeggyOrchestratorAddresses(ctx context.Context, ethFrom gethcommon.Address, orchAddr cosmostypes.AccAddress) error {
	return n.UpdatePeggyOrchestratorAddressesFn(ctx, ethFrom, orchAddr)
}

func (n MockCosmosNetwork) SendValsetConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, valset *peggytypes.Valset) error {
	return n.SendValsetConfirmFn(ctx, ethFrom, peggyID, valset)
}

func (n MockCosmosNetwork) SendBatchConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, batch *peggytypes.OutgoingTxBatch) error {
	return n.SendBatchConfirmFn(ctx, ethFrom, peggyID, batch)
}

func (n MockCosmosNetwork) SendRequestBatch(ctx context.Context, denom string) error {
	return n.SendRequestBatchFn(ctx, denom)
}

func (n MockCosmosNetwork) SendToEth(ctx context.Context, destination gethcommon.Address, amount, fee cosmostypes.Coin) error {
	return n.SendToEthFn(ctx, destination, amount, fee)
}

func (n MockCosmosNetwork) SendOldDepositClaim(ctx context.Context, deposit *peggyevents.PeggySendToCosmosEvent) error {
	return n.SendOldDepositClaimFn(ctx, deposit)
}

func (n MockCosmosNetwork) SendDepositClaim(ctx context.Context, deposit *peggyevents.PeggySendToInjectiveEvent) error {
	return n.SendDepositClaimFn(ctx, deposit)
}

func (n MockCosmosNetwork) SendWithdrawalClaim(ctx context.Context, withdrawal *peggyevents.PeggyTransactionBatchExecutedEvent) error {
	return n.SendWithdrawalClaimFn(ctx, withdrawal)
}

func (n MockCosmosNetwork) SendValsetClaim(ctx context.Context, vs *peggyevents.PeggyValsetUpdatedEvent) error {
	return n.SendValsetClaimFn(ctx, vs)
}

func (n MockCosmosNetwork) SendERC20DeployedClaim(ctx context.Context, erc20 *peggyevents.PeggyERC20DeployedEvent) error {
	return n.SendERC20DeployedClaimFn(ctx, erc20)
}

func (n MockCosmosNetwork) GetBlock(ctx context.Context, height int64) (*cometrpc.ResultBlock, error) {
	return n.GetBlockFn(ctx, height)
}

func (n MockCosmosNetwork) GetLatestBlockHeight(ctx context.Context) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (n MockCosmosNetwork) GetTxs(ctx context.Context, block *cometrpc.ResultBlock) ([]*cometrpc.ResultTx, error) {
	//TODO implement me
	panic("implement me")
}

func (n MockCosmosNetwork) GetValidatorSet(ctx context.Context, height int64) (*cometrpc.ResultValidators, error) {
	//TODO implement me
	panic("implement me")
}

type MockEthereumNetwork struct {
	GetHeaderByNumberFn                 func(ctx context.Context, number *big.Int) (*gethtypes.Header, error)
	GetPeggyIDFn                        func(ctx context.Context) (gethcommon.Hash, error)
	GetSendToCosmosEventsFn             func(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	GetSendToInjectiveEventsFn          func(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	GetPeggyERC20DeployedEventsFn       func(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	GetValsetUpdatedEventsFn            func(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	GetTransactionBatchExecutedEventsFn func(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)
	GetValsetNonceFn                    func(ctx context.Context) (*big.Int, error)
	SendEthValsetUpdateFn               func(ctx context.Context, oldValset *peggytypes.Valset, newValset *peggytypes.Valset, confirms []*peggytypes.MsgValsetConfirm) (*gethcommon.Hash, error)
	GetTxBatchNonceFn                   func(ctx context.Context, erc20ContractAddress gethcommon.Address) (*big.Int, error)
	SendTransactionBatchFn              func(ctx context.Context, currentValset *peggytypes.Valset, batch *peggytypes.OutgoingTxBatch, confirms []*peggytypes.MsgConfirmBatch) (*gethcommon.Hash, error)
	TokenDecimalsFn                     func(ctx context.Context, address gethcommon.Address) (uint8, error)
}

func (n MockEthereumNetwork) GetHeaderByNumber(ctx context.Context, number *big.Int) (*gethtypes.Header, error) {
	return n.GetHeaderByNumberFn(ctx, number)
}

func (n MockEthereumNetwork) TokenDecimals(ctx context.Context, tokenContract gethcommon.Address) (uint8, error) {
	return n.TokenDecimalsFn(ctx, tokenContract)
}

func (n MockEthereumNetwork) GetPeggyID(ctx context.Context) (gethcommon.Hash, error) {
	return n.GetPeggyIDFn(ctx)
}

func (n MockEthereumNetwork) GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
	return n.GetSendToCosmosEventsFn(startBlock, endBlock)
}

func (n MockEthereumNetwork) GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
	return n.GetSendToInjectiveEventsFn(startBlock, endBlock)
}

func (n MockEthereumNetwork) GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
	return n.GetPeggyERC20DeployedEventsFn(startBlock, endBlock)
}

func (n MockEthereumNetwork) GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
	return n.GetValsetUpdatedEventsFn(startBlock, endBlock)
}

func (n MockEthereumNetwork) GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
	return n.GetTransactionBatchExecutedEventsFn(startBlock, endBlock)
}

func (n MockEthereumNetwork) GetValsetNonce(ctx context.Context) (*big.Int, error) {
	return n.GetValsetNonceFn(ctx)
}

func (n MockEthereumNetwork) SendEthValsetUpdate(ctx context.Context, oldValset *peggytypes.Valset, newValset *peggytypes.Valset, confirms []*peggytypes.MsgValsetConfirm) (*gethcommon.Hash, error) {
	return n.SendEthValsetUpdateFn(ctx, oldValset, newValset, confirms)
}

func (n MockEthereumNetwork) GetTxBatchNonce(ctx context.Context, erc20ContractAddress gethcommon.Address) (*big.Int, error) {
	return n.GetTxBatchNonceFn(ctx, erc20ContractAddress)
}

func (n MockEthereumNetwork) SendTransactionBatch(ctx context.Context, currentValset *peggytypes.Valset, batch *peggytypes.OutgoingTxBatch, confirms []*peggytypes.MsgConfirmBatch) (*gethcommon.Hash, error) {
	return n.SendTransactionBatchFn(ctx, currentValset, batch, confirms)
}

var (
	DummyLog = DummyLogger{}
)

type DummyLogger struct{}

func (l DummyLogger) Success(format string, args ...interface{}) {
}

func (l DummyLogger) Warning(format string, args ...interface{}) {
}

func (l DummyLogger) Error(format string, args ...interface{}) {
}

func (l DummyLogger) Debug(format string, args ...interface{}) {
}

func (l DummyLogger) WithField(key string, value interface{}) log.Logger {
	return l
}

func (l DummyLogger) WithFields(fields log.Fields) log.Logger {
	return l
}

func (l DummyLogger) WithError(err error) log.Logger {
	return l
}

func (l DummyLogger) WithContext(ctx context.Context) log.Logger {
	return l
}

func (l DummyLogger) WithTime(t time.Time) log.Logger {
	return l
}

func (l DummyLogger) Logf(level log.Level, format string, args ...interface{}) {
}

func (l DummyLogger) Tracef(format string, args ...interface{}) {
}

func (l DummyLogger) Debugf(format string, args ...interface{}) {
}

func (l DummyLogger) Infof(format string, args ...interface{}) {
}

func (l DummyLogger) Printf(format string, args ...interface{}) {
}

func (l DummyLogger) Warningf(format string, args ...interface{}) {
}

func (l DummyLogger) Errorf(format string, args ...interface{}) {
}

func (l DummyLogger) Fatalf(format string, args ...interface{}) {
}

func (l DummyLogger) Panicf(format string, args ...interface{}) {
}

func (l DummyLogger) Log(level log.Level, args ...interface{}) {
}

func (l DummyLogger) Trace(args ...interface{}) {
}

func (l DummyLogger) Info(args ...interface{}) {
}

func (l DummyLogger) Print(args ...interface{}) {
}

func (l DummyLogger) Fatal(args ...interface{}) {
}

func (l DummyLogger) Panic(args ...interface{}) {
}

func (l DummyLogger) Logln(level log.Level, args ...interface{}) {
}

func (l DummyLogger) Traceln(args ...interface{}) {
}

func (l DummyLogger) Debugln(args ...interface{}) {
}

func (l DummyLogger) Infoln(args ...interface{}) {
}

func (l DummyLogger) Println(args ...interface{}) {
}

func (l DummyLogger) Warningln(args ...interface{}) {
}

func (l DummyLogger) Errorln(args ...interface{}) {
}

func (l DummyLogger) Fatalln(args ...interface{}) {
}

func (l DummyLogger) Panicln(args ...interface{}) {
}
