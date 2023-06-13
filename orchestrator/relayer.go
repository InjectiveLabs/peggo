package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/util"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"sort"
	"time"
)

func (s *PeggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "RelayerMainLoop")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		var pg loops.ParanoidGroup
		if s.valsetRelayEnabled {
			logger.Info("Valset Relay Enabled. Starting to relay valsets to Ethereum")
			pg.Go(func() error {
				return retry.Do(func() error {
					return s.relayValsets(ctx)
				}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
					logger.WithError(err).Warningf("failed to relay Valsets, will retry (%d)", n)
				}))
			})
		}

		if s.batchRelayEnabled {
			logger.Info("Batch Relay Enabled. Starting to relay batches to Ethereum")
			pg.Go(func() error {
				return retry.Do(func() error {
					return s.relayBatches(ctx)
				}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
					logger.WithError(err).Warningf("failed to relay TxBatches, will retry (%d)", n)
				}))
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

func (s *PeggyOrchestrator) relayValsets(ctx context.Context) error {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	// we should determine if we need to relay one
	// to Ethereum for that we will find the latest confirmed valset and compare it to the ethereum chain

	//latestValsets, err := s.cosmosQueryClient.LatestValsets(ctx)
	latestValsets, err := s.injective.LatestValsets(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to fetch latest valsets from cosmos")
		return err
	}

	var latestCosmosSigs []*types.MsgValsetConfirm
	var latestCosmosConfirmed *types.Valset
	for _, set := range latestValsets {
		//sigs, err := s.cosmosQueryClient.AllValsetConfirms(ctx, set.Nonce)

		sigs, err := s.injective.AllValsetConfirms(ctx, set.Nonce)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			err = errors.Wrapf(err, "failed to get valset confirms at nonce %d", set.Nonce)
			return err
		} else if len(sigs) == 0 {
			continue
		}

		latestCosmosSigs = sigs
		latestCosmosConfirmed = set
		break
	}

	if latestCosmosConfirmed == nil {
		log.Debugln("no confirmed valsets found, nothing to relay")
		return nil
	}

	currentEthValset, err := s.findLatestValset(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "couldn't find latest confirmed valset on Ethereum")
		return err
	}
	log.WithFields(log.Fields{"currentEthValset": currentEthValset, "latestCosmosConfirmed": latestCosmosConfirmed}).Debugln("Found Latest valsets")

	if latestCosmosConfirmed.Nonce > currentEthValset.Nonce {

		// todo
		latestEthereumValsetNonce, err := s.ethereum.GetValsetNonce(ctx)

		//latestEthereumValsetNonce, err := s.peggyContract.GetValsetNonce(ctx, s.peggyContract.FromAddress())
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			err = errors.Wrap(err, "failed to get latest Valset nonce")
			return err
		}

		// Check if latestCosmosConfirmed already submitted by other validators in mean time
		if latestCosmosConfirmed.Nonce > latestEthereumValsetNonce.Uint64() {

			// Check custom time delay offset
			blockResult, err := s.injective.GetBlock(ctx, int64(latestCosmosConfirmed.Height))
			if err != nil {
				return err
			}
			valsetCreatedAt := blockResult.Block.Time
			// todo: do this at init
			//relayValsetOffsetDur, err := time.ParseDuration(s.relayValsetOffsetDur)
			//if err != nil {
			//	return err
			//}
			customTimeDelay := valsetCreatedAt.Add(s.relayValsetOffsetDur)
			if time.Now().Sub(customTimeDelay) <= 0 {
				return nil
			}

			log.Infof("Detected latest cosmos valset nonce %d, but latest valset on Ethereum is %d. Sending update to Ethereum\n",
				latestCosmosConfirmed.Nonce, latestEthereumValsetNonce.Uint64())

			// todo
			txHash, err := s.ethereum.SendEthValsetUpdate(
				ctx,
				currentEthValset,
				latestCosmosConfirmed,
				latestCosmosSigs,
			)

			// Send Valset Update to Ethereum
			//txHash, err := s.peggyContract.SendEthValsetUpdate(
			//	ctx,
			//	currentEthValset,
			//	latestCosmosConfirmed,
			//	latestCosmosSigs,
			//)
			if err != nil {
				metrics.ReportFuncError(s.svcTags)
				return err
			}

			log.WithField("tx_hash", txHash.Hex()).Infoln("Sent Ethereum Tx (EthValsetUpdate)")
		}

	}

	return nil
}

func (s *PeggyOrchestrator) relayBatches(ctx context.Context) error {
	return nil
}

const valsetBlocksToSearch = 2000

// FindLatestValset finds the latest valset on the Peggy contract by looking back through the event
// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
// as the latest update will be in recent blockchain history and the search moves from the present
// backwards in time. In the case that the validator set has not been updated for a very long time
// this will take longer.
func (s *PeggyOrchestrator) findLatestValset(ctx context.Context) (*types.Valset, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	latestHeader, err := s.ethereum.HeaderByNumber(ctx, nil)
	//latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to get latest header")
		return nil, err
	}
	currentBlock := latestHeader.Number.Uint64()

	//peggyFilterer, err := wrappers.NewPeggyFilterer(s.peggyContract.Address(), s.ethProvider)
	//if err != nil {
	//	metrics.ReportFuncError(s.svcTags)
	//	err = errors.Wrap(err, "failed to init Peggy events filterer")
	//	return nil, err
	//}

	latestEthereumValsetNonce, err := s.ethereum.GetValsetNonce(ctx)
	//latestEthereumValsetNonce, err := s.peggyContract.GetValsetNonce(ctx, s.peggyContract.FromAddress())
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to get latest Valset nonce")
		return nil, err
	}

	cosmosValset, err := s.injective.ValsetAt(ctx, latestEthereumValsetNonce.Uint64())
	//cosmosValset, err := s.cosmosQueryClient.ValsetAt(ctx, latestEthereumValsetNonce.Uint64())
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to get cosmos Valset")
		return nil, err
	}

	for currentBlock > 0 {
		log.WithField("current_block", currentBlock).
			Debugln("About to submit a Valset or Batch looking back into the history to find the last Valset Update")

		var endSearchBlock uint64
		if currentBlock <= valsetBlocksToSearch {
			endSearchBlock = 0
		} else {
			endSearchBlock = currentBlock - valsetBlocksToSearch
		}

		valsetUpdatedEvents, err := s.ethereum.GetValsetUpdatedEvents(endSearchBlock, currentBlock)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			err = errors.Wrap(err, "failed to filter past ValsetUpdated events from Ethereum")
			return nil, err
		}
		//var valsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent
		//iter, err := peggyFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
		//	Start: endSearchBlock,
		//	End:   &currentBlock,
		//}, nil)
		//} else {
		//	for iter.Next() {
		//		valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
		//	}
		//
		//	iter.Close()
		//}

		// by default the lowest found valset goes first, we want the highest
		//
		// TODO(xlab): this follows the original impl, but sort might be skipped there:
		// we could access just the latest element later.
		sort.Sort(sort.Reverse(PeggyValsetUpdatedEvents(valsetUpdatedEvents)))

		log.Debugln("found events", valsetUpdatedEvents)

		if len(valsetUpdatedEvents) == 0 {
			currentBlock = endSearchBlock
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
