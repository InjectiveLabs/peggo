package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
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
	}

	s.logger.WithField("loop_duration", defaultLoopDur.String()).Debugln("starting BatchRequester...")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		return requester.RequestBatches(ctx)
	})
}

type batchRequester struct {
	*PeggyOrchestrator
	Injective cosmos.Network
	Ethereum  ethereum.Network
}

func (l *batchRequester) Logger() log.Logger {
	return l.logger.WithField("loop", "BatchRequest")
}

func (l *batchRequester) RequestBatches(ctx context.Context) error {
	fees, err := l.GetUnbatchedTokenFees(ctx)
	if err != nil {
		l.Logger().WithError(err).Warningln("unable to get outgoing withdrawal fees")
		return nil
	}

	if len(fees) == 0 {
		l.Logger().Infoln("no withdrawals to batch")
		return nil
	}

	for _, fee := range fees {
		l.RequestTokenBatch(ctx, fee)
	}

	return nil
}

func (l *batchRequester) GetUnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
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

func (l *batchRequester) RequestTokenBatch(ctx context.Context, fee *peggytypes.BatchFees) {
	tokenContract := gethcommon.HexToAddress(fee.Token)
	tokenPriceInUSD, err := l.priceFeed.QueryUSDPrice(tokenContract)
	if err != nil {
		l.Logger().WithError(err).Warningln("failed to query oracle for token price")
		return
	}

	tokenDecimals, err := l.Ethereum.TokenDecimals(ctx, tokenContract)
	if err != nil {
		l.Logger().WithError(err).Warningln("failed to query decimals from token contract")
		return
	}

	if l.CheckMinBatchFee(fee, tokenPriceInUSD, tokenDecimals) {
		return
	}

	tokenDenom := l.GetTokenDenom(tokenContract)
	l.Logger().WithFields(log.Fields{"token_denom": tokenDenom, "token_contract": tokenContract.String()}).Infoln("requesting new token batch on Injective")

	_ = l.Injective.SendRequestBatch(ctx, tokenDenom)
}

func (l *batchRequester) GetTokenDenom(tokenAddr gethcommon.Address) string {
	if cosmosDenom, ok := l.erc20ContractMapping[tokenAddr]; ok {
		return cosmosDenom
	}

	return peggytypes.PeggyDenomString(tokenAddr)
}

func (l *batchRequester) CheckMinBatchFee(fee *peggytypes.BatchFees, tokenPriceInUSD float64, tokenDecimals uint8) bool {
	if l.minBatchFeeUSD == 0 {
		return true
	}

	var (
		minFeeInUSDDec     = decimal.NewFromFloat(l.minBatchFeeUSD)
		tokenPriceInUSDDec = decimal.NewFromFloat(tokenPriceInUSD)
		totalFeeInUSDDec   = decimal.NewFromBigInt(fee.TotalFees.BigInt(), -1*int32(tokenDecimals)).Mul(tokenPriceInUSDDec)
	)

	if totalFeeInUSDDec.LessThan(minFeeInUSDDec) {
		l.Logger().WithFields(log.Fields{"token_contract": fee.Token, "total_fee": totalFeeInUSDDec.String(), "min_fee": minFeeInUSDDec.String()}).Debugln("insufficient fee for token batch request, skipping...")
		return false
	}

	return true
}
