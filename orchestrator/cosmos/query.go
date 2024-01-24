package cosmos

import (
	"context"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
)

type PeggyQueryClient interface {
	ValsetAt(ctx context.Context, nonce uint64) (*types.Valset, error)
	CurrentValset(ctx context.Context) (*types.Valset, error)
	OldestUnsignedValsets(ctx context.Context, valAccountAddress sdk.AccAddress) ([]*types.Valset, error)
	LatestValsets(ctx context.Context) ([]*types.Valset, error)
	AllValsetConfirms(ctx context.Context, nonce uint64) ([]*types.MsgValsetConfirm, error)
	OldestUnsignedTransactionBatch(ctx context.Context, valAccountAddress sdk.AccAddress) (*types.OutgoingTxBatch, error)
	LatestTransactionBatches(ctx context.Context) ([]*types.OutgoingTxBatch, error)
	UnbatchedTokensWithFees(ctx context.Context) ([]*types.BatchFees, error)

	TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract ethcmn.Address) ([]*types.MsgConfirmBatch, error)
	LastClaimEventByAddr(ctx context.Context, validatorAccountAddress sdk.AccAddress) (*types.LastClaimEvent, error)

	PeggyParams(ctx context.Context) (*types.Params, error)
	GetValidatorAddress(ctx context.Context, addr ethcmn.Address) (sdk.AccAddress, error)
}
