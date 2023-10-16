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
	r.log.WithField("min_batch_fee", r.minBatchFee).Infoln("scanning Injective for potential batches")

	unbatchedFees, err := r.getUnbatchedFeesByToken(ctx, injective)
	if err != nil {
		// non-fatal, just alert
		r.log.WithError(err).Warningln("unable to get unbatched fees from Injective")
		return nil
	}

	if len(unbatchedFees) == 0 {
		r.log.Debugln("no outgoing withdrawals or minimum batch fee is not met")
		return nil
	}

	for _, tokenFee := range unbatchedFees {
		r.requestBatchCreation(ctx, injective, feed, tokenFee)
	}

	return nil
}

func (r *batchRequester) getUnbatchedFeesByToken(ctx context.Context, injective InjectiveNetwork) ([]*types.BatchFees, error) {
	var unbatchedFees []*types.BatchFees
	retryFn := func() (err error) {
		unbatchedFees, err = injective.UnbatchedTokenFees(ctx)
		return err
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(r.retries),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Errorf("failed to get unbatched fees, will retry (%d)", n)
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
		tokenAddr  = eth.HexToAddress(batchFee.Token)
		tokenDenom = r.tokenDenom(tokenAddr)
	)

	if !checkPriceThreshold(
		feed,
		tokenAddr,
		batchFee.TotalFees,
		r.minBatchFee,
	) {
		r.log.WithFields(log.Fields{
			"token_denom":    tokenDenom,
			"token_contract": tokenAddr.String(),
			"total_fees":     batchFee.TotalFees.String(),
		}).Debugln("skipping underpriced batch")
		return
	}

	r.log.WithFields(log.Fields{
		"token_denom":    tokenDenom,
		"token_contract": tokenAddr.String(),
	}).Infoln("requesting batch creation on Injective")

	_ = injective.SendRequestBatch(ctx, tokenDenom)
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

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		return false
	}

	return true
}

func checkPriceThreshold(
	feed PriceFeed,
	tokenAddr eth.Address,
	totalFees cosmtypes.Int,
	minFee float64,
) bool {
	if minFee == 0 {
		return true
	}

	tokenPriceInUSD, err := feed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFees.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(minFee)

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		return false
	}

	return true
}
