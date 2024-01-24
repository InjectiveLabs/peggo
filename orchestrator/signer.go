package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Injective node they can simply be assumed to be
// valid and signed off on.
func (s *PeggyOrchestrator) EthSignerMainLoop(ctx context.Context, peggyID common.Hash) error {
	loop := ethSignerLoop{
		PeggyOrchestrator: s,
		loopDuration:      defaultLoopDur,
		peggyID:           peggyID,
		ethFrom:           s.eth.FromAddress(),
	}

	return loop.Run(ctx)
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

func (l *ethSignerLoop) Run(ctx context.Context) error {
	l.logger.WithField("loop_duration", l.loopDuration.String()).Debugln("starting EthSigner loop...")

	return loops.RunLoop(ctx, l.loopDuration, func() error {
		if err := l.signNewValsetUpdates(ctx); err != nil {
			return err
		}

		if err := l.signNewBatch(ctx); err != nil {
			return err
		}

		return nil
	})
}

func (l *ethSignerLoop) signNewValsetUpdates(ctx context.Context) error {
	oldestUnsignedValsets, err := l.getUnsignedValsets(ctx)
	if err != nil {
		return err
	}

	if len(oldestUnsignedValsets) == 0 {
		l.Logger().Infoln("no valset updates to confirm")
		return nil
	}

	for _, vs := range oldestUnsignedValsets {
		if err := l.signValset(ctx, vs); err != nil {
			return err
		}

		// todo: in case of multiple updates, we should sleep in between tx (non-continuous nonce)
	}

	return nil
}

func (l *ethSignerLoop) signNewBatch(ctx context.Context) error {
	oldestUnsignedTransactionBatch, err := l.getUnsignedBatch(ctx)
	if err != nil {
		return err
	}

	if oldestUnsignedTransactionBatch == nil {
		l.Logger().Infoln("no batch to confirm")
		return nil
	}

	if err := l.signBatch(ctx, oldestUnsignedTransactionBatch); err != nil {
		return err
	}

	return nil
}

func (l *ethSignerLoop) getUnsignedBatch(ctx context.Context) (*types.OutgoingTxBatch, error) {
	var oldestUnsignedBatch *types.OutgoingTxBatch
	getOldestUnsignedBatchFn := func() (err error) {
		// sign the last unsigned batch, TODO check if we already have signed this
		oldestUnsignedBatch, err = l.inj.OldestUnsignedTransactionBatch(ctx, l.orchestratorAddr)
		if oldestUnsignedBatch == nil {
			return nil
		}

		return err
	}

	if err := retry.Do(getOldestUnsignedBatchFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to get unconfirmed batch, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedBatch, nil
}

func (l *ethSignerLoop) signBatch(ctx context.Context, batch *types.OutgoingTxBatch) error {
	signFn := func() error {
		return l.inj.SendBatchConfirm(ctx, l.ethFrom, l.peggyID, batch)
	}

	if err := retry.Do(signFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to confirm batch on Injective, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{"token_contract": batch.TokenContract, "batch_nonce": batch.BatchNonce, "txs": len(batch.Transactions)}).Infoln("confirmed batch on Injective")

	return nil
}

func (l *ethSignerLoop) getUnsignedValsets(ctx context.Context) ([]*types.Valset, error) {
	var oldestUnsignedValsets []*types.Valset
	getOldestUnsignedValsetsFn := func() (err error) {
		oldestUnsignedValsets, err = l.inj.OldestUnsignedValsets(ctx, l.orchestratorAddr)
		if oldestUnsignedValsets == nil {
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

func (l *ethSignerLoop) signValset(ctx context.Context, vs *types.Valset) error {
	signFn := func() error {
		return l.inj.SendValsetConfirm(ctx, l.ethFrom, l.peggyID, vs)
	}

	if err := retry.Do(signFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to confirm valset update on Injective, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{"valset_nonce": vs.Nonce, "validators": len(vs.Members)}).Infoln("confirmed valset update on Injective")

	return nil
}
