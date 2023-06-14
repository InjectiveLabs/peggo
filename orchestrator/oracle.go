package orchestrator

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/avast/retry-go"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"

	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

// Considering blocktime of up to 3 seconds approx on the Injective Chain and an oracle loop duration = 1 minute,
// we broadcast only 20 events in each iteration.
// So better to search only 20 blocks to ensure all the events are broadcast to Injective Chain without misses.
const (
	ethBlockConfirmationDelay uint64 = 12
	defaultBlocksToSearch     uint64 = 20
)

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
func (s *PeggyOrchestrator) EthOracleMainLoop(ctx context.Context) error {
	logger := log.WithField("loop", "EthOracleMainLoop")
	lastResync := time.Now()
	var lastCheckedBlock uint64

	if err := retry.Do(func() (err error) {
		lastCheckedBlock, err = s.getLastConfirmedEthHeight(ctx)
		if lastCheckedBlock == 0 {
			peggyParams, err := s.injective.PeggyParams(ctx)
			if err != nil {
				log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
			}
			lastCheckedBlock = peggyParams.BridgeContractStartHeight
		}
		return
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.WithError(err).Warningf("failed to get last checked block, will retry (%d)", n)
	})); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	logger.WithField("lastCheckedBlock", lastCheckedBlock).Infoln("Start scanning for events")
	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		// Relays events from Ethereum -> Cosmos
		var currentBlock uint64
		if err := retry.Do(func() (err error) {
			currentBlock, err = s.relayEthEvents(ctx, lastCheckedBlock)
			return
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("error during Eth event checking, will retry (%d)", n)
		})); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		lastCheckedBlock = currentBlock

		/*
			Auto re-sync to catch up the nonce. Reasons why event nonce fall behind.
				1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as block produced, there is a chance the event is missed.
				   we need to re-scan this block to ensure events are not missed due to indexing delay.
				2. if validator was in UnBonding state, the claims broadcasted in last iteration are failed.
				3. if infura call failed while filtering events, the peggo missed to broadcast claim events occured in last iteration.
		**/
		if time.Since(lastResync) >= 48*time.Hour {
			if err := retry.Do(func() (err error) {
				lastCheckedBlock, err = s.getLastConfirmedEthHeight(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get last checked block, will retry (%d)", n)
			})); err != nil {
				logger.WithError(err).Errorln("got error, loop exits")
				return err
			}
			lastResync = time.Now()
			logger.WithFields(log.Fields{"lastResync": lastResync, "lastCheckedBlock": lastCheckedBlock}).Infoln("Auto resync")
		}

		return nil
	})
}

// getLastConfirmedEthHeight retrieves the last claim event this oracle has relayed to Cosmos.
func (s *PeggyOrchestrator) getLastConfirmedEthHeight(ctx context.Context) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	lastClaimEvent, err := s.injective.LastClaimEvent(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		return uint64(0), err
	}

	return lastClaimEvent.EthereumEventHeight, nil
}

// relayEthEvents checks for events such as a deposit to the Peggy Ethereum contract or a validator set update
// or a transaction batch update. It then responds to these events by performing actions on the Cosmos chain if required
func (s *PeggyOrchestrator) relayEthEvents(
	ctx context.Context,
	startingBlock uint64,
) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	latestHeader, err := s.ethereum.HeaderByNumber(ctx, nil)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to get latest header")
		return 0, err
	}

	// add delay to ensure minimum confirmations are received and block is finalised
	currentBlock := latestHeader.Number.Uint64() - ethBlockConfirmationDelay

	if currentBlock < startingBlock {
		return currentBlock, nil
	}

	if currentBlock > defaultBlocksToSearch+startingBlock {
		currentBlock = startingBlock + defaultBlocksToSearch
	}

	// todo: this will be part of each Get**Events method
	//peggyFilterer, err := wrappers.NewPeggyFilterer(s.peggyContract.Address(), s.ethProvider)
	//if err != nil {
	//	metrics.ReportFuncError(s.svcTags)
	//	err = errors.Wrap(err, "failed to init Peggy events filterer")
	//	return 0, err
	//}

	// todo
	legacyDeposits, err := s.ethereum.GetSendToCosmosEvents(startingBlock, currentBlock)
	if err != nil {
		return 0, err
	}

	//var sendToCosmosEvents []*wrappers.PeggySendToCosmosEvent
	//{
	//
	//	iter, err := peggyFilterer.FilterSendToCosmosEvent(&bind.FilterOpts{
	//		Start: startingBlock,
	//		End:   &currentBlock,
	//	}, nil, nil, nil)
	//	if err != nil {
	//		metrics.ReportFuncError(s.svcTags)
	//		log.WithFields(log.Fields{
	//			"start": startingBlock,
	//			"end":   currentBlock,
	//		}).Errorln("failed to scan past SendToCosmos events from Ethereum")
	//
	//		if !isUnknownBlockErr(err) {
	//			err = errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
	//			return 0, err
	//		} else if iter == nil {
	//			return 0, errors.New("no iterator returned")
	//		}
	//	}
	//
	//	for iter.Next() {
	//		sendToCosmosEvents = append(sendToCosmosEvents, iter.Event)
	//	}
	//
	//	iter.Close()
	//}

	log.WithFields(log.Fields{
		"start":       startingBlock,
		"end":         currentBlock,
		"OldDeposits": legacyDeposits,
	}).Debugln("Scanned SendToCosmos events from Ethereum")

	// todo
	deposits, err := s.ethereum.GetSendToInjectiveEvents(startingBlock, currentBlock)
	if err != nil {
		return 0, err
	}

	//var sendToInjectiveEvents []*wrappers.PeggySendToInjectiveEvent
	//{
	//
	//	iter, err := peggyFilterer.FilterSendToInjectiveEvent(&bind.FilterOpts{
	//		Start: startingBlock,
	//		End:   &currentBlock,
	//	}, nil, nil, nil)
	//	if err != nil {
	//		metrics.ReportFuncError(s.svcTags)
	//		log.WithFields(log.Fields{
	//			"start": startingBlock,
	//			"end":   currentBlock,
	//		}).Errorln("failed to scan past SendToInjective events from Ethereum")
	//
	//		if !isUnknownBlockErr(err) {
	//			err = errors.Wrap(err, "failed to scan past SendToInjective events from Ethereum")
	//			return 0, err
	//		} else if iter == nil {
	//			return 0, errors.New("no iterator returned")
	//		}
	//	}
	//
	//	for iter.Next() {
	//		sendToInjectiveEvents = append(sendToInjectiveEvents, iter.Event)
	//	}
	//
	//	iter.Close()
	//}

	log.WithFields(log.Fields{
		"start":    startingBlock,
		"end":      currentBlock,
		"Deposits": deposits,
	}).Debugln("Scanned SendToInjective events from Ethereum")

	// todo
	withdrawals, err := s.ethereum.GetTransactionBatchExecutedEvents(startingBlock, currentBlock)
	if err != nil {
		return 0, err
	}

	//var transactionBatchExecutedEvents []*wrappers.PeggyTransactionBatchExecutedEvent
	//{
	//	iter, err := peggyFilterer.FilterTransactionBatchExecutedEvent(&bind.FilterOpts{
	//		Start: startingBlock,
	//		End:   &currentBlock,
	//	}, nil, nil)
	//	if err != nil {
	//		metrics.ReportFuncError(s.svcTags)
	//		log.WithFields(log.Fields{
	//			"start": startingBlock,
	//			"end":   currentBlock,
	//		}).Errorln("failed to scan past TransactionBatchExecuted events from Ethereum")
	//
	//		if !isUnknownBlockErr(err) {
	//			err = errors.Wrap(err, "failed to scan past TransactionBatchExecuted events from Ethereum")
	//			return 0, err
	//		} else if iter == nil {
	//			return 0, errors.New("no iterator returned")
	//		}
	//	}
	//
	//	for iter.Next() {
	//		transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
	//	}
	//
	//	iter.Close()
	//}

	log.WithFields(log.Fields{
		"start":     startingBlock,
		"end":       currentBlock,
		"Withdraws": withdrawals,
	}).Debugln("Scanned TransactionBatchExecuted events from Ethereum")

	// todo
	erc20Deployments, err := s.ethereum.GetPeggyERC20DeployedEvents(startingBlock, currentBlock)
	if err != nil {
		return 0, err
	}

	//var erc20DeployedEvents []*wrappers.PeggyERC20DeployedEvent
	//{
	//	iter, err := peggyFilterer.FilterERC20DeployedEvent(&bind.FilterOpts{
	//		Start: startingBlock,
	//		End:   &currentBlock,
	//	}, nil)
	//	if err != nil {
	//		metrics.ReportFuncError(s.svcTags)
	//		log.WithFields(log.Fields{
	//			"start": startingBlock,
	//			"end":   currentBlock,
	//		}).Errorln("failed to scan past FilterERC20Deployed events from Ethereum")
	//
	//		if !isUnknownBlockErr(err) {
	//			err = errors.Wrap(err, "failed to scan past FilterERC20Deployed events from Ethereum")
	//			return 0, err
	//		} else if iter == nil {
	//			return 0, errors.New("no iterator returned")
	//		}
	//	}
	//
	//	for iter.Next() {
	//		erc20DeployedEvents = append(erc20DeployedEvents, iter.Event)
	//	}
	//
	//	iter.Close()
	//}

	log.WithFields(log.Fields{
		"start":         startingBlock,
		"end":           currentBlock,
		"erc20Deployed": erc20Deployments,
	}).Debugln("Scanned FilterERC20Deployed events from Ethereum")

	// todo
	valsetUpdates, err := s.ethereum.GetValsetUpdatedEvents(startingBlock, currentBlock)
	if err != nil {
		return 0, err
	}

	//var valsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent
	//{
	//	iter, err := peggyFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
	//		Start: startingBlock,
	//		End:   &currentBlock,
	//	}, nil)
	//	if err != nil {
	//		metrics.ReportFuncError(s.svcTags)
	//		log.WithFields(log.Fields{
	//			"start": startingBlock,
	//			"end":   currentBlock,
	//		}).Errorln("failed to scan past ValsetUpdatedEvent events from Ethereum")
	//
	//		if !isUnknownBlockErr(err) {
	//			err = errors.Wrap(err, "failed to scan past ValsetUpdatedEvent events from Ethereum")
	//			return 0, err
	//		} else if iter == nil {
	//			return 0, errors.New("no iterator returned")
	//		}
	//	}
	//
	//	for iter.Next() {
	//		valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
	//	}
	//
	//	iter.Close()
	//}

	log.WithFields(log.Fields{
		"start":         startingBlock,
		"end":           currentBlock,
		"valsetUpdates": valsetUpdates,
	}).Debugln("Scanned ValsetUpdatedEvents events from Ethereum")

	// note that starting block overlaps with our last checked block, because we have to deal with
	// the possibility that the relayer was killed after relaying only one of multiple events in a single
	// block, so we also need this routine so make sure we don't send in the first event in this hypothetical
	// multi event block again. In theory we only send all events for every block and that will pass of fail
	// atomically but lets not take that risk.
	lastClaimEvent, err := s.injective.LastClaimEvent(ctx)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.New("failed to query last claim event from backend")
		return 0, err
	}

	legacyDeposits = filterSendToCosmosEventsByNonce(legacyDeposits, lastClaimEvent.EthereumEventNonce)
	deposits = filterSendToInjectiveEventsByNonce(deposits, lastClaimEvent.EthereumEventNonce)
	withdrawals = filterTransactionBatchExecutedEventsByNonce(withdrawals, lastClaimEvent.EthereumEventNonce)
	erc20Deployments = filterERC20DeployedEventsByNonce(erc20Deployments, lastClaimEvent.EthereumEventNonce)
	valsetUpdates = filterValsetUpdateEventsByNonce(valsetUpdates, lastClaimEvent.EthereumEventNonce)

	if len(legacyDeposits) > 0 || len(deposits) > 0 || len(withdrawals) > 0 || len(erc20Deployments) > 0 || len(valsetUpdates) > 0 {
		// todo get eth chain id from the chain
		if err := s.injective.SendEthereumClaims(ctx, lastClaimEvent.EthereumEventNonce, legacyDeposits, deposits, withdrawals, erc20Deployments, valsetUpdates); err != nil {
			metrics.ReportFuncError(s.svcTags)
			err = errors.Wrap(err, "failed to send ethereum claims to Cosmos chain")
			return 0, err
		}
	}

	return currentBlock, nil
}

func filterSendToCosmosEventsByNonce(
	events []*wrappers.PeggySendToCosmosEvent,
	nonce uint64,
) []*wrappers.PeggySendToCosmosEvent {
	res := make([]*wrappers.PeggySendToCosmosEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterSendToInjectiveEventsByNonce(
	events []*wrappers.PeggySendToInjectiveEvent,
	nonce uint64,
) []*wrappers.PeggySendToInjectiveEvent {
	res := make([]*wrappers.PeggySendToInjectiveEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterTransactionBatchExecutedEventsByNonce(
	events []*wrappers.PeggyTransactionBatchExecutedEvent,
	nonce uint64,
) []*wrappers.PeggyTransactionBatchExecutedEvent {
	res := make([]*wrappers.PeggyTransactionBatchExecutedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterERC20DeployedEventsByNonce(
	events []*wrappers.PeggyERC20DeployedEvent,
	nonce uint64,
) []*wrappers.PeggyERC20DeployedEvent {
	res := make([]*wrappers.PeggyERC20DeployedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterValsetUpdateEventsByNonce(
	events []*wrappers.PeggyValsetUpdatedEvent,
	nonce uint64,
) []*wrappers.PeggyValsetUpdatedEvent {
	res := make([]*wrappers.PeggyValsetUpdatedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}
	return res
}

func isUnknownBlockErr(err error) bool {
	// Geth error
	if strings.Contains(err.Error(), "unknown block") {
		return true
	}

	// Parity error
	if strings.Contains(err.Error(), "One of the blocks specified in filter") {
		return true
	}

	return false
}
