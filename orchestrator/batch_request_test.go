package orchestrator

import (
	"context"
	"errors"
	"testing"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
)

func TestRequestBatches(t *testing.T) {
	t.Parallel()

	t.Run("UnbatchedTokenFees call fails", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
				unbatchedTokenFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.Nil(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
	})

	t.Run("no unbatched tokens", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			injective: &mockInjective{
				unbatchedTokenFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
					return nil, nil
				},
			},
		}

		assert.Nil(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
	})

	t.Run("batch does not meet fee threshold", func(t *testing.T) {
		t.Parallel()

		tokenAddr := ethcmn.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")

		injective := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggytypes.BatchFees{
					{
						Token:     tokenAddr.String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		orch := &PeggyOrchestrator{
			minBatchFeeUSD:       51.0,
			erc20ContractMapping: map[ethcmn.Address]string{tokenAddr: "inj"},
			pricefeed:            mockPriceFeed{queryFn: func(_ ethcmn.Address) (float64, error) { return 1, nil }},
			injective:            injective,
		}

		assert.Nil(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
		assert.Equal(t, injective.sendRequestBatchCallCount, 0)
	})

	t.Run("batch meets threshold and a request is sent", func(t *testing.T) {
		t.Parallel()

		tokenAddr := ethcmn.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")

		injective := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggytypes.BatchFees{
					{
						Token:     tokenAddr.String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		orch := &PeggyOrchestrator{
			minBatchFeeUSD:       49.0,
			erc20ContractMapping: map[ethcmn.Address]string{tokenAddr: "inj"},
			pricefeed: mockPriceFeed{queryFn: func(_ ethcmn.Address) (float64, error) {
				return 1, nil
			}},
			injective: injective,
		}

		assert.Nil(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
		assert.Equal(t, injective.sendRequestBatchCallCount, 1)
	})

}
