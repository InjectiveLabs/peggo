package orchestrator

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
)

func TestEthSignerLoop(t *testing.T) {
	t.Parallel()

	t.Run("failed to fetch peggy id from contract", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxAttempts: 1,
			ethereum: mockEthereum{
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

		sig := &ethSigner{log: log.DefaultLogger, retries: 1}

		feed := mockPriceFeed{func(_ common.Address) (float64, error) {
			return 0, nil
		}}

		assert.NoError(t, sig.run(context.TODO(), injective, feed))
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

		sig := &ethSigner{log: log.DefaultLogger, retries: 1}

		feed := mockPriceFeed{func(_ common.Address) (float64, error) {
			return 0, nil
		}}

		assert.Error(t, sig.run(context.TODO(), injective, feed))
	})

	t.Run("no transaction batch sign", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			oldestUnsignedValsetsFn:          func(_ context.Context) ([]*types.Valset, error) { return nil, nil },
			sendValsetConfirmFn:              func(context.Context, common.Hash, *types.Valset, common.Address) error { return nil },
			oldestUnsignedTransactionBatchFn: func(_ context.Context) (*types.OutgoingTxBatch, error) { return nil, errors.New("fail") },
			sendBatchConfirmFn:               func(context.Context, common.Hash, *types.OutgoingTxBatch, common.Address) error { return nil },
		}

		sig := &ethSigner{log: log.DefaultLogger, retries: 1}

		feed := mockPriceFeed{func(_ common.Address) (float64, error) {
			return 0, nil
		}}

		assert.NoError(t, sig.run(context.TODO(), injective, feed))
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

		sig := &ethSigner{log: log.DefaultLogger, retries: 1}

		feed := mockPriceFeed{func(_ common.Address) (float64, error) {
			return 0, nil
		}}

		assert.Error(t, sig.run(context.TODO(), injective, feed))
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

		sig := &ethSigner{log: log.DefaultLogger, retries: 1}

		feed := mockPriceFeed{func(_ common.Address) (float64, error) {
			return 0, nil
		}}

		assert.NoError(t, sig.run(context.TODO(), injective, feed))
	})
}
