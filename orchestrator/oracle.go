package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

// todo: this is outdated, need to update
// Considering blocktime of up to 3 seconds approx on the Injective Chain and an oracle loop duration = 1 minute,
// we broadcast only 20 events in each iteration.
// So better to search only 20 blocks to ensure all the events are broadcast to Injective Chain without misses.
const (
	ethBlockConfirmationDelay uint64 = 96
	defaultBlocksToSearch     uint64 = 2000
)

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
func (s *PeggyOrchestrator) EthOracleMainLoop(ctx context.Context) error {
	lastConfirmedEthHeight, err := s.getLastConfirmedEthHeightOnInjective(ctx)
	if err != nil {
		return err
	}

	s.logger.Debugln("last observed ethereum block", lastConfirmedEthHeight)

	loop := ethOracleLoop{
		PeggyOrchestrator:       s,
		loopDuration:            defaultLoopDur,
		lastCheckedEthHeight:    lastConfirmedEthHeight,
		lastResyncWithInjective: time.Now(),
	}

	return loop.Run(ctx)
}

func (s *PeggyOrchestrator) getLastConfirmedEthHeightOnInjective(ctx context.Context) (uint64, error) {
	var lastConfirmedEthHeight uint64
	getLastConfirmedEthHeightFn := func() (err error) {
		lastConfirmedEthHeight, err = s.getLastClaimBlockHeight(ctx)
		if lastConfirmedEthHeight == 0 {
			peggyParams, err := s.inj.PeggyParams(ctx)
			if err != nil {
				s.logger.WithError(err).Fatalln("unable to query peggy module params, is injectived running?")
				return err
			}

			lastConfirmedEthHeight = peggyParams.BridgeContractStartHeight
		}
		return
	}

	if err := retry.Do(getLastConfirmedEthHeightFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			s.logger.WithError(err).Warningf("failed to get last confirmed Ethereum height on Injective, will retry (%d)", n)
		}),
	); err != nil {
		s.logger.WithError(err).Errorln("got error, loop exits")
		return 0, err
	}

	return lastConfirmedEthHeight, nil
}

func (s *PeggyOrchestrator) getLastClaimBlockHeight(ctx context.Context) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	claim, err := s.inj.LastClaimEvent(ctx)
	if err != nil {
		return 0, err
	}

	return claim.EthereumEventHeight, nil
}

type ethOracleLoop struct {
	*PeggyOrchestrator
	loopDuration            time.Duration
	lastResyncWithInjective time.Time
	lastCheckedEthHeight    uint64
}

func (l *ethOracleLoop) Logger() log.Logger {
	return l.logger.WithField("loop", "EthOracle")
}

func (l *ethOracleLoop) Run(ctx context.Context) error {
	return loops.RunLoop(ctx, l.loopDuration, func() error {
		return l.observeEthEvents(ctx)
	})
}

func (l *ethOracleLoop) observeEthEvents(ctx context.Context) error {
	newHeight, err := l.relayEvents(ctx)
	if err != nil {
		return err
	}

	l.Logger().WithFields(log.Fields{"block_start": l.lastCheckedEthHeight, "block_end": newHeight}).Debugln("scanned Ethereum blocks for events")
	l.lastCheckedEthHeight = newHeight

	if time.Since(l.lastResyncWithInjective) >= 48*time.Hour {
		/**
			Auto re-sync to catch up the nonce. Reasons why event nonce fall behind.
				1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as block produced, there is a chance the event is missed.
				   we need to re-scan this block to ensure events are not missed due to indexing delay.
				2. if validator was in UnBonding state, the claims broadcasted in last iteration are failed.
				3. if infura call failed while filtering events, the peggo missed to broadcast claim events occured in last iteration.
		**/
		if err := l.autoResync(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (l *ethOracleLoop) relayEvents(ctx context.Context) (uint64, error) {
	var (
		currentHeight = l.lastCheckedEthHeight
		latestHeight  uint64
	)

	scanEthBlocksAndRelayEventsFn := func() error {
		metrics.ReportFuncCall(l.svcTags)
		doneFn := metrics.ReportFuncTiming(l.svcTags)
		defer doneFn()

		latestHeader, err := l.eth.HeaderByNumber(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get latest ethereum header")
		}

		latestHeight = latestHeader.Number.Uint64()

		if latestHeight < ethBlockConfirmationDelay {
			latestHeight = currentHeight // no blocks scanned
			return nil
		}

		// add delay to ensure minimum confirmations are received and block is finalised
		latestHeight = latestHeight - ethBlockConfirmationDelay
		if latestHeight <= currentHeight {
			latestHeight = currentHeight // no blocks scanned
			return nil
		}

		// calculate right bound of block search
		if currentHeight+defaultBlocksToSearch < latestHeight {
			latestHeight = currentHeight + defaultBlocksToSearch
		}

		legacyDeposits, err := l.eth.GetSendToCosmosEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToCosmos events")
		}

		deposits, err := l.eth.GetSendToInjectiveEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToInjective events")
		}

		withdrawals, err := l.eth.GetTransactionBatchExecutedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get TransactionBatchExecuted events")
		}

		erc20Deployments, err := l.eth.GetPeggyERC20DeployedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get ERC20Deployed events")
		}

		valsetUpdates, err := l.eth.GetValsetUpdatedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get ValsetUpdated events")
		}

		// note that starting block overlaps with our last checked block, because we have to deal with
		// the possibility that the relayer was killed after relaying only one of multiple events in a single
		// block, so we also need this routine so make sure we don't send in the first event in this hypothetical
		// multi event block again. In theory we only send all events for every block and that will pass of fail
		// atomically but lets not take that risk.
		lastClaimEvent, err := l.inj.LastClaimEvent(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to query last claim event from Injective")
		}

		l.logger.WithFields(log.Fields{
			"event_nonce": lastClaimEvent.EthereumEventNonce,
			"event_block": lastClaimEvent.EthereumEventHeight,
		}).Debugln("last Ethereum claim event on Injective")

		legacyDeposits = filterSendToCosmosEventsByNonce(legacyDeposits, lastClaimEvent.EthereumEventNonce)
		deposits = filterSendToInjectiveEventsByNonce(deposits, lastClaimEvent.EthereumEventNonce)
		withdrawals = filterTransactionBatchExecutedEventsByNonce(withdrawals, lastClaimEvent.EthereumEventNonce)
		erc20Deployments = filterERC20DeployedEventsByNonce(erc20Deployments, lastClaimEvent.EthereumEventNonce)
		valsetUpdates = filterValsetUpdateEventsByNonce(valsetUpdates, lastClaimEvent.EthereumEventNonce)

		if noEvents := len(legacyDeposits) == 0 && len(deposits) == 0 && len(withdrawals) == 0 &&
			len(erc20Deployments) == 0 && len(valsetUpdates) == 0; noEvents {
			l.Logger().Debugln("no new events on Ethereum")
			return nil
		}

		if err := l.inj.SendEthereumClaims(ctx,
			lastClaimEvent.EthereumEventNonce,
			legacyDeposits,
			deposits,
			withdrawals,
			erc20Deployments,
			valsetUpdates,
		); err != nil {
			return errors.Wrap(err, "failed to send event claims to Injective")
		}

		l.Logger().WithFields(log.Fields{
			"legacy_deposits":   len(legacyDeposits),
			"deposits":          len(deposits),
			"withdrawals":       len(withdrawals),
			"erc20_deployments": len(erc20Deployments),
			"valset_updates":    len(valsetUpdates),
		}).Infoln("sent new claims to Injective")

		return nil
	}

	if err := retry.Do(scanEthBlocksAndRelayEventsFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("error during Ethereum event checking, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return 0, err
	}

	return latestHeight, nil
}

func (l *ethOracleLoop) autoResync(ctx context.Context) error {
	var latestHeight uint64
	getLastClaimEventFn := func() (err error) {
		latestHeight, err = l.getLastClaimBlockHeight(ctx)
		return
	}

	if err := retry.Do(getLastClaimEventFn,
		retry.Context(ctx),
		retry.Attempts(l.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			l.Logger().WithError(err).Warningf("failed to get last confirmed eth height, will retry (%d)", n)
		}),
	); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.lastCheckedEthHeight = latestHeight
	l.lastResyncWithInjective = time.Now()

	l.Logger().WithFields(log.Fields{"last_resync_time": l.lastResyncWithInjective.String(), "last_confirmed_eth_height": l.lastCheckedEthHeight}).Infoln("auto resync event nonce with Injective")

	return nil
}

func filterSendToCosmosEventsByNonce(
	events []*peggyevents.PeggySendToCosmosEvent,
	nonce uint64,
) []*peggyevents.PeggySendToCosmosEvent {
	res := make([]*peggyevents.PeggySendToCosmosEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterSendToInjectiveEventsByNonce(
	events []*peggyevents.PeggySendToInjectiveEvent,
	nonce uint64,
) []*peggyevents.PeggySendToInjectiveEvent {
	res := make([]*peggyevents.PeggySendToInjectiveEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterTransactionBatchExecutedEventsByNonce(
	events []*peggyevents.PeggyTransactionBatchExecutedEvent,
	nonce uint64,
) []*peggyevents.PeggyTransactionBatchExecutedEvent {
	res := make([]*peggyevents.PeggyTransactionBatchExecutedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterERC20DeployedEventsByNonce(
	events []*peggyevents.PeggyERC20DeployedEvent,
	nonce uint64,
) []*peggyevents.PeggyERC20DeployedEvent {
	res := make([]*peggyevents.PeggyERC20DeployedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}

	return res
}

func filterValsetUpdateEventsByNonce(
	events []*peggyevents.PeggyValsetUpdatedEvent,
	nonce uint64,
) []*peggyevents.PeggyValsetUpdatedEvent {
	res := make([]*peggyevents.PeggyValsetUpdatedEvent, 0, len(events))

	for _, ev := range events {
		if ev.EventNonce.Uint64() > nonce {
			res = append(res, ev)
		}
	}
	return res
}
