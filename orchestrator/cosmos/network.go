package cosmos

import (
	"context"

	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggy "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

type Network struct {
	PeggyQueryClient
	PeggyBroadcastClient
}

func NewNetwork() (*Network, error) {
	return nil, nil
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
