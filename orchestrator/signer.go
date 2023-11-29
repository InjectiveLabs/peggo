package orchestrator

import (
	"context"
	"github.com/pkg/errors"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Injective node they can simply be assumed to be
// valid and signed off on.
func (s *PeggyOrchestrator) EthSignerMainLoop(ctx context.Context) error {
	peggyID, err := s.getPeggyID(ctx)
	if err != nil {
		return err
	}

	signer := &ethSigner{
		log:         log.WithField("loop", "EthSigner"),
		peggyID:     peggyID,
		ethFrom:     s.ethereum.FromAddress(),
		retries:     s.maxAttempts,
		minBatchFee: s.minBatchFeeUSD,
	}

	return loops.RunLoop(
		ctx,
		defaultLoopDur,
		func() error { return signer.run(ctx, s.injective, s.pricefeed) },
	)
}

func (s *PeggyOrchestrator) getPeggyID(ctx context.Context) (common.Hash, error) {
	var peggyID common.Hash
	retryFn := func() (err error) {
		peggyID, err = s.ethereum.GetPeggyID(ctx)
		return err
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Warningf("failed to get peggy ID from Ethereum contract, will retry (%d)", n)
		}),
	); err != nil {
		log.WithError(err).Errorln("got error, loop exits")
		return [32]byte{}, err
	}

	log.WithField("id", peggyID.Hex()).Debugln("got peggy ID from Ethereum contract")

	return peggyID, nil
}

type ethSigner struct {
	log         log.Logger
	peggyID     common.Hash
	ethFrom     common.Address
	retries     uint
	minBatchFee float64
}

func (s *ethSigner) run(
	ctx context.Context,
	injective InjectiveNetwork,
	feed PriceFeed,
) error {
	if err := s.signNewValsetUpdates(ctx, injective); err != nil {
		return err
	}

	if err := s.signNewBatches(ctx, injective, feed); err != nil {
		return err
	}

	return nil
}

func (s *ethSigner) signNewBatches(
	ctx context.Context,
	injective InjectiveNetwork,
	feed PriceFeed,
) error {
	unsignedBatches, err := s.getUnsignedBatches(ctx, injective)
	if err != nil {
		return err
	}

	for _, b := range unsignedBatches {
		var (
			totalFee  = cosmtypes.ZeroInt()
			tokenAddr = common.HexToAddress(b.TokenContract)
		)

		for _, tx := range b.Transactions {
			totalFee.Add(tx.Erc20Fee.Amount)
		}

		if !checkPriceThreshold(feed, tokenAddr, totalFee, s.minBatchFee) {
			s.log.WithFields(log.Fields{
				"token_contract": tokenAddr.String(),
				"batch_fee":      totalFee.String(),
				"min_fee":        s.minBatchFee,
			}).Debugln("skipping token batch confirmation")
			continue //	skip underpriced batch
		}

		if err := s.signBatch(ctx, injective, b); err != nil {
			return err
		}
	}

	return nil
}

func (s *ethSigner) getUnsignedBatches(ctx context.Context, injective InjectiveNetwork) ([]*types.OutgoingTxBatch, error) {
	var (
		unsignedBatches      []*types.OutgoingTxBatch
		getUnsignedBatchesFn = func() (err error) {
			unsignedBatches, err = injective.UnconfirmedTransactionBatches(ctx)
			if errors.Is(err, cosmos.ErrNotFound) || len(unsignedBatches) == 0 {
				return nil
			}

			return err
		}
	)

	if err := retry.Do(getUnsignedBatchesFn,
		retry.Context(ctx),
		retry.Attempts(s.retries),
		retry.OnRetry(func(n uint, err error) {
			s.log.WithError(err).Warningf("failed to get unconfirmed batch, will retry (%d)", n)
		}),
	); err != nil {
		s.log.WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return unsignedBatches, nil
}

func (s *ethSigner) checkFeeThreshold(batch *types.OutgoingTxBatch, feed PriceFeed) bool {
	if s.minBatchFee == 0 {
		return true
	}

	tokenAddr := common.HexToAddress(batch.TokenContract)

	var totalFees cosmtypes.Int
	for _, tx := range batch.Transactions {
		totalFees.Add(tx.Erc20Fee.Amount)
	}

	tokenPriceInUSD, err := feed.QueryUSDPrice(tokenAddr)
	if err != nil {
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFees.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(s.minBatchFee)

	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
		return true
	}

	return false
}

func (s *ethSigner) signBatch(
	ctx context.Context,
	injective InjectiveNetwork,
	batch *types.OutgoingTxBatch,
) error {
	if err := retry.Do(
		func() error { return injective.SendBatchConfirm(ctx, s.peggyID, batch, s.ethFrom) },
		retry.Context(ctx),
		retry.Attempts(s.retries),
		retry.OnRetry(func(n uint, err error) {
			s.log.WithError(err).Warningf("failed to confirm batch on Injective, will retry (%d)", n)
		}),
	); err != nil {
		s.log.WithError(err).Errorln("got error, loop exits")
		return err
	}

	s.log.WithFields(log.Fields{
		"batch_nonce": batch.BatchNonce,
		"batch_txs":   len(batch.Transactions),
	}).Infoln("confirmed token batch on Injective")

	return nil
}

func (s *ethSigner) signNewValsetUpdates(
	ctx context.Context,
	injective InjectiveNetwork,
) error {
	oldestUnsignedValsets, err := s.getUnsignedValsets(ctx, injective)
	if err != nil {
		return err
	}

	if len(oldestUnsignedValsets) == 0 {
		s.log.Debugln("no valset updates to confirm")
		return nil
	}

	for _, vs := range oldestUnsignedValsets {
		if err := s.signValset(ctx, injective, vs); err != nil {
			return err
		}
	}

	return nil
}

func (s *ethSigner) getUnsignedValsets(ctx context.Context, injective InjectiveNetwork) ([]*types.Valset, error) {
	var oldestUnsignedValsets []*types.Valset
	retryFn := func() (err error) {
		oldestUnsignedValsets, err = injective.OldestUnsignedValsets(ctx)
		if err == cosmos.ErrNotFound || oldestUnsignedValsets == nil {
			return nil
		}

		return err
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.retries),
		retry.OnRetry(func(n uint, err error) {
			s.log.WithError(err).Warningf("failed to get unconfirmed valset updates, will retry (%d)", n)
		}),
	); err != nil {
		s.log.WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedValsets, nil
}

func (s *ethSigner) signValset(
	ctx context.Context,
	injective InjectiveNetwork,
	vs *types.Valset,
) error {
	if err := retry.Do(
		func() error { return injective.SendValsetConfirm(ctx, s.peggyID, vs, s.ethFrom) },
		retry.Context(ctx),
		retry.Attempts(s.retries),
		retry.OnRetry(func(n uint, err error) {
			s.log.WithError(err).Warningf("failed to confirm valset update on Injective, will retry (%d)", n)
		}),
	); err != nil {
		s.log.WithError(err).Errorln("got error, loop exits")
		return err
	}

	s.log.WithFields(log.Fields{
		"valset_nonce":   vs.Nonce,
		"valset_members": len(vs.Members),
	}).Infoln("confirmed valset update on Injective")

	return nil
}
