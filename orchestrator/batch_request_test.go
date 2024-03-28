package orchestrator

import (
	"context"
	"errors"
	"testing"

	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	eth "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func TestRequestBatches(t *testing.T) {
	t.Parallel()

	t.Run("failed to get unbatched tokens from injective", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			unbatchedTokenFeesFn: func(context.Context) ([]*peggytypes.BatchFees, error) {
				return nil, errors.New("fail")
			},
		}

		o := &Orchestrator{
			logger:      suplog.DefaultLogger,
			inj:         inj,
			maxAttempts: 1,
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		assert.NoError(t, loop.requestBatches(context.TODO()))
	})

	t.Run("no unbatched tokens", func(t *testing.T) {
		t.Parallel()

		inj := &mockInjective{
			unbatchedTokenFeesFn: func(context.Context) ([]*peggytypes.BatchFees, error) {
				return nil, nil
			},
		}

		o := &Orchestrator{
			logger:      suplog.DefaultLogger,
			inj:         inj,
			maxAttempts: 1,
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		assert.NoError(t, loop.requestBatches(context.TODO()))

	})

	t.Run("batch does not meet fee threshold", func(t *testing.T) {
		t.Parallel()

		tokenAddr := "0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"

		inj := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(context.Context) ([]*peggytypes.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggytypes.BatchFees{
					{
						Token:     eth.HexToAddress(tokenAddr).String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		feed := mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) { return 1, nil }}

		o := &Orchestrator{
			logger:         suplog.DefaultLogger,
			inj:            inj,
			priceFeed:      feed,
			maxAttempts:    1,
			minBatchFeeUSD: 51.0,
			erc20ContractMapping: map[eth.Address]string{
				eth.HexToAddress(tokenAddr): "inj",
			},
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		assert.NoError(t, loop.requestBatches(context.TODO()))
		assert.Equal(t, inj.sendRequestBatchCallCount, 0)
	})

	t.Run("batch meets threshold and a request is sent", func(t *testing.T) {
		t.Parallel()

		tokenAddr := "0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"

		inj := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggytypes.BatchFees{
					{
						Token:     eth.HexToAddress(tokenAddr).String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		feed := mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) { return 1, nil }}

		o := &Orchestrator{
			logger:         suplog.DefaultLogger,
			inj:            inj,
			priceFeed:      feed,
			maxAttempts:    1,
			minBatchFeeUSD: 49.0,
			erc20ContractMapping: map[eth.Address]string{
				eth.HexToAddress(tokenAddr): "inj",
			},
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		assert.NoError(t, loop.requestBatches(context.TODO()))
		assert.Equal(t, inj.sendRequestBatchCallCount, 1)
	})

}

func TestCheckFeeThreshold(t *testing.T) {
	t.Parallel()

	t.Run("fee threshold is met", func(t *testing.T) {
		t.Parallel()

		var (
			totalFees, _ = cosmtypes.NewIntFromString("10000000000000000000") // 10inj
			tokenAddr    = eth.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")
			feed         = mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) {
				return 2.5, nil
			}}
		)

		o := &Orchestrator{
			logger:         suplog.DefaultLogger,
			priceFeed:      feed,
			minBatchFeeUSD: 21,
			erc20ContractMapping: map[eth.Address]string{
				tokenAddr: "inj",
			},
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		// 2.5 * 10 > 21
		assert.True(t, loop.checkFeeThreshold(tokenAddr, totalFees))
	})

	t.Run("fee threshold is met", func(t *testing.T) {
		t.Parallel()

		var (
			tokenAddr    = eth.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")
			totalFees, _ = cosmtypes.NewIntFromString("100000000000000000000") // 10inj
			feed         = mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) {
				return 2.5, nil
			}}
		)

		o := &Orchestrator{
			logger:         suplog.DefaultLogger,
			priceFeed:      feed,
			minBatchFeeUSD: 333.333,
			erc20ContractMapping: map[eth.Address]string{
				tokenAddr: "inj",
			},
		}

		loop := batchRequester{
			Orchestrator: o,
		}

		// 2.5 * 100 < 333.333
		assert.False(t, loop.checkFeeThreshold(tokenAddr, totalFees))
	})
}
