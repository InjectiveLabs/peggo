package cosmos

import (
	"context"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
)

type PeggyBroadcastClient interface {
	/// Send a transaction updating the eth address for the sending
	/// Cosmos address. The sending Cosmos address should be a validator
	UpdatePeggyOrchestratorAddresses(
		ctx context.Context,
		ethFrom ethcmn.Address,
		orchAddr sdk.AccAddress,
	) error

	// SendValsetConfirm broadcasts in a confirmation for a specific validator set for a specific block height.
	SendValsetConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		peggyID ethcmn.Hash,
		valset *peggytypes.Valset,
	) error

	// SendBatchConfirm broadcasts in a confirmation for a specific transaction batch set for a specific block height
	// since transaction batches also include validator sets this has all the arguments
	SendBatchConfirm(
		ctx context.Context,
		ethFrom ethcmn.Address,
		peggyID ethcmn.Hash,
		batch *peggytypes.OutgoingTxBatch,
	) error

	SendEthereumClaims(
		ctx context.Context,
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
	SendToEth(
		ctx context.Context,
		destination ethcmn.Address,
		amount, fee sdk.Coin,
	) error

	// SendRequestBatch broadcasts a requests a batch of withdrawal transactions to be generated on the chain.
	SendRequestBatch(
		ctx context.Context,
		denom string,
	) error
}
