package orchestrator

import (
	"context"
	"errors"
	"testing"

	eth "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"

	peggy "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
)

func TestRequestBatches(t *testing.T) {
	t.Parallel()

	t.Run("failed to get unbatched tokens from injective", func(t *testing.T) {
		t.Parallel()

		r := &batchRequester{
			log:     suplog.DefaultLogger,
			retries: 1,
		}

		inj := &mockInjective{
			unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
				return nil, errors.New("fail")
			},
		}
		feed := mockPriceFeed{}

		assert.NoError(t, r.run(context.TODO(), inj, feed))
	})

	t.Run("no unbatched tokens", func(t *testing.T) {
		t.Parallel()

		r := &batchRequester{
			log:     suplog.DefaultLogger,
			retries: 1,
		}

		inj := &mockInjective{
			unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
				return nil, nil
			},
		}
		feed := mockPriceFeed{}

		assert.NoError(t, r.run(context.TODO(), inj, feed))
	})

	t.Run("batch does not meet fee threshold", func(t *testing.T) {
		t.Parallel()

		tokenAddr := "0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"

		r := &batchRequester{
			log:         suplog.DefaultLogger,
			minBatchFee: 51.0,
			retries:     1,
			erc20ContractMapping: map[eth.Address]string{
				eth.HexToAddress(tokenAddr): "inj",
			},
		}

		inj := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggy.BatchFees{
					{
						Token:     eth.HexToAddress(tokenAddr).String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		feed := mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) { return 1, nil }}

		assert.NoError(t, r.run(context.TODO(), inj, feed))
		assert.Equal(t, inj.sendRequestBatchCallCount, 0)
	})

	t.Run("batch meets threshold and a request is sent", func(t *testing.T) {
		t.Parallel()

		tokenAddr := "0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"

		r := &batchRequester{
			log:         suplog.DefaultLogger,
			minBatchFee: 49.0,
			retries:     1,
			erc20ContractMapping: map[eth.Address]string{
				eth.HexToAddress(tokenAddr): "inj",
			},
		}

		inj := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(_ context.Context) ([]*peggy.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggy.BatchFees{
					{
						Token:     eth.HexToAddress(tokenAddr).String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		feed := mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) { return 1, nil }}

		assert.NoError(t, r.run(context.TODO(), inj, feed))
		assert.Equal(t, inj.sendRequestBatchCallCount, 1)
	})

}
