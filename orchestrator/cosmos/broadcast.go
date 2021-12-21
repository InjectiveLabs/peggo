package cosmos

import (
	"context"
	"sort"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/umee-network/peggo/cmd/peggo/client"
	"github.com/umee-network/peggo/orchestrator/ethereum/keystore"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
	umeeapp "github.com/umee-network/umee/app"
	"github.com/umee-network/umee/x/peggy/types"
)

type PeggyBroadcastClient interface {
	AccFromAddress() sdk.AccAddress

	// SendValsetConfirm broadcasts in a confirmation for a specific validator set for a specific block height.
	SendValsetConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		peggyID ethcmn.Hash,
		valset *types.Valset,
	) error

	// SendBatchConfirm broadcasts in a confirmation for a specific transaction batch set for a specific block height
	// since transaction batches also include validator sets this has all the arguments
	SendBatchConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		peggyID ethcmn.Hash,
		batch *types.OutgoingTxBatch,
	) error

	SendEthereumClaims(
		ctx context.Context,
		lastClaimEvent uint64,
		deposits []*wrappers.PeggySendToCosmosEvent,
		withdraws []*wrappers.PeggyTransactionBatchExecutedEvent,
		valsetUpdates []*wrappers.PeggyValsetUpdatedEvent,
		erc20Deployed []*wrappers.PeggyERC20DeployedEvent,
		loopDuration time.Duration,
	) error

	// SendRequestBatch broadcasts a requests a batch of withdrawal transactions to be generated on the chain.
	SendRequestBatch(
		ctx context.Context,
		denom string,
	) error
}

type (
	peggyBroadcastClient struct {
		logger            zerolog.Logger
		daemonQueryClient types.QueryClient
		broadcastClient   client.CosmosClient
		ethSignerFn       keystore.SignerFn
		ethPersonalSignFn keystore.PersonalSignFn
	}

	// sortableEvent exists with the only purpose to make a nicer sortable slice
	// for Ethereum events. It is only used in SendEthereumClaims.
	sortableEvent struct {
		EventNonce         uint64
		DepositEvent       *wrappers.PeggySendToCosmosEvent
		WithdrawEvent      *wrappers.PeggyTransactionBatchExecutedEvent
		ValsetUpdateEvent  *wrappers.PeggyValsetUpdatedEvent
		ERC20DeployedEvent *wrappers.PeggyERC20DeployedEvent
	}
)

func NewPeggyBroadcastClient(
	logger zerolog.Logger,
	queryClient types.QueryClient,
	broadcastClient client.CosmosClient,
	ethSignerFn keystore.SignerFn,
	ethPersonalSignFn keystore.PersonalSignFn,
) PeggyBroadcastClient {
	return &peggyBroadcastClient{
		logger:            logger.With().Str("module", "peggy_broadcast_client").Logger(),
		daemonQueryClient: queryClient,
		broadcastClient:   broadcastClient,
		ethSignerFn:       ethSignerFn,
		ethPersonalSignFn: ethPersonalSignFn,
	}
}

func (s *peggyBroadcastClient) AccFromAddress() sdk.AccAddress {
	return s.broadcastClient.FromAddress()
}

func (s *peggyBroadcastClient) SendValsetConfirm(
	ctx context.Context,
	ethFrom ethcmn.Address,
	peggyID ethcmn.Hash,
	valset *types.Valset,
) error {

	confirmHash := peggy.EncodeValsetConfirm(peggyID, valset)
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

func (s *peggyBroadcastClient) SendBatchConfirm(
	ctx context.Context,
	ethFrom ethcmn.Address,
	peggyID ethcmn.Hash,
	batch *types.OutgoingTxBatch,
) error {

	confirmHash := peggy.EncodeTxBatchConfirm(peggyID, batch)
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

func (s *peggyBroadcastClient) broadcastEthereumEvents(events []sortableEvent) error {
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
		case ev.DepositEvent != nil:
			recipientBz := ev.DepositEvent.Destination[:umeeapp.MaxAddrLen]

			msgs = append(msgs, &types.MsgDepositClaim{
				EventNonce:     ev.DepositEvent.EventNonce.Uint64(),
				BlockHeight:    ev.DepositEvent.Raw.BlockNumber,
				TokenContract:  ev.DepositEvent.TokenContract.Hex(),
				Amount:         sdk.NewIntFromBigInt(ev.DepositEvent.Amount),
				EthereumSender: ev.DepositEvent.Sender.Hex(),
				CosmosReceiver: sdk.AccAddress(recipientBz).String(),
				Orchestrator:   s.broadcastClient.FromAddress().String(),
			})
			evCounter["deposit"]++

		case ev.WithdrawEvent != nil:
			msgs = append(msgs, &types.MsgWithdrawClaim{
				EventNonce:    ev.WithdrawEvent.EventNonce.Uint64(),
				BatchNonce:    ev.WithdrawEvent.BatchNonce.Uint64(),
				BlockHeight:   ev.WithdrawEvent.Raw.BlockNumber,
				TokenContract: ev.WithdrawEvent.Token.Hex(),
				Orchestrator:  s.AccFromAddress().String(),
			})
			evCounter["withdraw"]++

		case ev.ValsetUpdateEvent != nil:
			members := make([]*types.BridgeValidator, len(ev.ValsetUpdateEvent.Validators))
			for i, val := range ev.ValsetUpdateEvent.Validators {
				members[i] = &types.BridgeValidator{
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
		Int("num_deposit", evCounter["deposit"]).
		Int("num_withdraw", evCounter["withdraw"]).
		Int("num_valset_update", evCounter["valset_update"]).
		Int("num_erc20_deploy", evCounter["erc20_deploy"]).
		Int("num_total_claims", len(events)).
		Msg("oracle observed events; sending claims")

	txResponse, err := s.broadcastClient.SyncBroadcastMsg(msgs...)
	if err != nil {
		s.logger.Err(err).Msg("broadcasting multiple claims failed")
		return err
	}

	s.logger.Info().
		Str("tx_hash", txResponse.TxHash).
		Int("total_claims", len(events)).
		Msg("oracle sent claims successfully")

	return nil
}

func (s *peggyBroadcastClient) SendEthereumClaims(
	ctx context.Context,
	lastClaimEvent uint64,
	deposits []*wrappers.PeggySendToCosmosEvent,
	withdraws []*wrappers.PeggyTransactionBatchExecutedEvent,
	valsetUpdates []*wrappers.PeggyValsetUpdatedEvent,
	erc20Deployed []*wrappers.PeggyERC20DeployedEvent,
	cosmosBlockTime time.Duration,
) error {
	allevents := []sortableEvent{}

	// We add all the events to the same list to be sorted.
	// Only events that have a nonce higher than the last claim event will be appended.
	for _, ev := range deposits {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:   ev.EventNonce.Uint64(),
				DepositEvent: ev,
			})
		}
	}

	for _, ev := range withdraws {
		if ev.EventNonce.Uint64() > lastClaimEvent {
			allevents = append(allevents, sortableEvent{
				EventNonce:    ev.EventNonce.Uint64(),
				WithdrawEvent: ev,
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

func (s *peggyBroadcastClient) SendRequestBatch(
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
		Denom:        denom,
		Orchestrator: s.AccFromAddress().String(),
	}
	if err := s.broadcastClient.QueueBroadcastMsg(msg); err != nil {
		err = errors.Wrap(err, "broadcasting MsgRequestBatch failed")
		return err
	}

	return nil
}
