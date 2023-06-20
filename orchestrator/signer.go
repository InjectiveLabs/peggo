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
	logger := log.WithField("loop", "EthSignerMainLoop")

	peggyID, err := s.getPeggyID(ctx, logger)
	if err != nil {
		return err
	}

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		return s.signerLoop(ctx, logger, peggyID)
	})
}

func (s *PeggyOrchestrator) getPeggyID(ctx context.Context, logger log.Logger) (common.Hash, error) {
	var peggyID common.Hash
	retryFn := func() error {
		id, err := s.ethereum.GetPeggyID(ctx)
		if err != nil {
			return err
		}

		peggyID = id
		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to get peggy ID from Ethereum contract, will retry (%d)", n)
		}),
	); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return [32]byte{}, err
	}

	logger.WithField("id", peggyID.Hex()).Debugln("got peggy ID from Ethereum contract")

	return peggyID, nil
}

func (s *PeggyOrchestrator) signerLoop(ctx context.Context, logger log.Logger, peggyID common.Hash) error {
	if err := s.signValsetUpdates(ctx, logger, peggyID); err != nil {
		return err
	}

	if err := s.signTransactionBatches(ctx, logger, peggyID); err != nil {
		return err
	}

	return nil
}

func (s *PeggyOrchestrator) signValsetUpdates(ctx context.Context, logger log.Logger, peggyID common.Hash) error {
	var oldestUnsignedValsets []*types.Valset
	retryFn := func() error {
		oldestValsets, err := s.injective.OldestUnsignedValsets(ctx)
		if err != nil {
			if err == cosmos.ErrNotFound || oldestValsets == nil {
				logger.Debugln("no new valset waiting to be signed")
				return nil
			}

			// dusan: this will never really trigger
			return err
		}
		oldestUnsignedValsets = oldestValsets
		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to get unsigned valset, will retry (%d)", n)
		}),
	); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	for _, vs := range oldestUnsignedValsets {
		logger.Infoln("sending confirm for valset %d", vs.Nonce)
		if err := retry.Do(func() error {
			return s.injective.SendValsetConfirm(ctx, peggyID, vs, s.ethereum.FromAddress())
		},
			retry.Context(ctx),
			retry.Attempts(s.maxAttempts),
			retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to sign and send valset confirmation to Injective, will retry (%d)", n)
			}),
		); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}
	}

	return nil
}

func (s *PeggyOrchestrator) signTransactionBatches(ctx context.Context, logger log.Logger, peggyID common.Hash) error {
	var oldestUnsignedTransactionBatch *types.OutgoingTxBatch
	retryFn := func() error {
		// sign the last unsigned batch, TODO check if we already have signed this
		txBatch, err := s.injective.OldestUnsignedTransactionBatch(ctx)
		if err != nil {
			if err == cosmos.ErrNotFound || txBatch == nil {
				logger.Debugln("no new transaction batch waiting to be signed")
				return nil
			}
			return err
		}
		oldestUnsignedTransactionBatch = txBatch
		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to get unsigned transaction batch, will retry (%d)", n)
		})); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	if oldestUnsignedTransactionBatch == nil {
		return nil
	}

	logger.Infoln("sending confirm for batch %d", oldestUnsignedTransactionBatch.BatchNonce)
	if err := retry.Do(func() error {
		return s.injective.SendBatchConfirm(ctx, peggyID, oldestUnsignedTransactionBatch, s.ethereum.FromAddress())
	}, retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to sign and send batch confirmation to Injective, will retry (%d)", n)
		})); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	return nil
}
