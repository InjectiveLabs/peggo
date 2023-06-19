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

func TestLogger(t *testing.T) {
	err := errors.New("dusan error")
	err2 := errors.New("another dusan error")

	logger := suplog.WithError(err).WithError(errors.New("wqerwerw d"))

	suplog.Infoln("random info line")
	suplog.WithFields(suplog.Fields{"field1": 42}).Infoln("info line with field")

	logger.Errorln("descriptive error line")
	logger.WithError(err2).Errorln("descriptive error line 2")

	logger = suplog.WithField("dusan", "dusan value")
	logger.Errorln("this is an error line")
	logger.Infoln("this is an info line")
	logger.Info("this is an info log")
	num := 10
	logger.Debugln("this", "is", "a", "debug", "log", "with num=", num)
	num2 := 11
	logger.WithFields(suplog.Fields{"field1": num, "field2": num2}).Warningln("warning with fields")

	//suplog.WithError(err).Fatalln("failed to initialize Injective keyring")

	suplog.WithFields(suplog.Fields{"chain_id": "888"}).Infoln("Connected to Injective chain")

	suplog.WithFields(suplog.Fields{
		"chain_id":       "*cfg.cosmosChainID",
		"injective_grpc": "*cfg.cosmosGRPC",
		"tendermint_rpc": "cfg.tendermintRPC",
	}).Infoln("connected to Injective network")
}

func TestRequestBatches(t *testing.T) {
	t.Parallel()

	t.Run("failed to get unbatched tokens from injective", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxAttempts: 1,
			injective: &mockInjective{
				unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
					return nil, errors.New("fail")
				},
			},
		}

		assert.NoError(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
	})

	t.Run("no unbatched tokens", func(t *testing.T) {
		t.Parallel()

		orch := &PeggyOrchestrator{
			maxAttempts: 1,
			injective: &mockInjective{
				unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
					return nil, nil
				},
			},
		}

		assert.NoError(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
	})

	t.Run("batch does not meet fee threshold", func(t *testing.T) {
		t.Parallel()

		tokenAddr := eth.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")

		injective := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(context.Context) ([]*peggy.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggy.BatchFees{
					{
						Token:     tokenAddr.String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		orch := &PeggyOrchestrator{
			maxAttempts:          1,
			minBatchFeeUSD:       51.0,
			erc20ContractMapping: map[eth.Address]string{tokenAddr: "inj"},
			pricefeed:            mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) { return 1, nil }},
			injective:            injective,
		}

		assert.NoError(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
		assert.Equal(t, injective.sendRequestBatchCallCount, 0)
	})

	t.Run("batch meets threshold and a request is sent", func(t *testing.T) {
		t.Parallel()

		tokenAddr := eth.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30")

		injective := &mockInjective{
			sendRequestBatchFn: func(context.Context, string) error { return nil },
			unbatchedTokenFeesFn: func(_ context.Context) ([]*peggy.BatchFees, error) {
				fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
				return []*peggy.BatchFees{
					{
						Token:     tokenAddr.String(),
						TotalFees: fees,
					},
				}, nil
			},
		}

		orch := &PeggyOrchestrator{
			maxAttempts:          1,
			minBatchFeeUSD:       49.0,
			erc20ContractMapping: map[eth.Address]string{tokenAddr: "inj"},
			pricefeed: mockPriceFeed{queryFn: func(_ eth.Address) (float64, error) {
				return 1, nil
			}},
			injective: injective,
		}

		assert.NoError(t, orch.requestBatches(context.TODO(), suplog.DefaultLogger, false))
		assert.Equal(t, injective.sendRequestBatchCallCount, 1)
	})

}
