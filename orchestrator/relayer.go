package orchestrator

import (
	"context"
	"sort"
	"time"

	"github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/util"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

const (
	defaultRelayerLoopDur    = 5 * time.Minute
	findValsetBlocksToSearch = 2000
)

func (s *PeggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	loop := relayerLoop{
		PeggyOrchestrator: s,
		loopDuration:      defaultRelayerLoopDur,
	}

	return loop.Run(ctx)
}

type relayerLoop struct {
	*PeggyOrchestrator
	loopDuration time.Duration
}

func (l *relayerLoop) Logger() log.Logger {
	return l.logger.WithField("loop", "Relayer")
}

func (l *relayerLoop) Run(ctx context.Context) error {
	return loops.RunLoop(ctx, l.loopDuration, func() error {
		return l.relayValsetsAndBatches(ctx)
	})
}

func (l *relayerLoop) relayValsetsAndBatches(ctx context.Context) error {
	var pg loops.ParanoidGroup

	if l.valsetRelayEnabled {
		pg.Go(func() error {
			return retry.Do(func() error { return l.relayValset(ctx) },
				retry.Context(ctx),
				retry.Attempts(l.maxAttempts),
				retry.OnRetry(func(n uint, err error) {
					l.Logger().WithError(err).Warningf("failed to relay valsets, will retry (%d)", n)
				}),
			)
		})
	}

	if l.batchRelayEnabled {
		pg.Go(func() error {
			return retry.Do(func() error { return l.relayBatch(ctx) },
				retry.Context(ctx),
				retry.Attempts(l.maxAttempts),
				retry.OnRetry(func(n uint, err error) {
					l.Logger().WithError(err).Warningf("failed to relay batches, will retry (%d)", n)
				}),
			)
		})
	}

	if pg.Initialized() {
		if err := pg.Wait(); err != nil {
			l.Logger().WithError(err).Errorln("got error, loop exits")
			return err
		}
	}

	return nil
}

func (l *relayerLoop) relayValset(ctx context.Context) error {
	metrics.ReportFuncCall(l.svcTags)
	doneFn := metrics.ReportFuncTiming(l.svcTags)
	defer doneFn()

	// we should determine if we need to relay one
	// to Ethereum for that we will find the latest confirmed valset and compare it to the ethereum chain
	latestValsets, err := l.inj.LatestValsets(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get latest valset updates from Injective")
	}

	var (
		oldestConfirmedValset     *types.Valset
		oldestConfirmedValsetSigs []*types.MsgValsetConfirm
	)

	for _, set := range latestValsets {
		sigs, err := l.inj.AllValsetConfirms(ctx, set.Nonce)
		if err != nil {
			return errors.Wrapf(err, "failed to get valset confirmations for nonce %d", set.Nonce)
		} else if len(sigs) == 0 {
			continue
		}

		oldestConfirmedValsetSigs = sigs
		oldestConfirmedValset = set
		break
	}

	if oldestConfirmedValset == nil {
		l.Logger().Debugln("no valset update to relay")
		return nil
	}

	currentEthValset, err := l.findLatestValsetOnEth(ctx, l.inj, l.eth)
	if err != nil {
		return errors.Wrap(err, "failed to find latest confirmed valset update on Ethereum")
	}

	if oldestConfirmedValset.Nonce <= currentEthValset.Nonce {
		l.Logger().WithFields(log.Fields{"eth_nonce": currentEthValset.Nonce, "inj_nonce": currentEthValset.Nonce}).Debugln("valset already updated on Ethereum")
		return nil
	}

	latestEthereumValsetNonce, err := l.eth.GetValsetNonce(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get latest valset nonce from Ethereum")
	}

	// Check if other validators already updated the valset
	if oldestConfirmedValset.Nonce <= latestEthereumValsetNonce.Uint64() {
		l.Logger().WithFields(log.Fields{"eth_nonce": latestEthereumValsetNonce, "inj_nonce": currentEthValset.Nonce}).Debugln("valset already updated on Ethereum")
		return nil
	}

	l.Logger().WithFields(log.Fields{"inj_nonce": oldestConfirmedValset.Nonce, "eth_nonce": latestEthereumValsetNonce.Uint64()}).Debugln("latest valset updates")

	// Check custom time delay offset
	blockTime, err := l.inj.GetBlockCreationTime(ctx, int64(oldestConfirmedValset.Height))
	if err != nil {
		return errors.Wrap(err, "failed to parse timestamp from block")
	}

	if timeElapsed := time.Since(blockTime); timeElapsed <= l.relayValsetOffsetDur {
		timeRemaining := time.Duration(int64(l.relayValsetOffsetDur) - int64(timeElapsed))
		l.Logger().WithField("time_remaining", timeRemaining.String()).Debugln("valset relay offset not reached yet")
		return nil
	}

	txHash, err := l.eth.SendEthValsetUpdate(
		ctx,
		currentEthValset,
		oldestConfirmedValset,
		oldestConfirmedValsetSigs,
	)
	if err != nil {
		return err
	}

	l.Logger().WithField("tx_hash", txHash.Hex()).Infoln("sent valset tx to Ethereum")

	return nil
}

func (l *relayerLoop) relayBatch(ctx context.Context) error {
	metrics.ReportFuncCall(l.svcTags)
	doneFn := metrics.ReportFuncTiming(l.svcTags)
	defer doneFn()

	latestBatches, err := l.inj.LatestTransactionBatches(ctx)
	if err != nil {
		return err
	}

	var (
		oldestConfirmedBatch     *types.OutgoingTxBatch
		oldestConfirmedBatchSigs []*types.MsgConfirmBatch
	)

	for _, batch := range latestBatches {
		// todo: fix
		if batch.TokenContract == "0xAD1794307245443B3Cb55d88e79EEE4d8a548C03" {
			// Sepolia migration: ignore batches built with GoerliINJ (batch 2300)
			continue
		}

		sigs, err := l.inj.TransactionBatchSignatures(ctx, batch.BatchNonce, common.HexToAddress(batch.TokenContract))
		if err != nil {
			return err
		} else if len(sigs) == 0 {
			continue
		}

		oldestConfirmedBatch = batch
		oldestConfirmedBatchSigs = sigs
	}

	if oldestConfirmedBatch == nil {
		l.Logger().Debugln("no batch to relay")
		return nil
	}

	latestEthereumBatch, err := l.eth.GetTxBatchNonce(ctx, common.HexToAddress(oldestConfirmedBatch.TokenContract))
	if err != nil {
		return err
	}

	currentValset, err := l.findLatestValsetOnEth(ctx, l.inj, l.eth)
	if err != nil {
		return errors.Wrap(err, "failed to find latest valset")
	} else if currentValset == nil {
		return errors.Wrap(err, "latest valset not found")
	}

	if oldestConfirmedBatch.BatchNonce <= latestEthereumBatch.Uint64() {
		l.Logger().WithFields(log.Fields{"eth_nonce": latestEthereumBatch.Uint64(), "inj_nonce": oldestConfirmedBatch.BatchNonce}).Debugln("batch already updated on Ethereum")
		return nil
	}

	latestEthereumBatch, err = l.eth.GetTxBatchNonce(ctx, common.HexToAddress(oldestConfirmedBatch.TokenContract))
	if err != nil {
		return err
	}

	// Check if ethereum batch was updated by other validators
	if oldestConfirmedBatch.BatchNonce <= latestEthereumBatch.Uint64() {
		l.Logger().WithFields(log.Fields{"eth_nonce": latestEthereumBatch.Uint64(), "inj_nonce": oldestConfirmedBatch.BatchNonce}).Debugln("batch already updated on Ethereum")
		return nil
	}

	l.Logger().WithFields(log.Fields{"inj_nonce": oldestConfirmedBatch.BatchNonce, "eth_nonce": latestEthereumBatch.Uint64()}).Debugln("latest batch updates")

	// Check custom time delay offset
	blockTime, err := l.inj.GetBlockCreationTime(ctx, int64(oldestConfirmedBatch.Block))
	if err != nil {
		return errors.Wrap(err, "failed to parse timestamp from block")
	}

	if timeElapsed := time.Since(blockTime); timeElapsed <= l.relayBatchOffsetDur {
		timeRemaining := time.Duration(int64(l.relayBatchOffsetDur) - int64(timeElapsed))
		l.Logger().WithField("time_remaining", timeRemaining.String()).Debugln("batch relay offset not reached yet")
		return nil
	}

	// Send SendTransactionBatch to Ethereum
	txHash, err := l.eth.SendTransactionBatch(ctx, currentValset, oldestConfirmedBatch, oldestConfirmedBatchSigs)
	if err != nil {
		return err
	}

	l.Logger().WithField("tx_hash", txHash.Hex()).Infoln("sent batch tx to Ethereum")

	return nil
}

// FindLatestValset finds the latest valset on the Peggy contract by looking back through the event
// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
// as the latest update will be in recent blockchain history and the search moves from the present
// backwards in time. In the case that the validator set has not been updated for a very long time
// this will take longer.
func (l *relayerLoop) findLatestValsetOnEth(ctx context.Context, injective InjectiveNetwork, ethereum EthereumNetwork) (*types.Valset, error) {
	latestHeader, err := ethereum.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest eth header")
	}

	latestEthereumValsetNonce, err := ethereum.GetValsetNonce(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest valset nonce on Ethereum")
	}

	cosmosValset, err := injective.ValsetAt(ctx, latestEthereumValsetNonce.Uint64())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Injective valset")
	}

	currentBlock := latestHeader.Number.Uint64()

	for currentBlock > 0 {
		var startSearchBlock uint64
		if currentBlock <= findValsetBlocksToSearch {
			startSearchBlock = 0
		} else {
			startSearchBlock = currentBlock - findValsetBlocksToSearch
		}

		valsetUpdatedEvents, err := ethereum.GetValsetUpdatedEvents(startSearchBlock, currentBlock)
		if err != nil {
			return nil, errors.Wrap(err, "failed to filter past ValsetUpdated events from Ethereum")
		}

		// by default the lowest found valset goes first, we want the highest
		//
		// TODO(xlab): this follows the original impl, but sort might be skipped there:
		// we could access just the latest element later.
		sort.Sort(sort.Reverse(PeggyValsetUpdatedEvents(valsetUpdatedEvents)))

		if len(valsetUpdatedEvents) == 0 {
			currentBlock = startSearchBlock
			continue
		}

		// we take only the first event if we find any at all.
		event := valsetUpdatedEvents[0]
		valset := &types.Valset{
			Nonce:        event.NewValsetNonce.Uint64(),
			Members:      make([]*types.BridgeValidator, 0, len(event.Powers)),
			RewardAmount: sdk.NewIntFromBigInt(event.RewardAmount),
			RewardToken:  event.RewardToken.Hex(),
		}

		for idx, p := range event.Powers {
			valset.Members = append(valset.Members, &types.BridgeValidator{
				Power:           p.Uint64(),
				EthereumAddress: event.Validators[idx].Hex(),
			})
		}

		checkIfValsetsDiffer(cosmosValset, valset)

		return valset, nil

	}

	return nil, ErrNotFound
}

var ErrNotFound = errors.New("not found")

type PeggyValsetUpdatedEvents []*peggyevents.PeggyValsetUpdatedEvent

func (a PeggyValsetUpdatedEvents) Len() int { return len(a) }
func (a PeggyValsetUpdatedEvents) Less(i, j int) bool {
	return a[i].NewValsetNonce.Cmp(a[j].NewValsetNonce) < 0
}
func (a PeggyValsetUpdatedEvents) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// This function exists to provide a warning if Cosmos and Ethereum have different validator sets
// for a given nonce. In the mundane version of this warning the validator sets disagree on sorting order
// which can happen if some relayer uses an unstable sort, or in a case of a mild griefing attack.
// The Peggy contract validates signatures in order of highest to lowest power. That way it can exit
// the loop early once a vote has enough power, if a relayer where to submit things in the reverse order
// they could grief users of the contract into paying more in gas.
// The other (and far worse) way a disagreement here could occur is if validators are colluding to steal
// funds from the Peggy contract and have submitted a hijacking update. If slashing for off Cosmos chain
// Ethereum signatures is implemented you would put that handler here.
func checkIfValsetsDiffer(cosmosValset, ethereumValset *types.Valset) {
	if cosmosValset == nil && ethereumValset.Nonce == 0 {
		// bootstrapping case
		return
	} else if cosmosValset == nil {
		log.WithField(
			"eth_valset_nonce",
			ethereumValset.Nonce,
		).Errorln("Cosmos does not have a valset for nonce from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if cosmosValset.Nonce != ethereumValset.Nonce {
		log.WithFields(log.Fields{
			"cosmos_valset_nonce": cosmosValset.Nonce,
			"eth_valset_nonce":    ethereumValset.Nonce,
		}).Errorln("Cosmos does have a wrong valset nonce, differs from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if len(cosmosValset.Members) != len(ethereumValset.Members) {
		log.WithFields(log.Fields{
			"cosmos_valset": len(cosmosValset.Members),
			"eth_valset":    len(ethereumValset.Members),
		}).Errorln("Cosmos and Ethereum Valsets have different length. Possible bridge hijacking!")
		return
	}

	BridgeValidators(cosmosValset.Members).Sort()
	BridgeValidators(ethereumValset.Members).Sort()

	for idx, member := range cosmosValset.Members {
		if ethereumValset.Members[idx].EthereumAddress != member.EthereumAddress {
			log.Errorln("Valsets are different, a sorting error?")
		}
		if ethereumValset.Members[idx].Power != member.Power {
			log.Errorln("Valsets are different, a sorting error?")
		}
	}
}

type BridgeValidators []*types.BridgeValidator

// Sort sorts the validators by power
func (b BridgeValidators) Sort() {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Power == b[j].Power {
			// Secondary sort on eth address in case powers are equal
			return util.EthAddrLessThan(b[i].EthereumAddress, b[j].EthereumAddress)
		}
		return b[i].Power > b[j].Power
	})
}

// HasDuplicates returns true if there are duplicates in the set
func (b BridgeValidators) HasDuplicates() bool {
	m := make(map[string]struct{}, len(b))
	for i := range b {
		m[b[i].EthereumAddress] = struct{}{}
	}
	return len(m) != len(b)
}

// GetPowers returns only the power values for all members
func (b BridgeValidators) GetPowers() []uint64 {
	r := make([]uint64, len(b))
	for i := range b {
		r[i] = b[i].Power
	}
	return r
}
