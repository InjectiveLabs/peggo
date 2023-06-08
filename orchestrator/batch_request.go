package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "BatchRequesterLoop")
	startTime := time.Now()

	// we're the only ones relaying
	isInjectiveRelayer := s.periodicBatchRequesting

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		mustRequestBatch := false
		if isInjectiveRelayer && time.Since(startTime) > time.Hour*8 {
			mustRequestBatch = true
		}

		var pg loops.ParanoidGroup
		pg.Go(func() error { return s.requestBatches(ctx, logger, mustRequestBatch) })
		return pg.Wait()
	})
}

func (s *PeggyOrchestrator) requestBatches(ctx context.Context, logger log.Logger, mustRequest bool) error {
	unbatchedTokensWithFees, err := s.getBatchFeesByToken(ctx, logger)
	if err != nil {
		// non-fatal, just alert
		logger.Warningln("unable to get UnbatchedTokensWithFees for the token")
		return nil
	}

	if len(unbatchedTokensWithFees) == 0 {
		logger.Debugln("No outgoing withdraw tx or Unbatched token fee less than threshold")
		return nil
	}

	logger.WithField("unbatchedTokensWithFees", unbatchedTokensWithFees).Debugln("Check if token fees meets set threshold amount and send batch request")
	for _, unbatchedToken := range unbatchedTokensWithFees {
		// check if the token is present in cosmos denom. if so, send batch request with cosmosDenom
		tokenAddr := ethcmn.HexToAddress(unbatchedToken.Token)

		thresholdMet := s.CheckFeeThreshold(tokenAddr, unbatchedToken.TotalFees, s.minBatchFeeUSD)
		if !thresholdMet && !mustRequest {
			//	non injective relayers only relay when the threshold is met
			continue
		}

		denom := s.getTokenDenom(tokenAddr)
		logger.WithFields(log.Fields{"tokenContract": tokenAddr, "denom": denom}).Infoln("sending batch request")
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

	if err := retry.Do(
		retryFn,
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Errorf("failed to get UnbatchedTokensWithFees, will retry (%d)", n)
		}),
	); err != nil {
		return nil, err
	}

	return unbatchedTokensWithFees, nil
}

func (s *PeggyOrchestrator) getTokenDenom(tokenAddr ethcmn.Address) string {
	if cosmosDenom, ok := s.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// peggy denom
	return types.PeggyDenomString(tokenAddr)
}

func (s *PeggyOrchestrator) CheckFeeThreshold(erc20Contract ethcmn.Address, totalFee cosmtypes.Int, minFeeInUSD float64) bool {
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
