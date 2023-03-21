package orchestrator

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"

	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

// Considering blocktime of up to 3 seconds approx on the Injective Chain and an oracle loop duration = 1 minute,
// we broadcast only 20 events in each iteration.
// So better to search only 20 blocks to ensure all the events are broadcast to Injective Chain without misses.
const defaultBlocksToSearch = 10_000

const ethBlockConfirmationDelay = 12

// CheckForEvents checks for events such as a deposit to the Peggy Ethereum contract or a validator set update
// or a transaction batch update. It then responds to these events by performing actions on the Cosmos chain if required
func (s *peggyOrchestrator) CheckForEvents(
	ctx context.Context,
	startingBlock uint64,
) (currentBlock uint64, err error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to get latest header")
		return 0, err
	}

	// add delay to ensure minimum confirmations are received and block is finalised
	currentBlock = latestHeader.Number.Uint64() - uint64(ethBlockConfirmationDelay)

	if currentBlock < startingBlock {
		return currentBlock, nil
	}

	if (currentBlock - startingBlock) > defaultBlocksToSearch {
		currentBlock = startingBlock + defaultBlocksToSearch
	}

	peggyFilterer, err := wrappers.NewPeggyFilterer(s.peggyContract.Address(), s.ethProvider)
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.Wrap(err, "failed to init Peggy events filterer")
		return 0, err
	}

	var sendToCosmosEvents []*wrappers.PeggySendToCosmosEvent
	{

		iter, err := peggyFilterer.FilterSendToCosmosEvent(&bind.FilterOpts{
			Start: startingBlock,
			End:   &currentBlock,
		}, nil, nil, nil)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			log.WithFields(log.Fields{
				"start": startingBlock,
				"end":   currentBlock,
			}).Errorln("failed to scan past SendToCosmos events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
				return 0, err
			} else if iter == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iter.Next() {
			sendToCosmosEvents = append(sendToCosmosEvents, iter.Event)
		}

		iter.Close()
	}

	log.WithFields(log.Fields{
		"start":       startingBlock,
		"end":         currentBlock,
		"OldDeposits": sendToCosmosEvents,
	}).Debugln("Scanned SendToCosmos events from Ethereum")

	var sendToInjectiveEvents []*wrappers.PeggySendToInjectiveEvent
	{

		iter, err := peggyFilterer.FilterSendToInjectiveEvent(&bind.FilterOpts{
			Start: startingBlock,
			End:   &currentBlock,
		}, nil, nil, nil)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			log.WithFields(log.Fields{
				"start": startingBlock,
				"end":   currentBlock,
			}).Errorln("failed to scan past SendToInjective events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past SendToInjective events from Ethereum")
				return 0, err
			} else if iter == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iter.Next() {
			sendToInjectiveEvents = append(sendToInjectiveEvents, iter.Event)
		}

		iter.Close()
	}

	log.WithFields(log.Fields{
		"start":    startingBlock,
		"end":      currentBlock,
		"Deposits": sendToInjectiveEvents,
	}).Debugln("Scanned SendToInjective events from Ethereum")

	var transactionBatchExecutedEvents []*wrappers.PeggyTransactionBatchExecutedEvent
	{
		iter, err := peggyFilterer.FilterTransactionBatchExecutedEvent(&bind.FilterOpts{
			Start: startingBlock,
			End:   &currentBlock,
		}, nil, nil)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			log.WithFields(log.Fields{
				"start": startingBlock,
				"end":   currentBlock,
			}).Errorln("failed to scan past TransactionBatchExecuted events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past TransactionBatchExecuted events from Ethereum")
				return 0, err
			} else if iter == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iter.Next() {
			transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
		}

		iter.Close()
	}
	log.WithFields(log.Fields{
		"start":     startingBlock,
		"end":       currentBlock,
		"Withdraws": transactionBatchExecutedEvents,
	}).Debugln("Scanned TransactionBatchExecuted events from Ethereum")

	var erc20DeployedEvents []*wrappers.PeggyERC20DeployedEvent
	{
		iter, err := peggyFilterer.FilterERC20DeployedEvent(&bind.FilterOpts{
			Start: startingBlock,
			End:   &currentBlock,
		}, nil)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			log.WithFields(log.Fields{
				"start": startingBlock,
				"end":   currentBlock,
			}).Errorln("failed to scan past FilterERC20Deployed events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past FilterERC20Deployed events from Ethereum")
				return 0, err
			} else if iter == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iter.Next() {
			erc20DeployedEvents = append(erc20DeployedEvents, iter.Event)
		}

		iter.Close()
	}
	log.WithFields(log.Fields{
		"start":         startingBlock,
		"end":           currentBlock,
		"erc20Deployed": erc20DeployedEvents,
	}).Debugln("Scanned FilterERC20Deployed events from Ethereum")

	var valsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent
	{
		iter, err := peggyFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
			Start: startingBlock,
			End:   &currentBlock,
		}, nil)
		if err != nil {
			metrics.ReportFuncError(s.svcTags)
			log.WithFields(log.Fields{
				"start": startingBlock,
				"end":   currentBlock,
			}).Errorln("failed to scan past ValsetUpdatedEvent events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past ValsetUpdatedEvent events from Ethereum")
				return 0, err
			} else if iter == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iter.Next() {
			valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
		}

		iter.Close()
	}

	log.WithFields(log.Fields{
		"start":         startingBlock,
		"end":           currentBlock,
		"valsetUpdates": valsetUpdatedEvents,
	}).Debugln("Scanned ValsetUpdatedEvents events from Ethereum")

	// note that starting block overlaps with our last checked block, because we have to deal with
	// the possibility that the relayer was killed after relaying only one of multiple events in a single
	// block, so we also need this routine so make sure we don't send in the first event in this hypothetical
	// multi event block again. In theory we only send all events for every block and that will pass of fail
	// atomically but lets not take that risk.
	lastClaimEvent, err := s.cosmosQueryClient.LastClaimEventByAddr(ctx, s.peggyBroadcastClient.AccFromAddress())
	if err != nil {
		metrics.ReportFuncError(s.svcTags)
		err = errors.New("failed to query last claim event from backend")
		return 0, err
	}

	oldDeposits := filterSendToCosmosEventsByNonce(sendToCosmosEvents, lastClaimEvent.EthereumEventNonce)
	deposits := filterSendToInjectiveEventsByNonce(sendToInjectiveEvents, lastClaimEvent.EthereumEventNonce)
	withdraws := filterTransactionBatchExecutedEventsByNonce(transactionBatchExecutedEvents, lastClaimEvent.EthereumEventNonce)
	erc20Deployments := filterERC20DeployedEventsByNonce(erc20DeployedEvents, lastClaimEvent.EthereumEventNonce)
	valsetUpdates := filterValsetUpdateEventsByNonce(valsetUpdatedEvents, lastClaimEvent.EthereumEventNonce)

	if len(oldDeposits) > 0 || len(deposits) > 0 || len(withdraws) > 0 || len(erc20Deployments) > 0 || len(valsetUpdates) > 0 {
		// todo get eth chain id from the chain
		if err := s.peggyBroadcastClient.SendEthereumClaims(ctx, lastClaimEvent.EthereumEventNonce, oldDeposits, deposits, withdraws, erc20Deployments, valsetUpdates); err != nil {
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
