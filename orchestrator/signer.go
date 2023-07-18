package orchestrator

import (
	"context"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
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
		func() error { return signer.run(ctx, s.injective) },
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

func (s *ethSigner) run(ctx context.Context, injective InjectiveNetwork) error {
	if err := s.signNewValsetUpdates(ctx, injective); err != nil {
		return err
	}

	if err := s.signNewBatches(ctx, injective); err != nil {
		return err
	}

	return nil
}

func (s *ethSigner) signNewBatches(ctx context.Context, injective InjectiveNetwork) error {
	s.log.Infoln("scanning Injective for unconfirmed batches")

	// todo: extend peggy to send mutiple batches instead of the oldest
	oldestUnsignedTransactionBatch, err := s.getUnsignedBatch(ctx, injective)
	if err != nil {
		return err
	}

	unsignedBatches := []*types.OutgoingTxBatch{oldestUnsignedTransactionBatch}

	for _, b := range unsignedBatches {
		// if batch does not satisfy fee, skip it

		if err := s.signBatch(ctx, injective, b); err != nil {
			return err
		}
	}

	return nil
}

func (s *ethSigner) getUnsignedBatch(ctx context.Context, injective InjectiveNetwork) (*types.OutgoingTxBatch, error) {
	var oldestUnsignedTransactionBatch *types.OutgoingTxBatch
	retryFn := func() (err error) {
		// sign the last unsigned batch, TODO check if we already have signed this
		oldestUnsignedTransactionBatch, err = injective.OldestUnsignedTransactionBatch(ctx)
		if err == cosmos.ErrNotFound || oldestUnsignedTransactionBatch == nil {
			return nil
		}

		return err
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.retries),
		retry.OnRetry(func(n uint, err error) {
			s.log.WithError(err).Warningf("failed to get unconfirmed batch, will retry (%d)", n)
		}),
	); err != nil {
		s.log.WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedTransactionBatch, nil
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

	s.log.WithField("batch_nonce", batch.BatchNonce).Infoln("confirmed batch on Injective")

	return nil
}

func (s *ethSigner) signNewValsetUpdates(
	ctx context.Context,
	injective InjectiveNetwork,
) error {
	s.log.Infoln("scanning Injective for unconfirmed valset updates")

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

	s.log.WithField("valset_nonce", vs.Nonce).Infoln("confirmed valset update on Injective")

	return nil
}
