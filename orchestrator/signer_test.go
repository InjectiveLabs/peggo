package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	log "github.com/xlab/suplog"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestEthSignerLoop(t *testing.T) {
	t.Parallel()

	t.Run("failed to fetch peggy id from contract", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxAttempts: 1,
			eth: mockEthereum{
				getPeggyIDFn: func(context.Context) (common.Hash, error) {
					return [32]byte{}, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.EthSignerMainLoop(context.TODO()))
	})

	t.Run("no valset to sign", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			oldestUnsignedValsetsFn: func(context.Context) ([]*types.Valset, error) {
				return nil, errors.New("fail")
			},
			sendValsetConfirmFn: func(context.Context, common.Hash, *types.Valset, common.Address) error {
				return nil
			},
			oldestUnsignedTransactionBatchFn: func(context.Context) (*types.OutgoingTxBatch, error) {
				return nil, nil
			},
			sendBatchConfirmFn: func(context.Context, common.Hash, *types.OutgoingTxBatch, common.Address) error {
				return nil
			},
		}

		o := &PeggyOrchestrator{
			logger:      log.DefaultLogger,
			inj:         injective,
			maxAttempts: 1,
		}

		l := ethSignerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultLoopDur,
		}

		loopFn := l.loopFn(context.TODO())

		assert.NoError(t, loopFn())
	})

	t.Run("failed to send valset confirm", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
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
			sendValsetConfirmFn: func(context.Context, common.Hash, *types.Valset, common.Address) error {
				return errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:      log.DefaultLogger,
			inj:         injective,
			maxAttempts: 1,
		}

		l := ethSignerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultLoopDur,
		}

		loopFn := l.loopFn(context.TODO())

		assert.Error(t, loopFn())
	})

	t.Run("no transaction batch sign", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			oldestUnsignedValsetsFn:          func(_ context.Context) ([]*types.Valset, error) { return nil, nil },
			sendValsetConfirmFn:              func(context.Context, common.Hash, *types.Valset, common.Address) error { return nil },
			oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) { return nil, errors.New("fail") },
			sendBatchConfirmFn:               func(context.Context, common.Hash, *types.OutgoingTxBatch, common.Address) error { return nil },
		}

		o := &PeggyOrchestrator{
			logger:      log.DefaultLogger,
			inj:         injective,
			maxAttempts: 1,
		}

		l := ethSignerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultLoopDur,
		}

		loopFn := l.loopFn(context.TODO())

		assert.NoError(t, loopFn())
	})

	t.Run("failed to send batch confirm", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			oldestUnsignedValsetsFn: func(_ context.Context) ([]*types.Valset, error) { return nil, nil },
			sendValsetConfirmFn:     func(context.Context, common.Hash, *types.Valset, common.Address) error { return nil },
			oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) {
				return &types.OutgoingTxBatch{}, nil // non-empty will do
			},
			sendBatchConfirmFn: func(context.Context, common.Hash, *types.OutgoingTxBatch, common.Address) error {
				return errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:      log.DefaultLogger,
			inj:         injective,
			maxAttempts: 1,
		}

		l := ethSignerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultLoopDur,
		}

		loopFn := l.loopFn(context.TODO())

		assert.Error(t, loopFn())
	})

	t.Run("valset update and transaction batch are confirmed", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			oldestUnsignedValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{}, nil // non-empty will do
			},
			oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) {
				return &types.OutgoingTxBatch{}, nil // non-empty will do
			},
			sendValsetConfirmFn: func(context.Context, common.Hash, *types.Valset, common.Address) error { return nil },
			sendBatchConfirmFn:  func(context.Context, common.Hash, *types.OutgoingTxBatch, common.Address) error { return nil },
		}

		o := &PeggyOrchestrator{
			logger:      log.DefaultLogger,
			inj:         injective,
			maxAttempts: 1,
		}

		l := ethSignerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultLoopDur,
		}

		loopFn := l.loopFn(context.TODO())

		assert.NoError(t, loopFn())
	})
}
