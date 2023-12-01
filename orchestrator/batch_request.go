package orchestrator

import (
	"context"

	"github.com/avast/retry-go"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	eth "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	requester := &batchRequester{
		log:                  log.WithField("loop", "BatchRequester"),
		retries:              s.maxAttempts,
		minBatchFee:          s.minBatchFeeUSD,
		erc20ContractMapping: s.erc20ContractMapping,
	}

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		return requester.run(ctx, s.injective, s.pricefeed)
	})
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
	tokenFees, err := r.getUnbatchedFeesByToken(ctx, injective)
	if err != nil {
		r.log.WithError(err).Warningln("failed to get token fees from Injective")
		return nil
	}

	for _, fee := range tokenFees {
		tokenAddr := eth.HexToAddress(fee.Token)
		tokenPrice, err := feed.QueryUSDPrice(tokenAddr)
		if err != nil {
			log.WithError(err).WithField("token_contract", tokenAddr.String()).Warningln("failed to query token price")
			continue
		}

		if !checkMinFee(r.minBatchFee, tokenPrice, fee.TotalFees) {
			r.log.WithField("token_contract", tokenAddr.String()).Debugln("skipping token batch creation")
			continue
		}

		if err := injective.SendRequestBatch(ctx, r.tokenDenom(tokenAddr)); err != nil {
			r.log.WithError(err).Warningln("failed to create batch")
			continue
		}

		r.log.WithField("token_contract", tokenAddr.String()).Infoln("created new token batch on Injective")
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

	if err := retry.Do(getUnbatchedFeesFn,
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

func (r *batchRequester) tokenDenom(tokenAddr eth.Address) string {
	if cosmosDenom, ok := r.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// peggy denom
	return types.PeggyDenomString(tokenAddr)
}

// todo: remove from tests
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

func checkMinFee(minFee, tokenPriceInUSD float64, totalFees cosmtypes.Int) bool {
	if minFee == 0 {
		return true
	}

	totalTokensScaled := decimal.NewFromBigInt(totalFees.BigInt(), -18)
	totalFeeInUSDDec := totalTokensScaled.Mul(decimal.NewFromFloat(tokenPriceInUSD))
	minFeeInUSDDec := decimal.NewFromFloat(minFee)

	log.WithFields(log.Fields{
		"min_fee":   minFeeInUSDDec.String(),
		"total_fee": totalFeeInUSDDec.String(),
	}).Debugln("token batch fee check")

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		return false
	}

	return true
}
