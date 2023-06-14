package orchestrator

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func TestRelayEvents(t *testing.T) {
	t.Parallel()

	t.Run("failed to get latest header from ethereum", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return nil, errors.New("fail")
				},
			},
		}

		_, err := orch.relayEthEvents(context.TODO(), 0)
		assert.Error(t, err)
	})

	t.Run("latest ethereum header is old", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return &types.Header{Number: big.NewInt(50)}, nil
				},
			},
		}

		currentBlock, err := orch.relayEthEvents(context.TODO(), 100)
		assert.NoError(t, err)
		assert.Equal(t, currentBlock, 50-ethBlockConfirmationDelay)
	})

	t.Run("failed to get SendToCosmos events", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return &types.Header{Number: big.NewInt(200)}, nil
				},
				getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
					return nil, errors.New("fail")
				},
			},
		}

		_, err := orch.relayEthEvents(context.TODO(), 100)
		assert.Error(t, err)
	})

	t.Run("failed to get last claim event from injective", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return &types.Header{Number: big.NewInt(200)}, nil
				},
				getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
					return []*wrappers.PeggySendToCosmosEvent{}, nil // empty slice will do
				},

				// no-ops
				getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyTransactionBatchExecutedEvent, error) {
					return nil, nil
				},
				getValsetUpdatedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
					return nil, nil
				},
				getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyERC20DeployedEvent, error) {
					return nil, nil
				},
				getSendToInjectiveEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToInjectiveEvent, error) {
					return nil, nil
				},
			},

			injective: &mockInjective{
				lastClaimEventFn: func(context.Context) (*peggytypes.LastClaimEvent, error) {
					return nil, errors.New("fail")
				},
			},
		}

		_, err := orch.relayEthEvents(context.TODO(), 100)
		assert.Error(t, err)
	})

	t.Run("old events are pruned", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			lastClaimEventFn: func(context.Context) (*peggytypes.LastClaimEvent, error) {
				return &peggytypes.LastClaimEvent{EthereumEventNonce: 6}, nil
			},
			sendEthereumClaimsFn: func(
				context.Context,
				uint64,
				[]*wrappers.PeggySendToCosmosEvent,
				[]*wrappers.PeggySendToInjectiveEvent,
				[]*wrappers.PeggyTransactionBatchExecutedEvent,
				[]*wrappers.PeggyERC20DeployedEvent,
				[]*wrappers.PeggyValsetUpdatedEvent,
			) error {
				return nil
			},
		}

		orch := &PeggyOrchestrator{
			injective: inj,
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return &types.Header{Number: big.NewInt(200)}, nil
				},
				getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
					return []*wrappers.PeggySendToCosmosEvent{{EventNonce: big.NewInt(5)}}, nil
				},

				// no-ops
				getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyTransactionBatchExecutedEvent, error) {
					return nil, nil
				},
				getValsetUpdatedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
					return nil, nil
				},
				getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyERC20DeployedEvent, error) {
					return nil, nil
				},
				getSendToInjectiveEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToInjectiveEvent, error) {
					return nil, nil
				},
			},
		}

		_, err := orch.relayEthEvents(context.TODO(), 100)
		assert.NoError(t, err)
		assert.Equal(t, inj.sendEthereumClaimsCallCount, 0)
	})

	t.Run("new events are sent to injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			lastClaimEventFn: func(context.Context) (*peggytypes.LastClaimEvent, error) {
				return &peggytypes.LastClaimEvent{EthereumEventNonce: 6}, nil
			},
			sendEthereumClaimsFn: func(
				context.Context,
				uint64,
				[]*wrappers.PeggySendToCosmosEvent,
				[]*wrappers.PeggySendToInjectiveEvent,
				[]*wrappers.PeggyTransactionBatchExecutedEvent,
				[]*wrappers.PeggyERC20DeployedEvent,
				[]*wrappers.PeggyValsetUpdatedEvent,
			) error {
				return nil
			},
		}

		orch := &PeggyOrchestrator{
			injective: inj,
			ethereum: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return &types.Header{Number: big.NewInt(200)}, nil
				},
				getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
					return []*wrappers.PeggySendToCosmosEvent{{EventNonce: big.NewInt(10)}}, nil
				},

				// no-ops
				getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyTransactionBatchExecutedEvent, error) {
					return nil, nil
				},
				getValsetUpdatedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
					return nil, nil
				},
				getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*wrappers.PeggyERC20DeployedEvent, error) {
					return nil, nil
				},
				getSendToInjectiveEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToInjectiveEvent, error) {
					return nil, nil
				},
			},
		}

		_, err := orch.relayEthEvents(context.TODO(), 100)
		assert.NoError(t, err)
		assert.Equal(t, inj.sendEthereumClaimsCallCount, 1)
	})
}
