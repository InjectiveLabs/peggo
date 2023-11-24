package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

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

	oracle := &ethOracle{
		log:                     log.WithField("loop", "EthOracle"),
		retries:                 s.maxAttempts,
		lastResyncWithInjective: time.Now(),
		lastCheckedEthHeight:    lastConfirmedEthHeight,
	}

	return loops.RunLoop(
		ctx,
		defaultLoopDur,
		func() error { return oracle.run(ctx, s.injective, s.ethereum) },
	)
}

func (s *PeggyOrchestrator) getLastConfirmedEthHeightOnInjective(ctx context.Context) (uint64, error) {
	var (
		lastConfirmedEthHeight      uint64
		getLastConfirmedEthHeightFn = func() error {
			lastClaimEvent, err := s.injective.LastClaimEvent(ctx)
			if err == nil && lastClaimEvent != nil && lastClaimEvent.EthereumEventHeight != 0 {
				lastConfirmedEthHeight = lastClaimEvent.EthereumEventHeight
				return nil
			}

			log.WithError(err).Warningln("failed to get last event claim from Injective. Querying peggy module params...")

			peggyParams, err := s.injective.PeggyParams(ctx)
			if err != nil {
				log.WithError(err).Fatalln("failed to query peggy module params, is injectived running?")
				return err
			}

			lastConfirmedEthHeight = peggyParams.BridgeContractStartHeight
			return nil
		}
	)

	if err := retry.Do(
		getLastConfirmedEthHeightFn,
		retry.Context(ctx),
		retry.Attempts(s.maxAttempts),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Warningf("failed to get last confirmed Ethereum height on Injective, will retry (%d)", n)
		}),
	); err != nil {
		log.WithError(err).Errorln("got error, loop exits")
		return 0, err
	}

	return lastConfirmedEthHeight, nil
}

type ethOracle struct {
	log                     log.Logger
	retries                 uint
	lastResyncWithInjective time.Time
	lastCheckedEthHeight    uint64
}

func (o *ethOracle) run(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) error {
	// Relays events from Ethereum -> Cosmos
	newHeight, err := o.relayEvents(ctx, injective, ethereum)
	if err != nil {
		return err
	}

	o.lastCheckedEthHeight = newHeight
	o.log.WithField("block_num", o.lastCheckedEthHeight).Debugln("last checked Ethereum block")

	if time.Since(o.lastResyncWithInjective) >= 48*time.Hour {
		/**
			Auto re-sync to catch up the nonce. Reasons why event nonce fall behind.
				1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as block produced, there is a chance the event is missed.
				   we need to re-scan this block to ensure events are not missed due to indexing delay.
				2. if validator was in UnBonding state, the claims broadcasted in last iteration are failed.
				3. if infura call failed while filtering events, the peggo missed to broadcast claim events occured in last iteration.
		**/
		if err := o.autoResync(ctx, injective); err != nil {
			return err
		}
	}

	return nil
}

func (o *ethOracle) relayEvents(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) (uint64, error) {
	// Relays events from Ethereum -> Cosmos
	var (
		latestHeight  uint64
		currentHeight = o.lastCheckedEthHeight
	)

	retryFn := func() error {
		latestHeader, err := ethereum.HeaderByNumber(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get latest ethereum header")
		}

		// add delay to ensure minimum confirmations are received and block is finalised
		latestHeight = latestHeader.Number.Uint64() - ethBlockConfirmationDelay
		if latestHeight < currentHeight {
			return nil
		}

		if latestHeight > currentHeight+defaultBlocksToSearch {
			latestHeight = currentHeight + defaultBlocksToSearch
		}

		legacyDeposits, err := ethereum.GetSendToCosmosEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToCosmos events")
		}

		deposits, err := ethereum.GetSendToInjectiveEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToInjective events")
		}

		withdrawals, err := ethereum.GetTransactionBatchExecutedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get TransactionBatchExecuted events")
		}

		erc20Deployments, err := ethereum.GetPeggyERC20DeployedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get ERC20Deployed events")
		}

		valsetUpdates, err := ethereum.GetValsetUpdatedEvents(currentHeight, latestHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get ValsetUpdated events")
		}

		// note that starting block overlaps with our last checked block, because we have to deal with
		// the possibility that the relayer was killed after relaying only one of multiple events in a single
		// block, so we also need this routine so make sure we don't send in the first event in this hypothetical
		// multi event block again. In theory we only send all events for every block and that will pass of fail
		// atomically but lets not take that risk.
		lastClaimEvent, err := injective.LastClaimEvent(ctx)
		if err != nil {
			return errors.New("failed to query last claim event from Injective")
		}

		legacyDeposits = filterSendToCosmosEventsByNonce(legacyDeposits, lastClaimEvent.EthereumEventNonce)
		deposits = filterSendToInjectiveEventsByNonce(deposits, lastClaimEvent.EthereumEventNonce)
		withdrawals = filterTransactionBatchExecutedEventsByNonce(withdrawals, lastClaimEvent.EthereumEventNonce)
		erc20Deployments = filterERC20DeployedEventsByNonce(erc20Deployments, lastClaimEvent.EthereumEventNonce)
		valsetUpdates = filterValsetUpdateEventsByNonce(valsetUpdates, lastClaimEvent.EthereumEventNonce)

		if noEvents := len(legacyDeposits) == 0 &&
			len(deposits) == 0 &&
			len(withdrawals) == 0 &&
			len(erc20Deployments) == 0 &&
			len(valsetUpdates) == 0; noEvents {
			o.log.Debugln("no new events on Ethereum")
			return nil
		}

		if err := injective.SendEthereumClaims(ctx,
			lastClaimEvent.EthereumEventNonce,
			legacyDeposits,
			deposits,
			withdrawals,
			erc20Deployments,
			valsetUpdates,
		); err != nil {
			return errors.Wrap(err, "failed to send event claims to Injective")
		}

		o.log.WithFields(log.Fields{
			"legacy_deposits":   len(legacyDeposits),
			"deposits":          len(deposits),
			"withdrawals":       len(withdrawals),
			"erc20_deployments": len(erc20Deployments),
			"valset_updates":    len(valsetUpdates),
		}).Infoln("sent new event claims to Injective")

		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(o.retries),
		retry.OnRetry(func(n uint, err error) {
			o.log.WithError(err).Warningf("error during Ethereum event checking, will retry (%d)", n)
		}),
	); err != nil {
		o.log.WithError(err).Errorln("got error, loop exits")
		return 0, err
	}

	return latestHeight, nil
}

func (o *ethOracle) autoResync(ctx context.Context, injective InjectiveNetwork) error {
	var latestHeight uint64
	retryFn := func() error {
		lastClaimEvent, err := injective.LastClaimEvent(ctx)
		if err != nil {
			return err
		}

		latestHeight = lastClaimEvent.EthereumEventHeight
		return nil
	}

	if err := retry.Do(retryFn,
		retry.Context(ctx),
		retry.Attempts(o.retries),
		retry.OnRetry(func(n uint, err error) {
			o.log.WithError(err).Warningf("failed to get last confirmed eth height, will retry (%d)", n)
		}),
	); err != nil {
		o.log.WithError(err).Errorln("got error, loop exits")
		return err
	}

	o.lastCheckedEthHeight = latestHeight
	o.lastResyncWithInjective = time.Now()

	o.log.WithFields(log.Fields{
		"last_resync":             o.lastResyncWithInjective.String(),
		"last_checked_eth_height": o.lastCheckedEthHeight,
	}).Infoln("auto resync")

	return nil
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
