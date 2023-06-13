package orchestrator

import (
	"context"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ctypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	tmctypes "github.com/tendermint/tendermint/rpc/core/types"
	types2 "github.com/tendermint/tendermint/types"
	"math/big"
	"testing"
	time "time"
)

func TestValsetRelaying(t *testing.T) {
	t.Parallel()

	t.Run("failed to fetch latest valsets from injective", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
				latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to fetch confirms for a valset", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
				latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
					return []*types.Valset{{}}, nil // non-empty will do
				},
				allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("no confirms for valset", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
				latestValsetsFn: func(_ context.Context) ([]*types.Valset, error) {
					return []*types.Valset{{}}, nil // non-empty will do
				},
				allValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*types.MsgValsetConfirm, error) {
					return nil, nil
				},
			},
		}

		assert.NoError(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to get latest ethereum header", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
				headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to get latest ethereum header", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
				headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to get valset nonce from peggy contract", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
				headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
					return &ctypes.Header{Number: big.NewInt(123)}, nil
				},
				getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to get specific valset from injective", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
				headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
					return &ctypes.Header{Number: big.NewInt(123)}, nil
				},
				getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
					return big.NewInt(100), nil
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("failed to get valset update events from ethereum", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
				headerByNumberFn: func(_ context.Context, _ *big.Int) (*ctypes.Header, error) {
					return &ctypes.Header{Number: big.NewInt(123)}, nil
				},
				getValsetNonceFn: func(_ context.Context) (*big.Int, error) {
					return big.NewInt(100), nil
				},
				getValsetUpdatedEventsFn: func(_ uint64, _ uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("ethereum valset is not higher than injective valset", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
			},
			ethereum: mockEthereum{
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
			},
		}

		assert.NoError(t, orch.relayValsets(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but failed to get block from injective", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
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
				getBlockFn: func(_ context.Context, _ int64) (*tmctypes.ResultBlock, error) {
					return nil, errors.New("fail")
				},
			},
			ethereum: mockEthereum{
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
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but valsetOffsetDur has not expired", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			relayValsetOffsetDur: time.Second * 5,
			injective: &mockInjective{
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
				getBlockFn: func(_ context.Context, _ int64) (*tmctypes.ResultBlock, error) {
					return &tmctypes.ResultBlock{
						Block: &types2.Block{
							Header: types2.Header{
								Time: time.Now().Add(time.Hour),
							},
						},
					}, nil
				},
			},
			ethereum: mockEthereum{
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
			},
		}

		assert.NoError(t, orch.relayValsets(context.TODO()))
	})

	t.Run("injective valset is higher than ethereum but failed to send update tx to ethereum", func(t *testing.T) {
		t.Parallel()

		oldTime := time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC)
		orch := &PeggyOrchestrator{
			relayValsetOffsetDur: time.Second * 5,
			injective: &mockInjective{
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
				getBlockFn: func(_ context.Context, _ int64) (*tmctypes.ResultBlock, error) {
					return &tmctypes.ResultBlock{
						Block: &types2.Block{
							Header: types2.Header{
								Time: oldTime,
							},
						},
					}, nil
				},
			},
			ethereum: mockEthereum{
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
			},
		}

		assert.Error(t, orch.relayValsets(context.TODO()))
	})

	t.Run("new valset update is sent to ethereum", func(t *testing.T) {
		t.Parallel()

		oldTime := time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC)
		orch := &PeggyOrchestrator{
			relayValsetOffsetDur: time.Second * 5,
			injective: &mockInjective{
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
				getBlockFn: func(_ context.Context, _ int64) (*tmctypes.ResultBlock, error) {
					return &tmctypes.ResultBlock{
						Block: &types2.Block{
							Header: types2.Header{
								Time: oldTime,
							},
						},
					}, nil
				},
			},
			ethereum: mockEthereum{
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
			},
		}

		assert.NoError(t, orch.relayValsets(context.TODO()))
	})
}
