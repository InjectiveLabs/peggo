package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"time"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

func (s *PeggyOrchestrator) BatchRequesterLoop(ctx context.Context, inj cosmos.Network, eth ethereum.Network) (err error) {
	requester := batchRequester{
		PeggyOrchestrator: s,
		Injective:         inj,
		Ethereum:          eth,
		LoopDuration:      defaultLoopDur,
	}

	s.logger.WithField("loop_duration", requester.LoopDuration.String()).Debugln("starting BatchRequester...")

	return loops.RunLoop(ctx, requester.LoopDuration, func() error {
		return requester.RequestBatches(ctx)
	})
}

type batchRequester struct {
	*PeggyOrchestrator
	Injective    cosmos.Network
	Ethereum     ethereum.Network
	LoopDuration time.Duration
}

func (l *batchRequester) Logger() log.Logger {
	return l.logger.WithField("loop", "BatchRequest")
}

func (l *batchRequester) RequestBatches(ctx context.Context) error {
	fees, err := l.getUnbatchedTokenFees(ctx)
	if err != nil {
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
}

func (l *batchRequester) getUnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	var unbatchedFees []*peggytypes.BatchFees
	fn := func() error {
		fees, err := l.Injective.UnbatchedTokensWithFees(ctx)
		if err != nil {
			return err
		}

		unbatchedFees = fees

		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), fn); err != nil {
		return nil, err
	}

	return unbatchedFees, nil
}

func (l *batchRequester) requestBatch(ctx context.Context, fee *peggytypes.BatchFees) {
	tokenAddr := gethcommon.HexToAddress(fee.Token)
	if thresholdMet := l.checkFeeThreshold(fee.TotalFees, tokenAddr); !thresholdMet {
		return
	}

	tokenDenom := l.tokenDenom(tokenAddr)
	l.Logger().WithFields(log.Fields{"denom": tokenDenom, "token_contract": tokenAddr.String()}).Infoln("requesting batch on Injective")

	_ = l.Injective.SendRequestBatch(ctx, tokenDenom)
}

func (l *batchRequester) tokenDenom(tokenAddr gethcommon.Address) string {
	if cosmosDenom, ok := l.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	return peggytypes.PeggyDenomString(tokenAddr)
}

func (l *batchRequester) checkFeeThreshold(fees cosmostypes.Int, tokenAddr gethcommon.Address) bool {
	if l.minBatchFeeUSD == 0 {
		return true
	}

	tokenPriceInUSD, err := l.priceFeed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	var (
		minFeeInUSDDec     = decimal.NewFromFloat(l.minBatchFeeUSD)
		tokenPriceInUSDDec = decimal.NewFromFloat(tokenPriceInUSD)
		totalFeeInUSDDec   = decimal.NewFromBigInt(fees.BigInt(), -18).Mul(tokenPriceInUSDDec) // todo: fix decimals
	)

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		l.Logger().WithFields(log.Fields{"token_contract": tokenAddr.String(), "batch_fee": totalFeeInUSDDec.String(), "min_fee": minFeeInUSDDec.String()}).Debugln("insufficient token batch fee")
		return false
	}

	return true
}
