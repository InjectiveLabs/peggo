package orchestrator

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"

	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

// InjectiveNetwork is the orchestrator's reference endpoint to the Injective network
type InjectiveNetwork interface {
	PeggyParams(ctx context.Context) (*peggytypes.Params, error)
	GetBlockCreationTime(ctx context.Context, height int64) (time.Time, error)
	GetValidatorAddress(ctx context.Context, addr gethcommon.Address) (sdk.ValAddress, error)

	LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error)
	SendEthereumClaims(ctx context.Context,
		lastClaimEvent uint64,
		oldDeposits []*peggyevents.PeggySendToCosmosEvent,
		deposits []*peggyevents.PeggySendToInjectiveEvent,
		withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
		erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
		valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
	) error

	UnbatchedTokensWithFees(ctx context.Context) ([]*peggytypes.BatchFees, error)
	SendRequestBatch(ctx context.Context, denom string) error
	OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error)
	SendBatchConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, batch *peggytypes.OutgoingTxBatch) error
	LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error)
	TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error)

	OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	SendValsetConfirm(ctx context.Context, ethFrom gethcommon.Address, peggyID gethcommon.Hash, valset *peggytypes.Valset) error
	LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error)
	ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error)
}
