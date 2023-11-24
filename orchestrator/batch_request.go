package orchestrator

import (
	"context"
	"github.com/avast/retry-go"
	eth "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	requester := &batchRequester{
		log:                  log.WithField("loop", "BatchRequester"),
		retries:              s.maxAttempts,
		minBatchFee:          s.minBatchFeeUSD,
		erc20ContractMapping: s.erc20ContractMapping,
	}

	return loops.RunLoop(
		ctx,
		defaultLoopDur,
		func() error { return requester.run(ctx, s.injective, s.pricefeed) },
	)
}

type batchRequester struct {
	log                  log.Logger
	retries              uint
	minBatchFee          float64
	erc20ContractMapping map[eth.Address]string
}

func (r *batchRequester) run(
	ctx context.Context,
	injective InjectiveNetwork,
	feed PriceFeed,
) error {
	unbatchedFees, err := r.getUnbatchedFeesByToken(ctx, injective)
	if err != nil {
		// non-fatal, just alert
		r.log.WithError(err).Warningln("unable to get unbatched fees from Injective")
		return nil
	}

	for _, tokenFee := range unbatchedFees {
		r.requestBatchCreation(ctx, injective, feed, tokenFee)
	}

	return nil
}

func (r *batchRequester) getUnbatchedFeesByToken(ctx context.Context, injective InjectiveNetwork) ([]*types.BatchFees, error) {
	var (
		unbatchedFees      []*types.BatchFees
		getUnbatchedFeesFn = func() (err error) {
			unbatchedFees, err = injective.UnbatchedTokenFees(ctx)
			return err
		}
	)

	if err := retry.Do(
		getUnbatchedFeesFn,
		retry.Context(ctx),
		retry.Attempts(r.retries),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Warningf("failed to get unbatched fees, will retry (%d)", n)
		}),
	); err != nil {
		return nil, err
	}

	return unbatchedFees, nil
}

func (r *batchRequester) requestBatchCreation(
	ctx context.Context,
	injective InjectiveNetwork,
	feed PriceFeed,
	batchFee *types.BatchFees,
) {
	var (
		tokenAddr = eth.HexToAddress(batchFee.Token)
		denom     = r.tokenDenom(tokenAddr)
		fees      = batchFee.TotalFees
	)

	if thresholdMet := r.checkFeeThreshold(feed, tokenAddr, fees); !thresholdMet {
		return
	}

	r.log.WithFields(log.Fields{
		"denom":          denom,
		"token_contract": tokenAddr.String(),
		"fees":           fees.String(),
	}).Infoln("creating new token batch on Injective")

	if err := injective.SendRequestBatch(ctx, denom); err != nil {
		r.log.WithError(err).Warningln("failed to create batch")
	}
}

func (r *batchRequester) tokenDenom(tokenAddr eth.Address) string {
	if cosmosDenom, ok := r.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// peggy denom
	return types.PeggyDenomString(tokenAddr)
}

func (r *batchRequester) checkFeeThreshold(
	feed PriceFeed,
	tokenAddr eth.Address,
	totalFees cosmtypes.Int,
) bool {
	if r.minBatchFee == 0 {
		return true
	}

	tokenPriceInUSD, err := feed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFees.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(r.minBatchFee)

	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
		return true
	}

	r.log.WithFields(log.Fields{
		"token_contract": tokenAddr.String(),
		"token_denom":    r.tokenDenom(tokenAddr),
		"batch_fee":      totalFeeInUSDDec.String(),
		"min_fee":        r.minBatchFee,
	}).Debugln("skipping underpriced batch")

	return false
}
