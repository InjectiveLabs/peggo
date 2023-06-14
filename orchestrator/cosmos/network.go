package cosmos

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tmclient"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggy "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	tmctypes "github.com/tendermint/tendermint/rpc/core/types"
)

type Network struct {
	tmclient.TendermintClient
	PeggyQueryClient
	PeggyBroadcastClient
}

func NewNetwork() (*Network, error) {
	return nil, nil
}

func (n *Network) GetBlock(ctx context.Context, height int64) (*tmctypes.ResultBlock, error) {
	return n.TendermintClient.GetBlock(ctx, height)
}

func (n *Network) PeggyParams(ctx context.Context) (*peggy.Params, error) {
	return n.PeggyQueryClient.PeggyParams(ctx)
}

func (n *Network) LastClaimEvent(ctx context.Context) (*peggy.LastClaimEvent, error) {
	return n.LastClaimEventByAddr(ctx, n.AccFromAddress())
}

func (n *Network) SendEthereumClaims(
	ctx context.Context,
	lastClaimEvent uint64,
	oldDeposits []*peggyevents.PeggySendToCosmosEvent,
	deposits []*peggyevents.PeggySendToInjectiveEvent,
	withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
	erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
	valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
) error {
	return n.PeggyBroadcastClient.SendEthereumClaims(ctx,
		lastClaimEvent,
		oldDeposits,
		deposits,
		withdraws,
		erc20Deployed,
		valsetUpdates,
	)
}

func (n *Network) UnbatchedTokenFees(ctx context.Context) ([]*peggy.BatchFees, error) {
	return n.PeggyQueryClient.UnbatchedTokensWithFees(ctx)
}

func (n *Network) SendRequestBatch(ctx context.Context, denom string) error {
	return n.PeggyBroadcastClient.SendRequestBatch(ctx, denom)
}

func (n *Network) OldestUnsignedValsets(ctx context.Context) ([]*peggy.Valset, error) {
	return n.PeggyQueryClient.OldestUnsignedValsets(ctx, n.AccFromAddress())
}

func (n *Network) LatestValsets(ctx context.Context) ([]*peggy.Valset, error) {
	return n.PeggyQueryClient.LatestValsets(ctx)
}

func (n *Network) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggy.MsgValsetConfirm, error) {
	return n.PeggyQueryClient.AllValsetConfirms(ctx, nonce)
}

func (n *Network) ValsetAt(ctx context.Context, nonce uint64) (*peggy.Valset, error) {
	return n.PeggyQueryClient.ValsetAt(ctx, nonce)
}

func (n *Network) SendValsetConfirm(
	ctx context.Context,
	peggyID ethcmn.Hash,
	valset *peggy.Valset,
	ethFrom ethcmn.Address,
) error {
	return n.PeggyBroadcastClient.SendValsetConfirm(ctx, ethFrom, peggyID, valset)
}

func (n *Network) OldestUnsignedTransactionBatch(ctx context.Context) (*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.OldestUnsignedTransactionBatch(ctx, n.AccFromAddress())
}

func (n *Network) LatestTransactionBatches(ctx context.Context) ([]*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.LatestTransactionBatches(ctx)
}

func (n *Network) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract ethcmn.Address) ([]*peggy.MsgConfirmBatch, error) {
	return n.PeggyQueryClient.TransactionBatchSignatures(ctx, nonce, tokenContract)
}

func (n *Network) SendBatchConfirm(
	ctx context.Context,
	peggyID ethcmn.Hash,
	batch *peggy.OutgoingTxBatch,
	ethFrom ethcmn.Address,
) error {
	return n.PeggyBroadcastClient.SendBatchConfirm(ctx, ethFrom, peggyID, batch)
}
