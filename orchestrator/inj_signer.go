package orchestrator

import (
	"context"
	gethcommon "github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Injective node they can simply be assumed to be
// valid and signed off on.
func (s *PeggyOrchestrator) EthSignerMainLoop(ctx context.Context, inj cosmos.Network, peggyID gethcommon.Hash) error {
	signer := ethSigner{
		PeggyOrchestrator: s,
		Injective:         inj,
		PeggyID:           peggyID,
	}

	s.logger.WithField("loop_duration", defaultLoopDur.String()).Debugln("starting EthSigner...")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		return signer.SignValsetsAndBatches(ctx)
	})
}

type ethSigner struct {
	*PeggyOrchestrator
	Injective cosmos.Network
	PeggyID   gethcommon.Hash
}

func (l *ethSigner) Logger() log.Logger {
	return l.logger.WithField("loop", "EthSigner")
}

func (l *ethSigner) SignValsetsAndBatches(ctx context.Context) error {
	if err := l.signNewValsetUpdates(ctx); err != nil {
		return err
	}

	if err := l.signNewBatch(ctx); err != nil {
		return err
	}

	return nil
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

	if err := l.signBatch(ctx, oldestUnsignedTransactionBatch); err != nil {
		return err
	}

	return nil
}

func (l *ethSigner) getUnsignedBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error) {
	var oldestUnsignedBatch *peggytypes.OutgoingTxBatch
	if err := retryFnOnErr(ctx, l.Logger(), func() error {
		oldestUnsignedBatch, _ = l.Injective.OldestUnsignedTransactionBatch(ctx, l.injAddr)
		return nil
	}); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedBatch, nil
}

func (l *ethSigner) signBatch(ctx context.Context, batch *peggytypes.OutgoingTxBatch) error {
	if batch == nil {
		l.Logger().Infoln("no batch to confirm")
		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), func() error {
		return l.Injective.SendBatchConfirm(ctx, l.ethAddr, l.PeggyID, batch)
	}); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{"token_contract": batch.TokenContract, "batch_nonce": batch.BatchNonce, "txs": len(batch.Transactions)}).Infoln("confirmed batch on Injective")

	return nil
}

func (l *ethSigner) getUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	var oldestUnsignedValsets []*peggytypes.Valset
	fn := func() error {
		oldestUnsignedValsets, _ = l.Injective.OldestUnsignedValsets(ctx, l.injAddr)
		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), fn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return nil, err
	}

	return oldestUnsignedValsets, nil
}

func (l *ethSigner) signValset(ctx context.Context, vs *peggytypes.Valset) error {
	if err := retryFnOnErr(ctx, l.Logger(), func() error {
		return l.Injective.SendValsetConfirm(ctx, l.ethAddr, l.PeggyID, vs)
	}); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.Logger().WithFields(log.Fields{"valset_nonce": vs.Nonce, "validators": len(vs.Members)}).Infoln("confirmed valset update on Injective")

	return nil
}
