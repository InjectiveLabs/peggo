package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	gethcommon "github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Injective node they can simply be assumed to be
// valid and signed off on.
func (s *PeggyOrchestrator) EthSignerMainLoop(ctx context.Context, inj cosmos.Network, peggyID gethcommon.Hash) error {
	signer := ethSigner{
		PeggyOrchestrator: s,
		Injective:         inj,
		PeggyID:           peggyID,
		LoopDuration:      defaultLoopDur,
	}

	s.logger.WithField("loop_duration", signer.LoopDuration.String()).Debugln("starting EthSigner...")

	return loops.RunLoop(ctx, signer.LoopDuration, signer.SignValsetsAndBatchesLoop(ctx))
}

type ethSigner struct {
	*PeggyOrchestrator
	Injective    cosmos.Network
	LoopDuration time.Duration
	PeggyID      gethcommon.Hash
}

func (l *ethSigner) Logger() log.Logger {
	return l.logger.WithField("loop", "EthSigner")
}

func (l *ethSigner) SignValsetsAndBatchesLoop(ctx context.Context) func() error {
	return func() error {
		if err := l.signNewValsetUpdates(ctx); err != nil {
			return err
		}

		if err := l.signNewBatch(ctx); err != nil {
			return err
		}

		return nil
	}
}

func (l *ethSigner) signNewValsetUpdates(ctx context.Context) error {
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
	}

	return nil
}

func (l *ethSigner) signNewBatch(ctx context.Context) error {
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

func (l *ethSigner) getUnsignedBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error) {
	var oldestUnsignedBatch *peggytypes.OutgoingTxBatch
	getOldestUnsignedBatchFn := func() (err error) {
		// sign the last unsigned batch, TODO check if we already have signed this
		oldestUnsignedBatch, err = l.Injective.OldestUnsignedTransactionBatch(ctx, l.injAddr)
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

func (l *ethSigner) signBatch(ctx context.Context, batch *peggytypes.OutgoingTxBatch) error {
	signFn := func() error {
		return l.Injective.SendBatchConfirm(ctx, l.ethAddr, l.PeggyID, batch)
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

func (l *ethSigner) getUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	var oldestUnsignedValsets []*peggytypes.Valset
	getOldestUnsignedValsetsFn := func() (err error) {
		oldestUnsignedValsets, err = l.Injective.OldestUnsignedValsets(ctx, l.injAddr)
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

func (l *ethSigner) signValset(ctx context.Context, vs *peggytypes.Valset) error {
	signFn := func() error {
		return l.Injective.SendValsetConfirm(ctx, l.ethAddr, l.PeggyID, vs)
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
