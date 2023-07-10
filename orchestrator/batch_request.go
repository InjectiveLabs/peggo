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

	unbatchedTokensWithFees, err := r.getBatchFeesByToken(ctx, injective)
	if err != nil {
		// non-fatal, just alert
		r.log.WithError(err).Warningln("unable to get unbatched fees from Injective")
		return nil
	}

	if len(unbatchedTokensWithFees) == 0 {
		r.log.Debugln("no outgoing withdrawals or minimum batch fee is not met")
		return nil
	}

	for _, unbatchedToken := range unbatchedTokensWithFees {
		r.requestBatchCreation(ctx, injective, feed, unbatchedToken)
	}

	return nil
}

func (r *batchRequester) getBatchFeesByToken(ctx context.Context, injective InjectiveNetwork) ([]*types.BatchFees, error) {
	var unbatchedTokensWithFees []*types.BatchFees
	retryFn := func() (err error) {
		unbatchedTokensWithFees, err = injective.UnbatchedTokenFees(ctx)
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

	return unbatchedTokensWithFees, nil
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
	)

	if thresholdMet := r.checkFeeThreshold(feed, tokenAddr, batchFee.TotalFees); !thresholdMet {
		r.log.WithFields(log.Fields{
			"denom":          denom,
			"token_contract": tokenAddr.String(),
			"total_fees":     batchFee.TotalFees.String(),
		}).Debugln("skipping underpriced batch")
		return
	}

	r.log.WithFields(log.Fields{
		"denom":          denom,
		"token_contract": tokenAddr.String(),
	}).Infoln("requesting batch creation on Injective")

	_ = injective.SendRequestBatch(ctx, denom)
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

	return false
}

func (s *PeggyOrchestrator) requestBatches(ctx context.Context, logger log.Logger, mustRequest bool) error {
	logger.WithField("min_batch_fee", s.minBatchFeeUSD).Infoln("scanning Injective for potential batches")

	unbatchedTokensWithFees, err := s.getBatchFeesByToken(ctx, logger)
	if err != nil {
		// non-fatal, just alert
		logger.WithError(err).Warningln("unable to get unbatched fees from Injective")
		return nil
	}

	if len(unbatchedTokensWithFees) == 0 {
		logger.WithField("min_fee", s.minBatchFeeUSD).Debugln("no outgoing withdrawals or minimum batch fee is not met")
		return nil
	}

	logger.WithField("unbatched_fees_by_token", unbatchedTokensWithFees).Debugln("checking if batch fee is met")
	for _, unbatchedToken := range unbatchedTokensWithFees {
		// check if the token is present in cosmos denom. if so, send batch request with cosmosDenom
		tokenAddr := eth.HexToAddress(unbatchedToken.Token)
		denom := s.getTokenDenom(tokenAddr)

		thresholdMet := s.checkFeeThreshold(tokenAddr, unbatchedToken.TotalFees, s.minBatchFeeUSD)
		if !thresholdMet && !mustRequest {
			//	non-injective relayers only relay when the threshold is met
			logger.WithFields(log.Fields{
				"denom":          denom,
				"token_contract": tokenAddr.String(),
			}).Debugln("skipping batch creation")
			continue
		}

		logger.WithFields(log.Fields{
			"denom":          denom,
			"token_contract": tokenAddr.String(),
		}).Infoln("sending MsgRequestBatch to Injective")

		_ = s.injective.SendRequestBatch(ctx, denom)
	}

	return nil
}

func (s *PeggyOrchestrator) getBatchFeesByToken(ctx context.Context, log log.Logger) ([]*types.BatchFees, error) {
	var unbatchedTokensWithFees []*types.BatchFees
	retryFn := func() error {
		fees, err := s.injective.UnbatchedTokenFees(ctx)
		if err != nil {
			return err
		}

		unbatchedTokensWithFees = fees
		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Errorf("failed to get unbatched fees, will retry (%d)", n)
		}),
	); err != nil {
		return nil, err
	}

	return unbatchedTokensWithFees, nil
}

func (s *PeggyOrchestrator) getTokenDenom(tokenAddr eth.Address) string {
	if cosmosDenom, ok := s.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// peggy denom
	return types.PeggyDenomString(tokenAddr)
}

func (s *PeggyOrchestrator) checkFeeThreshold(erc20Contract eth.Address, totalFee cosmtypes.Int, minFeeInUSD float64) bool {
	if minFeeInUSD == 0 {
		return true
	}

	tokenPriceInUSD, err := s.pricefeed.QueryUSDPrice(erc20Contract)
	if err != nil {
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFee.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(minFeeInUSD)

	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
		return true
	}
	return false
}
