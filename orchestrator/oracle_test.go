package orchestrator

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"

	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func TestEthOracle(t *testing.T) {
	t.Parallel()

	t.Run("failed to get latest header from ethereum", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			logger: suplog.DefaultLogger,
			eth: mockEthereum{
				headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Error(t, orch.EthOracleMainLoop(context.TODO()))
	})

	t.Run("latest ethereum header is old", func(t *testing.T) {
		t.Parallel()

		ethereum := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(50)}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         ethereum,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Now(),
			LastObservedEthHeight:   100,
		}

		assert.NoError(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(100))
	})

	t.Run("failed to get SendToCosmos events", func(t *testing.T) {
		t.Parallel()

		ethereum := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         ethereum,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Now(),
			LastObservedEthHeight:   100,
		}

		assert.Error(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(100))
	})

	t.Run("failed to get last claim event from injective", func(t *testing.T) {
		t.Parallel()

		ethereum := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},

			// no-ops
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
				return nil, nil
			},
			getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
				return nil, nil
			},
			getValsetUpdatedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
				return nil, nil
			},
			getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
				return nil, nil
			},
			getSendToInjectiveEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
				return nil, nil
			},
		}

		injective := &mockInjective{
			lastClaimEventFn: func(context.Context) (*peggytypes.LastClaimEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         ethereum,
			inj:         injective,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Now(),
			LastObservedEthHeight:   100,
		}

		assert.Error(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(100))
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
				[]*peggyevents.PeggySendToCosmosEvent,
				[]*peggyevents.PeggySendToInjectiveEvent,
				[]*peggyevents.PeggyTransactionBatchExecutedEvent,
				[]*peggyevents.PeggyERC20DeployedEvent,
				[]*peggyevents.PeggyValsetUpdatedEvent,
			) error {
				return nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
				return []*peggyevents.PeggySendToCosmosEvent{{EventNonce: big.NewInt(5)}}, nil
			},

			// no-ops
			getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
				return nil, nil
			},
			getValsetUpdatedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
				return nil, nil
			},
			getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
				return nil, nil
			},
			getSendToInjectiveEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
				return nil, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         eth,
			inj:         inj,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Now(),
			LastObservedEthHeight:   100,
		}

		assert.NoError(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(104))
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
				[]*peggyevents.PeggySendToCosmosEvent,
				[]*peggyevents.PeggySendToInjectiveEvent,
				[]*peggyevents.PeggyTransactionBatchExecutedEvent,
				[]*peggyevents.PeggyERC20DeployedEvent,
				[]*peggyevents.PeggyValsetUpdatedEvent,
			) error {
				return nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
				return []*peggyevents.PeggySendToCosmosEvent{{EventNonce: big.NewInt(10)}}, nil
			},

			// no-ops
			getTransactionBatchExecutedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
				return nil, nil
			},
			getValsetUpdatedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
				return nil, nil
			},
			getPeggyERC20DeployedEventsFn: func(uint64, uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
				return nil, nil
			},
			getSendToInjectiveEventsFn: func(uint64, uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
				return nil, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         eth,
			inj:         inj,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Now(),
			LastObservedEthHeight:   100,
		}

		assert.NoError(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(104))
		assert.Equal(t, inj.sendEthereumClaimsCallCount, 1)
	})

	t.Run("auto resync", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			lastClaimEventFn: func(_ context.Context) (*peggytypes.LastClaimEvent, error) {
				return &peggytypes.LastClaimEvent{EthereumEventHeight: 101}, nil
			},
		}

		eth := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(50)}, nil
			},
		}

		o := &PeggyOrchestrator{
			logger:      suplog.DefaultLogger,
			eth:         eth,
			inj:         inj,
			maxAttempts: 1,
		}

		loop := ethOracle{
			PeggyOrchestrator:       o,
			LastResyncWithInjective: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
			LastObservedEthHeight:   100,
		}

		assert.NoError(t, loop.observeEthEvents(context.TODO()))
		assert.Equal(t, loop.LastObservedEthHeight, uint64(101))
		assert.True(t, time.Since(loop.LastResyncWithInjective) < 1*time.Second)
	})
}
