package peggy

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/InjectiveLabs/metrics"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
)

type BroadcastClient interface {
	/// Send a transaction updating the eth address for the sending
	/// Cosmos address. The sending Cosmos address should be a validator
	UpdatePeggyOrchestratorAddresses(ctx context.Context, ethFrom gethcommon.Address, orchAddr cosmostypes.AccAddress) error

	// SendValsetConfirm broadcasts in a confirmation for a specific validator set for a specific block height.
	SendValsetConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, valset *peggytypes.Valset) error

	// SendBatchConfirm broadcasts in a confirmation for a specific transaction batch set for a specific block height
	// since transaction batches also include validator sets this has all the arguments
	SendBatchConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, batch *peggytypes.OutgoingTxBatch) error

	SendEthereumClaims(ctx context.Context,
		lastClaimEvent uint64,
		oldDeposits []*peggyevents.PeggySendToCosmosEvent,
		deposits []*peggyevents.PeggySendToInjectiveEvent,
		withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
		erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
		valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
	) (uint64, error)

	// SendToEth broadcasts a Tx that tokens from Cosmos to Ethereum.
	// These tokens will not be sent immediately. Instead, they will require
	// some time to be included in a batch.
	SendToEth(ctx context.Context, destination gethcommon.Address, amount, fee cosmostypes.Coin) error

	// SendRequestBatch broadcasts a requests a batch of withdrawal transactions to be generated on the chain.
	SendRequestBatch(ctx context.Context, denom string) error
}

type broadcastClient struct {
	chainclient.ChainClient

	ethSignFn keystore.PersonalSignFn
	svcTags   metrics.Tags
}

func NewBroadcastClient(client chainclient.ChainClient, signFn keystore.PersonalSignFn) BroadcastClient {
	return broadcastClient{
		ChainClient: client,
		ethSignFn:   signFn,
		svcTags:     metrics.Tags{"svc": "peggy_broadcast"},
	}
}

func (c broadcastClient) UpdatePeggyOrchestratorAddresses(_ context.Context, ethFrom gethcommon.Address, orchAddr cosmostypes.AccAddress) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()
	// SetOrchestratorAddresses

	// This message allows validators to delegate their voting responsibilities
	// to a given key. This key is then used as an optional authentication method
	// for sigining oracle claims
	// This is used by the validators to set the Ethereum address that represents
	// them on the Ethereum side of the bridge. They must sign their Cosmos address
	// using the Ethereum address they have submitted. Like ValsetResponse this
	// message can in theory be submitted by anyone, but only the current validator
	// sets submissions carry any weight.
	// -------------
	msg := &peggytypes.MsgSetOrchestratorAddresses{
		Sender:       c.ChainClient.FromAddress().String(),
		EthAddress:   ethFrom.Hex(),
		Orchestrator: orchAddr.String(),
	}

	res, err := c.ChainClient.SyncBroadcastMsg(msg)
	fmt.Println("Response of set eth address", "res", res)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgSetOrchestratorAddresses failed")
	}

	return nil
}

func (c broadcastClient) SendValsetConfirm(_ context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, valset *peggytypes.Valset) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	confirmHash := peggy.EncodeValsetConfirm(peggyID, valset)
	signature, err := c.ethSignFn(ethFrom, confirmHash.Bytes())
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.New("failed to sign validator address")
	}

	// MsgValsetConfirm
	// this is the message sent by the validators when they wish to submit their
	// signatures over the validator set at a given block height. A validator must
	// first call MsgSetEthAddress to set their Ethereum address to be used for
	// signing. Then someone (anyone) must make a ValsetRequest the request is
	// essentially a messaging mechanism to determine which block all validators
	// should submit signatures over. Finally validators sign the validator set,
	// powers, and Ethereum addresses of the entire validator set at the height of a
	// ValsetRequest and submit that signature with this message.
	//
	// If a sufficient number of validators (66% of voting power) (A) have set
	// Ethereum addresses and (B) submit ValsetConfirm messages with their
	// signatures it is then possible for anyone to view these signatures in the
	// chain store and submit them to Ethereum to update the validator set
	// -------------
	msg := &peggytypes.MsgValsetConfirm{
		Orchestrator: c.FromAddress().String(),
		EthAddress:   ethFrom.Hex(),
		Nonce:        valset.Nonce,
		Signature:    gethcommon.Bytes2Hex(signature),
	}

	if err = c.ChainClient.QueueBroadcastMsg(msg); err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgValsetConfirm failed")
	}

	return nil
}

func (c broadcastClient) SendBatchConfirm(_ context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, batch *peggytypes.OutgoingTxBatch) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	confirmHash := peggy.EncodeTxBatchConfirm(peggyID, batch)
	signature, err := c.ethSignFn(ethFrom, confirmHash.Bytes())
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.New("failed to sign validator address")
	}

	// MsgConfirmBatch
	// When validators observe a MsgRequestBatch they form a batch by ordering
	// transactions currently in the txqueue in order of highest to lowest fee,
	// cutting off when the batch either reaches a hardcoded maximum size (to be
	// decided, probably around 100) or when transactions stop being profitable
	// (TODO determine this without nondeterminism) This message includes the batch
	// as well as an Ethereum signature over this batch by the validator
	// -------------
	msg := &peggytypes.MsgConfirmBatch{
		Orchestrator:  c.FromAddress().String(),
		Nonce:         batch.BatchNonce,
		Signature:     gethcommon.Bytes2Hex(signature),
		EthSigner:     ethFrom.Hex(),
		TokenContract: batch.TokenContract,
	}

	if err = c.ChainClient.QueueBroadcastMsg(msg); err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgConfirmBatch failed")
	}

	return nil
}

func (c broadcastClient) SendToEth(ctx context.Context, destination gethcommon.Address, amount, fee cosmostypes.Coin) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	// MsgSendToEth
	// This is the message that a user calls when they want to bridge an asset
	// it will later be removed when it is included in a batch and successfully
	// submitted tokens are removed from the users balance immediately
	// -------------
	// AMOUNT:
	// the coin to send across the bridge, note the restriction that this is a
	// single coin not a set of coins that is normal in other Cosmos messages
	// FEE:
	// the fee paid for the bridge, distinct from the fee paid to the chain to
	// actually send this message in the first place. So a successful send has
	// two layers of fees for the user
	// -------------
	msg := &peggytypes.MsgSendToEth{
		Sender:    c.FromAddress().String(),
		EthDest:   destination.Hex(),
		Amount:    amount,
		BridgeFee: fee, // TODO: use exactly that fee for transaction
	}

	if err := c.ChainClient.QueueBroadcastMsg(msg); err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgSendToEth failed")
	}

	return nil
}

func (c broadcastClient) SendRequestBatch(ctx context.Context, denom string) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	// MsgRequestBatch
	// this is a message anyone can send that requests a batch of transactions to
	// send across the bridge be created for whatever block height this message is
	// included in. This acts as a coordination point, the handler for this message
	// looks at the AddToOutgoingPool tx's in the store and generates a batch, also
	// available in the store tied to this message. The validators then grab this
	// batch, sign it, submit the signatures with a MsgConfirmBatch before a relayer
	// can finally submit the batch
	// -------------
	msg := &peggytypes.MsgRequestBatch{
		Denom:        denom,
		Orchestrator: c.FromAddress().String(),
	}
	if err := c.ChainClient.QueueBroadcastMsg(msg); err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgRequestBatch failed")
	}

	return nil
}

func (c broadcastClient) SendEthereumClaims(ctx context.Context, lastClaimEventNonce uint64, oldDeposits []*peggyevents.PeggySendToCosmosEvent, deposits []*peggyevents.PeggySendToInjectiveEvent, withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent, erc20Deployed []*peggyevents.PeggyERC20DeployedEvent, valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent) (uint64, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	events := sortEventsByNonce(oldDeposits, deposits, withdraws, erc20Deployed, valsetUpdates)

	// this can't happen outside of programmer error
	if firstToSend := events[0]; firstToSend.Nonce() != lastClaimEventNonce+1 {
		return 0, errors.Errorf("expected event with nonce %d, got %d", lastClaimEventNonce+1, firstToSend.Nonce())
	}

	for _, e := range events {
		if err := c.sendEventClaim(ctx, e); err != nil {
			return 0, err
		}

		// Considering blockTime=1s on Injective chain, Adding Sleep to make sure new event is
		// sent only after previous event is executed successfully.
		// Otherwise it will through `non contiguous event nonce` failing CheckTx.
		time.Sleep(1200 * time.Millisecond)
	}

	lastClaimEventNonce = events[len(events)-1].Nonce()

	return lastClaimEventNonce, nil
}

type event interface {
	Nonce() uint64
}

type (
	eventSendToCosmos             peggyevents.PeggySendToCosmosEvent
	eventSendToInjective          peggyevents.PeggySendToInjectiveEvent
	eventTransactionBatchExecuted peggyevents.PeggyTransactionBatchExecutedEvent
	eventERC20Deployed            peggyevents.PeggyERC20DeployedEvent
	eventValsetUpdated            peggyevents.PeggyValsetUpdatedEvent
)

func (e *eventSendToCosmos) Nonce() uint64 {
	return e.EventNonce.Uint64()
}

func (e *eventSendToInjective) Nonce() uint64 {
	return e.EventNonce.Uint64()
}

func (e *eventTransactionBatchExecuted) Nonce() uint64 {
	return e.EventNonce.Uint64()
}

func (e *eventERC20Deployed) Nonce() uint64 {
	return e.EventNonce.Uint64()
}

func (e *eventValsetUpdated) Nonce() uint64 {
	return e.EventNonce.Uint64()
}

func sortEventsByNonce(
	oldDeposits []*peggyevents.PeggySendToCosmosEvent,
	deposits []*peggyevents.PeggySendToInjectiveEvent,
	withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
	erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
	valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
) []event {
	total := len(oldDeposits) + len(deposits) + len(withdraws) + len(erc20Deployed) + len(valsetUpdates)
	events := make([]event, 0, total)

	for _, deposit := range oldDeposits {
		e := eventSendToCosmos(*deposit)
		events = append(events, &e)
	}

	for _, deposit := range deposits {
		e := eventSendToInjective(*deposit)
		events = append(events, &e)
	}

	for _, withdrawal := range withdraws {
		e := eventTransactionBatchExecuted(*withdrawal)
		events = append(events, &e)
	}

	for _, deployment := range erc20Deployed {
		e := eventERC20Deployed(*deployment)
		events = append(events, &e)
	}

	for _, vs := range valsetUpdates {
		e := eventValsetUpdated(*vs)
		events = append(events, &e)
	}

	// sort by nonce
	sort.Slice(events, func(i, j int) bool {
		return events[i].Nonce() < events[j].Nonce()
	})

	return events
}

func (c broadcastClient) sendEventClaim(ctx context.Context, ev event) error {
	switch ev := ev.(type) {
	case *eventSendToCosmos:
		e := peggyevents.PeggySendToCosmosEvent(*ev)
		return c.sendOldDepositClaims(ctx, &e)
	case *eventSendToInjective:
		e := peggyevents.PeggySendToInjectiveEvent(*ev)
		return c.sendDepositClaims(ctx, &e)
	case *eventTransactionBatchExecuted:
		e := peggyevents.PeggyTransactionBatchExecutedEvent(*ev)
		return c.sendWithdrawClaims(ctx, &e)
	case *eventERC20Deployed:
		e := peggyevents.PeggyERC20DeployedEvent(*ev)
		return c.sendErc20DeployedClaims(ctx, &e)
	case *eventValsetUpdated:
		e := peggyevents.PeggyValsetUpdatedEvent(*ev)
		return c.sendValsetUpdateClaims(ctx, &e)
	}

	return errors.Errorf("unknown event type %T", ev)
}

func (c broadcastClient) sendOldDepositClaims(
	ctx context.Context,
	oldDeposit *peggyevents.PeggySendToCosmosEvent,
) error {
	// EthereumBridgeDepositClaim
	// When more than 66% of the active validator set has
	// claimed to have seen the deposit enter the ethereum blockchain coins are
	// issued to the Cosmos address in question
	// -------------
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	log.WithFields(log.Fields{
		"sender":      oldDeposit.Sender.Hex(),
		"destination": cosmostypes.AccAddress(oldDeposit.Destination[12:32]).String(),
		"amount":      oldDeposit.Amount.String(),
		"event_nonce": oldDeposit.EventNonce.String(),
	}).Debugln("observed SendToCosmosEvent")

	msg := &peggytypes.MsgDepositClaim{
		EventNonce:     oldDeposit.EventNonce.Uint64(),
		BlockHeight:    oldDeposit.Raw.BlockNumber,
		TokenContract:  oldDeposit.TokenContract.Hex(),
		Amount:         cosmostypes.NewIntFromBigInt(oldDeposit.Amount),
		EthereumSender: oldDeposit.Sender.Hex(),
		CosmosReceiver: cosmostypes.AccAddress(oldDeposit.Destination[12:32]).String(),
		Orchestrator:   c.ChainClient.FromAddress().String(),
		Data:           "",
	}

	resp, err := c.ChainClient.SyncBroadcastMsg(msg)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgDepositClaim failed")
	}

	log.WithFields(log.Fields{
		"event_height": msg.BlockHeight,
		"event_nonce":  msg.EventNonce,
		"tx_hash":      resp.TxResponse.TxHash,
	}).Infoln("Oracle sent MsgDepositClaim")

	return nil
}

func (c broadcastClient) sendDepositClaims(
	ctx context.Context,
	deposit *peggyevents.PeggySendToInjectiveEvent,
) error {
	// EthereumBridgeDepositClaim
	// When more than 66% of the active validator set has
	// claimed to have seen the deposit enter the ethereum blockchain coins are
	// issued to the Cosmos address in question
	// -------------
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	log.WithFields(log.Fields{
		"sender":         deposit.Sender.Hex(),
		"destination":    cosmostypes.AccAddress(deposit.Destination[12:32]).String(),
		"amount":         deposit.Amount.String(),
		"data":           deposit.Data,
		"token_contract": deposit.TokenContract.Hex(),
	}).Debugln("observed SendToInjectiveEvent")

	msg := &peggytypes.MsgDepositClaim{
		EventNonce:     deposit.EventNonce.Uint64(),
		BlockHeight:    deposit.Raw.BlockNumber,
		TokenContract:  deposit.TokenContract.Hex(),
		Amount:         cosmostypes.NewIntFromBigInt(deposit.Amount),
		EthereumSender: deposit.Sender.Hex(),
		CosmosReceiver: cosmostypes.AccAddress(deposit.Destination[12:32]).String(),
		Orchestrator:   c.ChainClient.FromAddress().String(),
		Data:           deposit.Data,
	}

	resp, err := c.ChainClient.SyncBroadcastMsg(msg)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgDepositClaim failed")
	}

	log.WithFields(log.Fields{
		"event_nonce":  msg.EventNonce,
		"event_height": msg.BlockHeight,
		"tx_hash":      resp.TxResponse.TxHash,
	}).Infoln("EthOracle sent MsgDepositClaim")

	return nil
}

func (c broadcastClient) sendWithdrawClaims(
	ctx context.Context,
	withdraw *peggyevents.PeggyTransactionBatchExecutedEvent,
) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	log.WithFields(log.Fields{
		"batch_nonce":    withdraw.BatchNonce.String(),
		"token_contract": withdraw.Token.Hex(),
	}).Debugln("observed TransactionBatchExecutedEvent")

	// WithdrawClaim claims that a batch of withdrawal
	// operations on the bridge contract was executed.
	msg := &peggytypes.MsgWithdrawClaim{
		EventNonce:    withdraw.EventNonce.Uint64(),
		BatchNonce:    withdraw.BatchNonce.Uint64(),
		BlockHeight:   withdraw.Raw.BlockNumber,
		TokenContract: withdraw.Token.Hex(),
		Orchestrator:  c.FromAddress().String(),
	}

	resp, err := c.ChainClient.SyncBroadcastMsg(msg)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgWithdrawClaim failed")
	}

	log.WithFields(log.Fields{
		"event_height": msg.BlockHeight,
		"event_nonce":  msg.EventNonce,
		"tx_hash":      resp.TxResponse.TxHash,
	}).Infoln("EthOracle sent MsgWithdrawClaim")

	return nil
}

func (c broadcastClient) sendValsetUpdateClaims(
	ctx context.Context,
	valsetUpdate *peggyevents.PeggyValsetUpdatedEvent,
) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	log.WithFields(log.Fields{
		"valset_nonce":  valsetUpdate.NewValsetNonce.Uint64(),
		"validators":    valsetUpdate.Validators,
		"powers":        valsetUpdate.Powers,
		"reward_amount": valsetUpdate.RewardAmount,
		"reward_token":  valsetUpdate.RewardToken.Hex(),
	}).Debugln("observed ValsetUpdatedEvent")

	members := make([]*peggytypes.BridgeValidator, len(valsetUpdate.Validators))
	for i, val := range valsetUpdate.Validators {
		members[i] = &peggytypes.BridgeValidator{
			EthereumAddress: val.Hex(),
			Power:           valsetUpdate.Powers[i].Uint64(),
		}
	}

	msg := &peggytypes.MsgValsetUpdatedClaim{
		EventNonce:   valsetUpdate.EventNonce.Uint64(),
		ValsetNonce:  valsetUpdate.NewValsetNonce.Uint64(),
		BlockHeight:  valsetUpdate.Raw.BlockNumber,
		RewardAmount: cosmostypes.NewIntFromBigInt(valsetUpdate.RewardAmount),
		RewardToken:  valsetUpdate.RewardToken.Hex(),
		Members:      members,
		Orchestrator: c.FromAddress().String(),
	}

	resp, err := c.ChainClient.SyncBroadcastMsg(msg)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgValsetUpdatedClaim failed")
	}

	log.WithFields(log.Fields{
		"event_nonce":  msg.EventNonce,
		"event_height": msg.BlockHeight,
		"tx_hash":      resp.TxResponse.TxHash,
	}).Infoln("Oracle sent MsgValsetUpdatedClaim")

	return nil
}

func (c broadcastClient) sendErc20DeployedClaims(
	ctx context.Context,
	erc20Deployed *peggyevents.PeggyERC20DeployedEvent,
) error {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	log.WithFields(log.Fields{
		"cosmos_denom":   erc20Deployed.CosmosDenom,
		"token_contract": erc20Deployed.TokenContract.Hex(),
		"name":           erc20Deployed.Name,
		"symbol":         erc20Deployed.Symbol,
		"decimals":       erc20Deployed.Decimals,
	}).Debugln("observed ERC20DeployedEvent")

	msg := &peggytypes.MsgERC20DeployedClaim{
		EventNonce:    erc20Deployed.EventNonce.Uint64(),
		BlockHeight:   erc20Deployed.Raw.BlockNumber,
		CosmosDenom:   erc20Deployed.CosmosDenom,
		TokenContract: erc20Deployed.TokenContract.Hex(),
		Name:          erc20Deployed.Name,
		Symbol:        erc20Deployed.Symbol,
		Decimals:      uint64(erc20Deployed.Decimals),
		Orchestrator:  c.FromAddress().String(),
	}

	resp, err := c.ChainClient.SyncBroadcastMsg(msg)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return errors.Wrap(err, "broadcasting MsgERC20DeployedClaim failed")
	}

	log.WithFields(log.Fields{
		"event_nonce":  msg.EventNonce,
		"event_height": msg.BlockHeight,
		"tx_hash":      resp.TxResponse.TxHash,
	}).Infoln("Oracle sent MsgERC20DeployedClaim")

	return nil
}
