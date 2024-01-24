package orchestrator

import (
	"context"
	"time"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/avast/retry-go"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	loop := batchRequestLoop{
		PeggyOrchestrator: s,
		loopDuration:      defaultLoopDur,
	}

	return loop.Run(ctx)
}

type batchRequestLoop struct {
	*PeggyOrchestrator
	loopDuration time.Duration
}

func (l *batchRequestLoop) Logger() log.Logger {
	return l.logger.WithField("loop", "BatchRequest")
}

func (l *batchRequestLoop) Run(ctx context.Context) error {
	l.logger.WithField("loop_duration", l.loopDuration.String()).Debugln("starting BatchRequester loop...")

	return loops.RunLoop(ctx, l.loopDuration, func() error {
		fees, err := l.getUnbatchedTokenFees(ctx)
		if err != nil {
			// non-fatal, just alert
			l.Logger().WithError(err).Warningln("unable to get outgoing withdrawal fees")
			return nil
		}

		if len(fees) == 0 {
			l.Logger().Infoln("no withdrawals to batch")
			return nil
		}

		for _, fee := range fees {
			l.requestBatch(ctx, fee)
		}

		return nil
	})
}

func (l *batchRequestLoop) getUnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	var unbatchedFees []*peggytypes.BatchFees
	getUnbatchedTokenFeesFn := func() (err error) {
		unbatchedFees, err = l.inj.UnbatchedTokensWithFees(ctx)
		return err
	}

	if err := retry.Do(getUnbatchedTokenFeesFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Errorf("failed to get outgoing withdrawal fees, will retry (%d)", n)
		}),
	); err != nil {
		return nil, err
	}

	return unbatchedFees, nil
}

func (l *batchRequestLoop) requestBatch(ctx context.Context, fee *peggytypes.BatchFees) {
	var (
		tokenAddr  = gethcommon.HexToAddress(fee.Token)
		tokenDenom = l.tokenDenom(tokenAddr)
	)

	if thresholdMet := l.checkFeeThreshold(tokenAddr, fee.TotalFees); !thresholdMet {
		return
	}

	l.Logger().WithFields(log.Fields{"denom": tokenDenom, "token_contract": tokenAddr.String()}).Infoln("requesting batch on Injective")

	_ = l.inj.SendRequestBatch(ctx, tokenDenom)
}

func (l *batchRequestLoop) tokenDenom(tokenAddr gethcommon.Address) string {
	if cosmosDenom, ok := l.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	// todo: revisit peggy denom addresses
	return peggytypes.PeggyDenomString(tokenAddr)
}

func (l *batchRequestLoop) checkFeeThreshold(tokenAddr gethcommon.Address, fees cosmostypes.Int) bool {
	if l.minBatchFeeUSD == 0 {
		return true
	}

	tokenPriceInUSD, err := l.pricefeed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	minFeeInUSDDec := decimal.NewFromFloat(l.minBatchFeeUSD)
	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(fees.BigInt(), -18).Mul(tokenPriceInUSDDec)

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		l.Logger().WithFields(log.Fields{"token_contract": tokenAddr.String(), "batch_fee": totalFeeInUSDDec.String(), "min_fee": minFeeInUSDDec.String()}).Debugln("insufficient token batch fee")
		return false
	}

	return true
}
