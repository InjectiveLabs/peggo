package orchestrator

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"

	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

func TestEthOracle(t *testing.T) {
	t.Parallel()

	t.Run("failed to get latest header from ethereum", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
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

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Now(),
			lastCheckedEthHeight:    100,
		}

		assert.NoError(t, o.run(context.TODO(), nil, ethereum))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(38))
	})

	t.Run("failed to get SendToCosmos events", func(t *testing.T) {
		t.Parallel()

		ethereum := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Now(),
			lastCheckedEthHeight:    100,
		}

		assert.Error(t, o.run(context.TODO(), nil, ethereum))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(100))
	})

	t.Run("failed to get last claim event from injective", func(t *testing.T) {
		t.Parallel()

		ethereum := mockEthereum{
			headerByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
				return &types.Header{Number: big.NewInt(200)}, nil
			},

			// no-ops
			getSendToCosmosEventsFn: func(uint64, uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
				return nil, nil
			},
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
		}

		injective := &mockInjective{
			lastClaimEventFn: func(context.Context) (*peggytypes.LastClaimEvent, error) {
				return nil, errors.New("fail")
			},
		}

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Now(),
			lastCheckedEthHeight:    100,
		}

		assert.Error(t, o.run(context.TODO(), injective, ethereum))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(100))
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

		eth := mockEthereum{
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
		}

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Now(),
			lastCheckedEthHeight:    100,
		}

		assert.NoError(t, o.run(context.TODO(), inj, eth))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(120))
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

		eth := mockEthereum{
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
		}

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Now(),
			lastCheckedEthHeight:    100,
		}

		assert.NoError(t, o.run(context.TODO(), inj, eth))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(120))
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

		o := &ethOracle{
			log:                     suplog.DefaultLogger,
			retries:                 1,
			lastResyncWithInjective: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
			lastCheckedEthHeight:    100,
		}

		assert.NoError(t, o.run(context.TODO(), inj, eth))
		assert.Equal(t, o.lastCheckedEthHeight, uint64(101))
		assert.True(t, time.Since(o.lastResyncWithInjective) < 1*time.Second)
	})
}
