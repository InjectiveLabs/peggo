package orchestrator

import (
	"context"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ctypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"
	"math/big"
	"testing"
	"time"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func TestValsetRelaying(t *testing.T) {
	t.Parallel()

	t.Run("failed to fetch latest valsets from injective", func(t *testing.T) {
		t.Parallel()

		injective := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                injective,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to fetch confirms for a valset", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("no confirms for valset", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return nil, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to get latest ethereum header", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to get latest ethereum header", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to get valset nonce from peggy contract", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to get specific valset from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return nil, errors.New("fail")
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("failed to get valset update events from ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{{}}, nil // non-empty will do
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{}, nil // non-empty will do
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("ethereum valset is not higher than injective valset", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{
					{
						Nonce:        333,
						RewardAmount: cosmtypes.NewInt(1000),
						RewardToken:  "0xfafafafafafafafa",
					},
				}, nil
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{
					Nonce:        333,
					RewardAmount: cosmtypes.NewInt(1000),
					RewardToken:  "0xfafafafafafafafa",
				}, nil // non-empty will do
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(333),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xfafafafafafafafa"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayValset(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but failed to get block from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{
					{
						Nonce:        444,
						RewardAmount: cosmtypes.NewInt(1000),
						RewardToken:  "0xfafafafafafafafa",
					},
				}, nil
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{
					Nonce:        333,
					RewardAmount: cosmtypes.NewInt(1000),
					RewardToken:  "0xfafafafafafafafa",
				}, nil // non-empty will do
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Time{}, errors.New("fail")
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(333),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xfafafafafafafafa"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:             suplog.DefaultLogger,
			inj:                inj,
			eth:                eth,
			maxAttempts:        1,
			valsetRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but valsetOffsetDur has not expired", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{
					{
						Nonce:        444,
						RewardAmount: cosmtypes.NewInt(1000),
						RewardToken:  "0xfafafafafafafafa",
					},
				}, nil
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{
					Nonce:        333,
					RewardAmount: cosmtypes.NewInt(1000),
					RewardToken:  "0xfafafafafafafafa",
				}, nil // non-empty will do
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Now().Add(time.Hour), nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(333),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xfafafafafafafafa"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:               suplog.DefaultLogger,
			inj:                  inj,
			eth:                  eth,
			maxAttempts:          1,
			valsetRelayEnabled:   true,
			relayValsetOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayValset(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but failed to send update tx to ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{
					{
						Nonce:        444,
						RewardAmount: cosmtypes.NewInt(1000),
						RewardToken:  "0xfafafafafafafafa",
					},
				}, nil
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{
					Nonce:        333,
					RewardAmount: cosmtypes.NewInt(1000),
					RewardToken:  "0xfafafafafafafafa",
				}, nil // non-empty will do
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC), nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(333),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xfafafafafafafafa"),
					},
				}, nil
			},
			sendEthValsetUpdateFn: func(_ context.Context, _ *types.Valset, _ *types.Valset, _ []*types.MsgValsetConfirm) (*common.Hash, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:               suplog.DefaultLogger,
			inj:                  inj,
			eth:                  eth,
			maxAttempts:          1,
			valsetRelayEnabled:   true,
			relayValsetOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayValset(context.TODO()))
	})

	t.Run("new valset update is sent to ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
				return []*types.Valset{
					{
						Nonce:        444,
						RewardAmount: cosmtypes.NewInt(1000),
						RewardToken:  "0xfafafafafafafafa",
					},
				}, nil
			},
			allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
				return []*types.MsgValsetConfirm{
					{
						Nonce:        5,
						Orchestrator: "orch",
						EthAddress:   "eth",
						Signature:    "sig",
					},
				}, nil
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{
					Nonce:        333,
					RewardAmount: cosmtypes.NewInt(1000),
					RewardToken:  "0xfafafafafafafafa",
				}, nil
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC), nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(123)}, nil
			},
			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(333),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xfafafafafafafafa"),
					},
				}, nil
			},
			sendEthValsetUpdateFn: func(_ context.Context, _ *types.Valset, _ *types.Valset, _ []*types.MsgValsetConfirm) (*common.Hash, error) {
				return &common.Hash{}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:               suplog.DefaultLogger,
			inj:                  inj,
			eth:                  eth,
			maxAttempts:          1,
			valsetRelayEnabled:   true,
			relayValsetOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayValset(context.TODO()))
	})
}

func TestBatchRelaying(t *testing.T) {
	t.Parallel()

	t.Run("failed to get latest batches from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get latest batches from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{{}}, nil // non-empty will do
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("no batch confirms", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{{}}, nil // non-empty will do
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return nil, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get batch nonce from ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{{}}, nil // non-empty will do
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get latest ethereum header", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    100,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(99), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get valset nonce from ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    100,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(99), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get specific valset from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    100,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return nil, errors.New("fail")
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(99), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("failed to get valset updated events from ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    100,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{}, nil
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(99), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("ethereum batch is not lower than injective batch", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    202,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{Nonce: 202}, nil
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(202), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(202),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xcafecafecafecafe"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayBatch(context.TODO()))
	})

	t.Run("ethereum batch is lower than injective batch but failed to get block from injhective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    202,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{Nonce: 202}, nil
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Time{}, errors.New("fail")
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(201), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(202),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xcafecafecafecafe"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:            suplog.DefaultLogger,
			inj:               inj,
			eth:               eth,
			maxAttempts:       1,
			batchRelayEnabled: true,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))
	})

	t.Run("ethereum batch is lower than injective batch but relayBatchOffsetDur has not expired", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    202,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{Nonce: 202}, nil
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Now().Add(time.Hour), nil
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(201), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(202),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xcafecafecafecafe"),
					},
				}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:              suplog.DefaultLogger,
			inj:                 inj,
			eth:                 eth,
			maxAttempts:         1,
			batchRelayEnabled:   true,
			relayBatchOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayBatch(context.TODO()))
	})

	t.Run("ethereum batch is lower than injective batch but failed to send batch update", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    202,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{Nonce: 202}, nil
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC), nil
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(201), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(202),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xcafecafecafecafe"),
					},
				}, nil
			},
			sendTransactionBatchFn: func(_ context.Context, _ *types.Valset, _ *types.OutgoingTxBatch, _ []*types.MsgConfirmBatch) (*common.Hash, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:              suplog.DefaultLogger,
			inj:                 inj,
			eth:                 eth,
			maxAttempts:         1,
			batchRelayEnabled:   true,
			relayBatchOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.Error(t, l.relayBatch(context.TODO()))

	})

	t.Run("sending a batch update to ethereum", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			latestTransactionBatchesFn: func(_ context.Context) ([]*types.OutgoingTxBatch, error) {
				return []*types.OutgoingTxBatch{
					{
						TokenContract: "tokenContract",
						BatchNonce:    202,
					},
				}, nil
			},
			transactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ common.Address) ([]*types.MsgConfirmBatch, error) {
				return []*types.MsgConfirmBatch{{}}, nil // non-nil will do
			},
			valsetAtFn: func(_ context.Context, _ uint64) (*types.Valset, error) {
				return &types.Valset{Nonce: 202}, nil
			},
			getBlockCreationTimeFn: func(_ context.Context, _ int64) (time.Time, error) {
				return time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC), nil
			},
		}

		eth := mockEthereum{
			getTxBatchNonceFn: func(_ context.Context, _ common.Address) (*big.Int, error) {
				return big.NewInt(201), nil
			},
			headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
				return &ctypes.Header{Number: big.NewInt(100)}, nil
			},

			getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
				return big.NewInt(100), nil
			},
			getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
				return []*wrappers.PeggyValsetUpdatedEvent{
					{
						NewValsetNonce: big.NewInt(202),
						RewardAmount:   big.NewInt(1000),
						RewardToken:    common.HexToAddress("0xcafecafecafecafe"),
					},
				}, nil
			},
			sendTransactionBatchFn: func(_ context.Context, _ *types.Valset, _ *types.OutgoingTxBatch, _ []*types.MsgConfirmBatch) (*common.Hash, error) {
				return &common.Hash{}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:              suplog.DefaultLogger,
			inj:                 inj,
			eth:                 eth,
			maxAttempts:         1,
			batchRelayEnabled:   true,
			relayBatchOffsetDur: time.Second * 5,
		}

		l := relayerLoop{
			PeggyOrchestrator: o,
			loopDuration:      defaultRelayerLoopDur,
		}

		assert.NoError(t, l.relayBatch(context.TODO()))
	})
}
