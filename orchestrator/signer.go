package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
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

	loop := ethSignerLoop{
		PeggyOrchestrator: s,
		loopDuration:      defaultLoopDur,
		peggyID:           peggyID,
		ethFrom:           s.ethereum.FromAddress(),
	}

	return loop.Run(ctx, s.injective)
}

func (s *PeggyOrchestrator) getPeggyID(ctx context.Context) (common.Hash, error) {
	var peggyID common.Hash
	getPeggyIDFn := func() (err error) {
		peggyID, err = s.ethereum.GetPeggyID(ctx)
		return err
	}

	if err := retry.Do(getPeggyIDFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Warningf("failed to get Peggy ID from Ethereum contract, will retry (%d)", n)
		}),
	); err != nil {
		log.WithError(err).Errorln("got error, loop exits")
		return [32]byte{}, err
	}

	log.WithField("id", peggyID.Hex()).Debugln("got peggy ID from Ethereum contract")

	return peggyID, nil
}

type ethSignerLoop struct {
	*PeggyOrchestrator
	loopDuration time.Duration
	peggyID      common.Hash
	ethFrom      common.Address
}

func (l *ethSignerLoop) Logger() log.Logger {
	return l.logger.WithField("loop", "EthSigner")
}

func (l *ethSignerLoop) Run(ctx context.Context, injective InjectiveNetwork) error {
	return loops.RunLoop(ctx, l.loopDuration, l.loopFn(ctx, injective))
}

func (l *ethSignerLoop) loopFn(ctx context.Context, injective InjectiveNetwork) func() error {
	return func() error {
		if err := l.signNewValsetUpdates(ctx, injective); err != nil {
			return err
		}

		if err := l.signNewBatch(ctx, injective); err != nil {
			return err
		}

		return nil
	}
}

func (l *ethSignerLoop) signNewValsetUpdates(ctx context.Context, injective InjectiveNetwork) error {
	oldestUnsignedValsets, err := l.getUnsignedValsets(ctx, injective)
	if err != nil {
		return err
	}

	if len(oldestUnsignedValsets) == 0 {
		l.Logger().Debugln("no valset updates to confirm")
		return nil
	}

	for _, vs := range oldestUnsignedValsets {
		if err := l.signValset(ctx, injective, vs); err != nil {
			return err
		}

		// todo: in case of multiple updates, we should sleep in between confirms requests (non-continuous nonce)
	}

	return nil
}

func (l *ethSignerLoop) signNewBatch(ctx context.Context, injective InjectiveNetwork) error {
	oldestUnsignedTransactionBatch, err := l.getUnsignedBatch(ctx, injective)
	if err != nil {
		return err
	}

	if oldestUnsignedTransactionBatch == nil {
		l.Logger().Debugln("no outgoing batch to confirm")
		return nil
	}

	if err := l.signBatch(ctx, injective, oldestUnsignedTransactionBatch); err != nil {
		return err
	}

	return nil
}

func (l *ethSignerLoop) getUnsignedBatch(ctx context.Context, injective InjectiveNetwork) (*types.OutgoingTxBatch, error) {
	var oldestUnsignedTransactionBatch *types.OutgoingTxBatch
	retryFn := func() (err error) {
		// sign the last unsigned batch, TODO check if we already have signed this
		oldestUnsignedTransactionBatch, err = injective.OldestUnsignedTransactionBatch(ctx)
		if errors.Is(err, cosmos.ErrNotFound) || oldestUnsignedTransactionBatch == nil {
			return nil
		}

		return err
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to get unconfirmed batch, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedTransactionBatch, nil
}

func (l *ethSignerLoop) signBatch(ctx context.Context, injective InjectiveNetwork, batch *types.OutgoingTxBatch) error {
	if err := retry.Do(
		func() error { return injective.SendBatchConfirm(ctx, l.peggyID, batch, l.ethFrom) },
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to confirm batch on Injective, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{
		"token_contract": batch.TokenContract,
		"nonce":          batch.BatchNonce,
		"txs":            len(batch.Transactions),
	}).Infoln("confirmed batch on Injective")

	return nil
}

func (l *ethSignerLoop) getUnsignedValsets(ctx context.Context, injective InjectiveNetwork) ([]*types.Valset, error) {
	var oldestUnsignedValsets []*types.Valset
	getOldestUnsignedValsetsFn := func() (err error) {
		oldestUnsignedValsets, err = injective.OldestUnsignedValsets(ctx)
		if errors.Is(err, cosmos.ErrNotFound) || oldestUnsignedValsets == nil {
			return nil
		}

		return err
	}

	if err := retry.Do(getOldestUnsignedValsetsFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to get unconfirmed valset updates, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedValsets, nil
}

func (l *ethSignerLoop) signValset(ctx context.Context, injective InjectiveNetwork, vs *types.Valset) error {
	if err := retry.Do(func() error {
		return injective.SendValsetConfirm(ctx, l.peggyID, vs, l.ethFrom)
	},
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to confirm valset update on Injective, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{
		"nonce":   vs.Nonce,
		"members": len(vs.Members),
	}).Infoln("confirmed valset update on Injective")

	return nil
}
