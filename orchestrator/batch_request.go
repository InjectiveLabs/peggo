package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/avast/retry-go"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	eth "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"
	"time"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	loop := batchRequestLoop{
		PeggyOrchestrator: s,
		loopDuration:      defaultLoopDur,
	}

	return loop.Run(ctx, s.injective, s.pricefeed)
}

type batchRequestLoop struct {
	*PeggyOrchestrator
	loopDuration time.Duration
}

func (l batchRequestLoop) Logger() log.Logger {
	return l.logger.WithField("loop", "BatchRequest")
}

func (l batchRequestLoop) Run(ctx context.Context, injective InjectiveNetwork, priceFeed PriceFeed) error {
	return loops.RunLoop(ctx, l.loopDuration, l.loopFn(ctx, injective, priceFeed))
}

func (l batchRequestLoop) loopFn(ctx context.Context, injective InjectiveNetwork, priceFeed PriceFeed) func() error {
	return func() error {
		fees, err := l.getUnbatchedTokenFees(ctx, injective)
		if err != nil {
			// non-fatal, just alert
			l.Logger().WithError(err).Warningln("unable to get unbatched fees from Injective")
			return nil
		}

		if len(fees) == 0 {
			l.Logger().Debugln("no token fees to batch")
			return nil
		}

		for _, fee := range fees {
			l.requestBatch(ctx, injective, priceFeed, fee)

			// todo: in case of multiple tokens, we should sleep in between batch requests (non-continuous nonce)
		}

		return nil
	}
}

func (l batchRequestLoop) getUnbatchedTokenFees(ctx context.Context, injective InjectiveNetwork) ([]*types.BatchFees, error) {
	var unbatchedFees []*types.BatchFees
	getUnbatchedTokenFeesFn := func() (err error) {
		unbatchedFees, err = injective.UnbatchedTokenFees(ctx)
		return err
	}

	if err := retry.Do(getUnbatchedTokenFeesFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Errorf("failed to get unbatched token fees, will retry (%d)", n)
		}),
	); err != nil {
		return nil, err
	}

	return unbatchedFees, nil
}

func (l batchRequestLoop) requestBatch(
	ctx context.Context,
	injective InjectiveNetwork,
	feed PriceFeed,
	fee *types.BatchFees,
) {
	var (
		tokenAddr = eth.HexToAddress(fee.Token)
		denom     = l.tokenDenom(tokenAddr)
	)

	if thresholdMet := l.checkFeeThreshold(feed, tokenAddr, fee.TotalFees); !thresholdMet {
		l.Logger().WithFields(log.Fields{
			"denom":          denom,
			"token_contract": tokenAddr.String(),
			"total_fees":     fee.TotalFees.String(),
		}).Debugln("skipping underpriced batch")
		return
	}

	l.Logger().WithFields(log.Fields{
		"denom":          denom,
		"token_contract": tokenAddr.String(),
	}).Infoln("requesting batch on Injective")

	_ = injective.SendRequestBatch(ctx, denom)
}

func (l batchRequestLoop) tokenDenom(tokenAddr eth.Address) string {
	if cosmosDenom, ok := l.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// peggy denom
	return types.PeggyDenomString(tokenAddr)
}

func (l batchRequestLoop) checkFeeThreshold(feed PriceFeed, tokenAddr eth.Address, fees cosmtypes.Int) bool {
	if l.minBatchFeeUSD == 0 {
		return true
	}

	tokenPriceInUSD, err := feed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	minFeeInUSDDec := decimal.NewFromFloat(l.minBatchFeeUSD)
	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(fees.BigInt(), -18).Mul(tokenPriceInUSDDec)

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		return false
	}

	return true
}

//
//type batchRequester struct {
//	log                  log.Logger
//	retries              uint
//	minBatchFee          float64
//	erc20ContractMapping map[eth.Address]string
//}
//
//func (r *batchRequester) run(
//	ctx context.Context,
//	injective InjectiveNetwork,
//	feed PriceFeed,
//) error {
//	r.log.WithField("min_batch_fee", r.minBatchFee).Infoln("scanning Injective for potential batches")
//
//	unbatchedFees, err := r.getUnbatchedFeesByToken(ctx, injective)
//	if err != nil {
//		// non-fatal, just alert
//		r.log.WithError(err).Warningln("unable to get unbatched fees from Injective")
//		return nil
//	}
//
//	if len(unbatchedFees) == 0 {
//		r.log.Debugln("no outgoing withdrawals or minimum batch fee is not met")
//		return nil
//	}
//
//	for _, tokenFee := range unbatchedFees {
//		r.requestBatchCreation(ctx, injective, feed, tokenFee)
//	}
//
//	return nil
//}
//
//func (r *batchRequester) getUnbatchedFeesByToken(ctx context.Context, injective InjectiveNetwork) ([]*types.BatchFees, error) {
//	var unbatchedFees []*types.BatchFees
//	retryFn := func() (err error) {
//		unbatchedFees, err = injective.UnbatchedTokenFees(ctx)
//		return err
//	}
//
//	if err := retry.Do(retryFn,
//		retry.Context(ctx),
//		retry.Attempts(r.retries),
//		retry.OnRetry(func(n uint, err error) {
//			log.WithError(err).Errorf("failed to get unbatched fees, will retry (%d)", n)
//		}),
//	); err != nil {
//		return nil, err
//	}
//
//	return unbatchedFees, nil
//}
//
//func (r *batchRequester) requestBatchCreation(
//	ctx context.Context,
//	injective InjectiveNetwork,
//	feed PriceFeed,
//	batchFee *types.BatchFees,
//) {
//	var (
//		tokenAddr = eth.HexToAddress(batchFee.Token)
//		denom     = r.tokenDenom(tokenAddr)
//	)
//
//	if thresholdMet := r.checkFeeThreshold(feed, tokenAddr, batchFee.TotalFees); !thresholdMet {
//		r.log.WithFields(log.Fields{
//			"denom":          denom,
//			"token_contract": tokenAddr.String(),
//			"total_fees":     batchFee.TotalFees.String(),
//		}).Debugln("skipping underpriced batch")
//		return
//	}
//
//	r.log.WithFields(log.Fields{
//		"denom":          denom,
//		"token_contract": tokenAddr.String(),
//	}).Infoln("requesting batch creation on Injective")
//
//	_ = injective.SendRequestBatch(ctx, denom)
//}
//
//func (r *batchRequester) tokenDenom(tokenAddr eth.Address) string {
//	if cosmosDenom, ok := r.erc20ContractMapping[tokenAddr]; ok {
//		return cosmosDenom
//	}
//
//	// peggy denom
//	return types.PeggyDenomString(tokenAddr)
//}
//
//func (r *batchRequester) checkFeeThreshold(
//	feed PriceFeed,
//	tokenAddr eth.Address,
//	totalFees cosmtypes.Int,
//) bool {
//	if r.minBatchFee == 0 {
//		return true
//	}
//
//	tokenPriceInUSD, err := feed.QueryUSDPrice(tokenAddr)
//	if err != nil {
//		return false
//	}
//
//	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
//	totalFeeInUSDDec := decimal.NewFromBigInt(totalFees.BigInt(), -18).Mul(tokenPriceInUSDDec)
//	minFeeInUSDDec := decimal.NewFromFloat(r.minBatchFee)
//
//	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
//		return true
//	}
//
//	return false
//}
