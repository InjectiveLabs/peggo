package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"
	"testing"
)

func TestEthSignerLoop(t *testing.T) {
	t.Parallel()

	t.Run("failed to fetch peggy id from contract", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1, // todo, hardcode do 10
			ethereum: mockEthereum{
				getPeggyIDFn: func(context.Context) (common.Hash, error) {
					return [32]byte{}, errors.New("fail")
				},
			},
		}

		err := orch.EthSignerMainLoop(context.TODO())
		assert.Error(t, err)
	})

	t.Run("no valset to sign", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1,
			injective: &mockInjective{
				oldestUnsignedValsetsFn: func(context.Context) ([]*types.Valset, error) {
					return nil, errors.New("fail")
				},
				sendValsetConfirmFn: func(_ context.Context, _ common.Hash, _ *types.Valset) error {
					return nil
				},
				oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) {
					return nil, nil
				},
				sendBatchConfirmFn: func(_ context.Context, _ common.Hash, _ *types.OutgoingTxBatch) error {
					return nil
				},
			},
		}

		err := orch.signerLoop(context.TODO(), suplog.DefaultLogger, [32]byte{1, 2, 3})
		assert.NoError(t, err)
	})

	t.Run("failed to send valset confirm", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1,
			injective: &mockInjective{
				oldestUnsignedValsetsFn: func(context.Context) ([]*types.Valset, error) {
					return []*types.Valset{
						{
							Nonce: 5,
							Members: []*types.BridgeValidator{
								{
									Power:           100,
									EthereumAddress: "abcd",
								},
							},
							Height:       500,
							RewardAmount: cosmtypes.NewInt(123),
							RewardToken:  "dusanToken",
						},
					}, nil
				},
				sendValsetConfirmFn: func(_ context.Context, _ common.Hash, _ *types.Valset) error {
					return errors.New("fail")
				},
			},
		}

		err := orch.signerLoop(context.TODO(), suplog.DefaultLogger, [32]byte{1, 2, 3})
		assert.Error(t, err)
	})

	t.Run("no transaction batch sign", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1,
			injective: &mockInjective{
				oldestUnsignedValsetsFn:          func(_ context.Context) ([]*types.Valset, error) { return nil, nil },
				sendValsetConfirmFn:              func(_ context.Context, _ common.Hash, _ *types.Valset) error { return nil },
				oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) { return nil, errors.New("fail") },
				sendBatchConfirmFn:               func(_ context.Context, _ common.Hash, _ *types.OutgoingTxBatch) error { return nil },
			},
		}

		err := orch.signerLoop(context.TODO(), suplog.DefaultLogger, [32]byte{})
		assert.NoError(t, err)
	})

	t.Run("failed to send batch confirm", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1,
			injective: &mockInjective{
				oldestUnsignedValsetsFn: func(_ context.Context) ([]*types.Valset, error) { return nil, nil },
				sendValsetConfirmFn:     func(_ context.Context, _ common.Hash, _ *types.Valset) error { return nil },
				oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) {
					return &types.OutgoingTxBatch{}, nil // non-empty will do
				},
				sendBatchConfirmFn: func(_ context.Context, _ common.Hash, _ *types.OutgoingTxBatch) error { return errors.New("fail") },
			},
		}

		err := orch.signerLoop(context.TODO(), suplog.DefaultLogger, [32]byte{})
		assert.Error(t, err)
	})

	t.Run("valset update and transaction batch are confirmed", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxRetries: 1,
			injective: &mockInjective{
				oldestUnsignedValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
					return []*types.Valset{}, nil // non-empty will do
				},
				oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) {
					return &types.OutgoingTxBatch{}, nil // non-empty will do
				},
				sendValsetConfirmFn: func(_ context.Context, _ common.Hash, _ *types.Valset) error { return nil },
				sendBatchConfirmFn:  func(_ context.Context, _ common.Hash, _ *types.OutgoingTxBatch) error { return nil },
			},
		}

		err := orch.signerLoop(context.TODO(), suplog.DefaultLogger, [32]byte{})
		assert.NoError(t, err)
	})
}
