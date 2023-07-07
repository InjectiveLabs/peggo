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
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func (s *PeggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "Relayer")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		var pg loops.ParanoidGroup
		if s.valsetRelayEnabled {
			logger.Infoln("scanning injective for confirmed valset updates")
			pg.Go(func() error {
				return retry.Do(func() error { return s.relayValsets(ctx, logger.WithField("Relayer", "Valset")) },
					retry.Context(ctx),
					retry.Attempts(s.maxAttempts),
					retry.OnRetry(func(n uint, err error) {
						logger.WithError(err).Warningf("failed to relay valsets, will retry (%d)", n)
					}),
				)
			})
		}

		if s.batchRelayEnabled {
			logger.Infoln("scanning injective for confirmed batches")
			pg.Go(func() error {
				return retry.Do(func() error { return s.relayBatches(ctx, logger.WithField("Relayer", "Batch")) },
					retry.Context(ctx),
					retry.Attempts(s.maxAttempts),
					retry.OnRetry(func(n uint, err error) {
						logger.WithError(err).Warningf("failed to relay batches, will retry (%d)", n)
					}),
				)
			})
		}

		if pg.Initialized() {
			if err := pg.Wait(); err != nil {
				logger.WithError(err).Errorln("got error, loop exits")
				return err
			}
		}

		return nil
	})
}

func (s *PeggyOrchestrator) relayValsets(ctx context.Context, logger log.Logger) error {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	// we should determine if we need to relay one
	// to Ethereum for that we will find the latest confirmed valset and compare it to the ethereum chain

	latestValsets, err := s.injective.LatestValsets(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return errors.Wrap(err, "failed to fetch latest valsets from Injective")
	}

	var latestCosmosSigs []*types.MsgValsetConfirm
	var latestCosmosConfirmed *types.Valset
	for _, set := range latestValsets {
		sigs, err := s.injective.AllValsetConfirms(ctx, set.Nonce)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			return errors.Wrapf(err, "failed to get valset confirms at nonce %d", set.Nonce)
		} else if len(sigs) == 0 {
			continue
		}

		latestCosmosSigs = sigs
		latestCosmosConfirmed = set
		break
	}

	if latestCosmosConfirmed == nil {
		logger.Debugln("no confirmed valsets found on Injective, nothing to relay...")
		return nil
	}

	currentEthValset, err := s.findLatestValsetOnEthereum(ctx, logger)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return errors.Wrap(err, "failed to find latest confirmed valset on Ethereum")
	}

	logger.WithFields(log.Fields{
		"inj_valset": latestCosmosConfirmed,
		"eth_valset": currentEthValset,
	}).Debugln("latest valsets")

	if latestCosmosConfirmed.Nonce <= currentEthValset.Nonce {
		return nil
	}

	latestEthereumValsetNonce, err := s.ethereum.GetValsetNonce(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return errors.Wrap(err, "failed to get latest valset nonce from Ethereum")
	}

	// Check if other validators already updated the valset
	if latestCosmosConfirmed.Nonce <= latestEthereumValsetNonce.Uint64() {
		return nil
	}

	// Check custom time delay offset
	blockResult, err := s.injective.GetBlock(ctx, int64(latestCosmosConfirmed.Height))
	if err != nil {
		return errors.Wrapf(err, "failed to get block %d from Injective", latestCosmosConfirmed.Height)
	}

	valsetCreatedAt := blockResult.Block.Time
	customTimeDelay := valsetCreatedAt.Add(s.relayValsetOffsetDur)

	if time.Now().Sub(customTimeDelay) <= 0 {
		return nil
	}

	logger.WithFields(log.Fields{
		"inj_valset": latestCosmosConfirmed.Nonce,
		"eth_valset": latestEthereumValsetNonce.Uint64(),
	}).Infoln("detected new valset on Injective")

	txHash, err := s.ethereum.SendEthValsetUpdate(
		ctx,
		currentEthValset,
		latestCosmosConfirmed,
		latestCosmosSigs,
	)

	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return err
	}

	logger.WithField("tx_hash", txHash.Hex()).Infoln("updated valset on Ethereum")

	return nil
}

func (s *PeggyOrchestrator) relayBatches(ctx context.Context, logger log.Logger) error {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	latestBatches, err := s.injective.LatestTransactionBatches(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return err
	}

	var (
		oldestSignedBatch *types.OutgoingTxBatch
		oldestSigs        []*types.MsgConfirmBatch
	)

	for _, batch := range latestBatches {
		sigs, err := s.injective.TransactionBatchSignatures(ctx, batch.BatchNonce, common.HexToAddress(batch.TokenContract))
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			return err
		} else if len(sigs) == 0 {
			continue
		}

		oldestSignedBatch = batch
		oldestSigs = sigs
	}

	if oldestSignedBatch == nil {
		logger.Debugln("no confirmed transaction batches on Injective, nothing to relay...")
		return nil
	}

	latestEthereumBatch, err := s.ethereum.GetTxBatchNonce(
		ctx,
		common.HexToAddress(oldestSignedBatch.TokenContract),
	)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return err
	}

	logger.WithFields(log.Fields{
		"inj_batch": oldestSignedBatch.BatchNonce,
		"eth_batch": latestEthereumBatch.Uint64(),
	}).Debugln("latest batches")

	if oldestSignedBatch.BatchNonce <= latestEthereumBatch.Uint64() {
		return nil
	}

	currentValset, err := s.findLatestValsetOnEthereum(ctx, logger)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return errors.Wrap(err, "failed to find latest valset")
	} else if currentValset == nil {
		metrics.ReportFuncError(s.svcTags)
		return errors.Wrap(err, "latest valset not found")
	}

	latestEthereumBatch, err = s.ethereum.GetTxBatchNonce(ctx, common.HexToAddress(oldestSignedBatch.TokenContract))
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return err
	}

	// Check if ethereum batch was updated by other validators
	if oldestSignedBatch.BatchNonce <= latestEthereumBatch.Uint64() {
		return nil
	}

	// Check custom time delay offset
	blockResult, err := s.injective.GetBlock(ctx, int64(oldestSignedBatch.Block))
	if err != nil {
		return errors.Wrapf(err, "failed to get block %d from Injective", oldestSignedBatch.Block)
	}

	batchCreatedAt := blockResult.Block.Time
	customTimeDelay := batchCreatedAt.Add(s.relayBatchOffsetDur)

	if time.Now().Sub(customTimeDelay) <= 0 {
		return nil
	}

	logger.WithFields(log.Fields{
		"inj_batch": oldestSignedBatch.BatchNonce,
		"eth_batch": latestEthereumBatch.Uint64(),
	}).Infoln("detected new transaction batch on Injective")

	// Send SendTransactionBatch to Ethereum
	txHash, err := s.ethereum.SendTransactionBatch(ctx, currentValset, oldestSignedBatch, oldestSigs)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return err
	}

	logger.WithField("tx_hash", txHash.Hex()).Infoln("updated transaction batch on Ethereum")

	return nil
}

const valsetBlocksToSearch = 2000

// FindLatestValset finds the latest valset on the Peggy contract by looking back through the event
// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
// as the latest update will be in recent blockchain history and the search moves from the present
// backwards in time. In the case that the validator set has not been updated for a very long time
// this will take longer.
func (s *PeggyOrchestrator) findLatestValsetOnEthereum(ctx context.Context, logger log.Logger) (*types.Valset, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	latestHeader, err := s.ethereum.HeaderByNumber(ctx, nil)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return nil, errors.Wrap(err, "failed to get latest eth header")
	}

	latestEthereumValsetNonce, err := s.ethereum.GetValsetNonce(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return nil, errors.Wrap(err, "failed to get latest valset nonce on Ethereum")
	}

	cosmosValset, err := s.injective.ValsetAt(ctx, latestEthereumValsetNonce.Uint64())
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return nil, errors.Wrap(err, "failed to get Injective valset")
	}

	currentBlock := latestHeader.Number.Uint64()

	for currentBlock > 0 {
		var startSearchBlock uint64
		if currentBlock <= valsetBlocksToSearch {
			startSearchBlock = 0
		} else {
			startSearchBlock = currentBlock - valsetBlocksToSearch
		}

		logger.WithFields(log.Fields{
			"block_start": startSearchBlock,
			"block_end":   currentBlock,
		}).Debugln("looking back into Ethereum history to find the last valset update")

		valsetUpdatedEvents, err := s.ethereum.GetValsetUpdatedEvents(startSearchBlock, currentBlock)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
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

		logger.Debugln("found events", valsetUpdatedEvents)

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

		s.checkIfValsetsDiffer(cosmosValset, valset)
		return valset, nil

	}

	return nil, ErrNotFound
}

var ErrNotFound = errors.New("not found")

type PeggyValsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent

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
func (s *PeggyOrchestrator) checkIfValsetsDiffer(cosmosValset, ethereumValset *types.Valset) {
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
