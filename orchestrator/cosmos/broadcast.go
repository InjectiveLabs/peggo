package cosmos

import (
	"context"
	"sort"
	"time"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/umee-network/peggo/cmd/peggo/client"
	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/keystore"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

type GravityBroadcastClient interface {
	AccFromAddress() sdk.AccAddress

	// SendValsetConfirm broadcasts in a confirmation for a specific validator set for a specific block height.
	SendValsetConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		gravityID string,
		valset types.Valset,
	) error

	// SendBatchConfirm broadcasts in a confirmation for a specific transaction batch set for a specific block height
	// since transaction batches also include validator sets this has all the arguments
	SendBatchConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		gravityID string,
		batch types.OutgoingTxBatch,
	) error

	SendEthereumClaims(
		ctx context.Context,
		lastClaimEvent uint64,
		deposits []*wrappers.GravitySendToCosmosEvent,
		withdraws []*wrappers.GravityTransactionBatchExecutedEvent,
		valsetUpdates []*wrappers.GravityValsetUpdatedEvent,
		erc20Deployed []*wrappers.GravityERC20DeployedEvent,
		loopDuration time.Duration,
	) error

	// SendRequestBatch broadcasts a requests a batch of withdrawal transactions to be generated on the chain.
	SendRequestBatch(
		ctx context.Context,
		denom string,
	) error
}

type (
	gravityBroadcastClient struct {
		logger            zerolog.Logger
		daemonQueryClient types.QueryClient
		broadcastClient   client.CosmosClient
		ethSignerFn       keystore.SignerFn
		ethPersonalSignFn keystore.PersonalSignFn
		msgsPerTx         int
	}

	// sortableEvent exists with the only purpose to make a nicer sortable slice
	// for Ethereum events. It is only used in SendEthereumClaims.
	sortableEvent struct {
		EventNonce                    uint64
		SendToCosmosEvent             *wrappers.GravitySendToCosmosEvent
		TransactionBatchExecutedEvent *wrappers.GravityTransactionBatchExecutedEvent
		ValsetUpdateEvent             *wrappers.GravityValsetUpdatedEvent
		ERC20DeployedEvent            *wrappers.GravityERC20DeployedEvent
	}
)

func NewGravityBroadcastClient(
	logger zerolog.Logger,
	queryClient types.QueryClient,
	broadcastClient client.CosmosClient,
	ethSignerFn keystore.SignerFn,
	ethPersonalSignFn keystore.PersonalSignFn,
	msgsPerTx int,
) GravityBroadcastClient {
	return &gravityBroadcastClient{
		logger:            logger.With().Str("module", "gravity_broadcast_client").Logger(),
		daemonQueryClient: queryClient,
		broadcastClient:   broadcastClient,
		ethSignerFn:       ethSignerFn,
		ethPersonalSignFn: ethPersonalSignFn,
		msgsPerTx:         msgsPerTx,
	}
}

func (s *gravityBroadcastClient) AccFromAddress() sdk.AccAddress {
	return s.broadcastClient.FromAddress()
}

func (s *gravityBroadcastClient) SendValsetConfirm(
	ctx context.Context,
	ethFrom ethcmn.Address,
	gravityID string,
	valset types.Valset,
) error {

	confirmHash := gravity.EncodeValsetConfirm(gravityID, valset)
	signature, err := s.ethPersonalSignFn(ethFrom, confirmHash.Bytes())
	if err != nil {
		err = errors.New("failed to sign validator address")
		return err
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
	msg := &types.MsgValsetConfirm{
		Orchestrator: s.AccFromAddress().String(),
		EthAddress:   ethFrom.Hex(),
		Nonce:        valset.Nonce,
		Signature:    ethcmn.Bytes2Hex(signature),
	}
	if err = s.broadcastClient.QueueBroadcastMsg(msg); err != nil {
		err = errors.Wrap(err, "broadcasting MsgValsetConfirm failed")
		return err
	}

	return nil
}

func (s *gravityBroadcastClient) SendBatchConfirm(
	ctx context.Context,
	ethFrom ethcmn.Address,
	gravityID string,
	batch types.OutgoingTxBatch,
) error {

	confirmHash := gravity.EncodeTxBatchConfirm(gravityID, batch)
	signature, err := s.ethPersonalSignFn(ethFrom, confirmHash.Bytes())
	if err != nil {
		err = errors.New("failed to sign validator address")
		return err
	}

	// MsgConfirmBatch
	// When validators observe a MsgRequestBatch they form a batch by ordering
	// transactions currently in the txqueue in order of highest to lowest fee,
	// cutting off when the batch either reaches a hardcoded maximum size (to be
	// decided, probably around 100) or when transactions stop being profitable
	// (TODO determine this without nondeterminism) This message includes the batch
	// as well as an Ethereum signature over this batch by the validator
	// -------------
	msg := &types.MsgConfirmBatch{
		Orchestrator:  s.AccFromAddress().String(),
		Nonce:         batch.BatchNonce,
		Signature:     ethcmn.Bytes2Hex(signature),
		EthSigner:     ethFrom.Hex(),
		TokenContract: batch.TokenContract,
	}
	if err = s.broadcastClient.QueueBroadcastMsg(msg); err != nil {
		err = errors.Wrap(err, "broadcasting MsgConfirmBatch failed")
		return err
	}

	return nil
}

func (s *gravityBroadcastClient) SendEthereumClaims(
	ctx context.Context,
	lastClaimEvent uint64,
	deposits []*wrappers.GravitySendToCosmosEvent,
	withdraws []*wrappers.GravityTransactionBatchExecutedEvent,
	valsetUpdates []*wrappers.GravityValsetUpdatedEvent,
	erc20Deployed []*wrappers.GravityERC20DeployedEvent,
	cosmosBlockTime time.Duration,
) error {
	allevents := []sortableEvent{}

	// We add all the events to the same list to be sorted.
	// Only events that have a nonce higher than the last claim event will be appended.
	for _, ev := range deposits {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:        ev.EventNonce.Uint64(),
				SendToCosmosEvent: ev,
			})
		}
	}

	for _, ev := range withdraws {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:                    ev.EventNonce.Uint64(),
				TransactionBatchExecutedEvent: ev,
			})
		}
	}

	for _, ev := range valsetUpdates {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:        ev.EventNonce.Uint64(),
				ValsetUpdateEvent: ev,
			})
		}
	}

	for _, ev := range erc20Deployed {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:         ev.EventNonce.Uint64(),
				ERC20DeployedEvent: ev,
			})
		}
	}

	return s.broadcastEthereumEvents(allevents)
}

func (s *gravityBroadcastClient) SendRequestBatch(
	ctx context.Context,
	denom string,
) error {
	// MsgRequestBatch
	// this is a message anyone can send that requests a batch of transactions to
	// send across the bridge be created for whatever block height this message is
	// included in. This acts as a coordination point, the handler for this message
	// looks at the AddToOutgoingPool tx's in the store and generates a batch, also
	// available in the store tied to this message. The validators then grab this
	// batch, sign it, submit the signatures with a MsgConfirmBatch before a relayer
	// can finally submit the batch
	// -------------

	msg := &types.MsgRequestBatch{
		Denom:  denom,
		Sender: s.AccFromAddress().String(),
	}
	if err := s.broadcastClient.QueueBroadcastMsg(msg); err != nil {
		err = errors.Wrap(err, "broadcasting MsgRequestBatch failed")
		return err
	}

	return nil
}

func (s *gravityBroadcastClient) broadcastEthereumEvents(events []sortableEvent) error {
	msgs := []sdk.Msg{}

	// Use SliceStable so we always get the same order
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].EventNonce < events[j].EventNonce
	})

	evCounter := map[string]int{
		"deposit":       0,
		"withdraw":      0,
		"valset_update": 0,
		"erc20_deploy":  0,
	}

	// iterate through events and send them sequentially.
	for _, ev := range events {
		switch {
		case ev.SendToCosmosEvent != nil:

			msgs = append(msgs, &types.MsgSendToCosmosClaim{
				EventNonce:     ev.SendToCosmosEvent.EventNonce.Uint64(),
				BlockHeight:    ev.SendToCosmosEvent.Raw.BlockNumber,
				TokenContract:  ev.SendToCosmosEvent.TokenContract.Hex(),
				Amount:         sdk.NewIntFromBigInt(ev.SendToCosmosEvent.Amount),
				EthereumSender: ev.SendToCosmosEvent.Sender.Hex(),
				CosmosReceiver: ev.SendToCosmosEvent.Destination,
				Orchestrator:   s.broadcastClient.FromAddress().String(),
			})
			evCounter["send_to_cosmos"]++

		case ev.TransactionBatchExecutedEvent != nil:
			msgs = append(msgs, &types.MsgBatchSendToEthClaim{
				EventNonce:    ev.TransactionBatchExecutedEvent.EventNonce.Uint64(),
				BatchNonce:    ev.TransactionBatchExecutedEvent.BatchNonce.Uint64(),
				BlockHeight:   ev.TransactionBatchExecutedEvent.Raw.BlockNumber,
				TokenContract: ev.TransactionBatchExecutedEvent.Token.Hex(),
				Orchestrator:  s.AccFromAddress().String(),
			})
			evCounter["transaction_batch_executed"]++

		case ev.ValsetUpdateEvent != nil:
			members := make([]types.BridgeValidator, len(ev.ValsetUpdateEvent.Validators))
			for i, val := range ev.ValsetUpdateEvent.Validators {
				members[i] = types.BridgeValidator{
					EthereumAddress: val.Hex(),
					Power:           ev.ValsetUpdateEvent.Powers[i].Uint64(),
				}
			}

			msgs = append(msgs, &types.MsgValsetUpdatedClaim{
				EventNonce:   ev.ValsetUpdateEvent.EventNonce.Uint64(),
				ValsetNonce:  ev.ValsetUpdateEvent.NewValsetNonce.Uint64(),
				BlockHeight:  ev.ValsetUpdateEvent.Raw.BlockNumber,
				RewardAmount: sdk.NewIntFromBigInt(ev.ValsetUpdateEvent.RewardAmount),
				RewardToken:  ev.ValsetUpdateEvent.RewardToken.Hex(),
				Members:      members,
				Orchestrator: s.AccFromAddress().String(),
			})
			evCounter["valset_update"]++

		case ev.ERC20DeployedEvent != nil:
			msgs = append(msgs, &types.MsgERC20DeployedClaim{
				EventNonce:    ev.ERC20DeployedEvent.EventNonce.Uint64(),
				BlockHeight:   ev.ERC20DeployedEvent.Raw.BlockNumber,
				Orchestrator:  s.AccFromAddress().String(),
				CosmosDenom:   ev.ERC20DeployedEvent.CosmosDenom,
				TokenContract: ev.ERC20DeployedEvent.TokenContract.Hex(),
				Name:          ev.ERC20DeployedEvent.Name,
				Decimals:      uint64(ev.ERC20DeployedEvent.Decimals),
				Symbol:        ev.ERC20DeployedEvent.Symbol,
			})
			evCounter["erc20_deploy"]++

		}
	}

	s.logger.Info().
		Int("num_send_to_cosmos", evCounter["send_to_cosmos"]).
		Int("num_transaction_batch_executed", evCounter["transaction_batch_executed"]).
		Int("num_valset_update", evCounter["valset_update"]).
		Int("num_erc20_deploy", evCounter["erc20_deploy"]).
		Int("num_total_claims", len(events)).
		Msg("oracle observed events; sending claims")

	// We send the messages in batches, so that we don't hit any limits
	msgSets := splitMsgs(msgs, s.msgsPerTx)

	for _, msgSet := range msgSets {
		txResponse, err := s.broadcastClient.SyncBroadcastMsg(msgSet...)
		if err != nil {
			s.logger.Err(err).Msg("broadcasting multiple claims failed")
			return err
		}

		s.logger.Info().
			Str("tx_hash", txResponse.TxHash).
			Int("total_claims", len(events)).
			Int("claims_sent", len(msgSet)).
			Msg("oracle sent set of claims successfully")
	}

	return nil
}

func splitMsgs(buf []sdk.Msg, lim int) [][]sdk.Msg {
	var chunk []sdk.Msg
	chunks := make([][]sdk.Msg, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf)
	}
	return chunks
}
