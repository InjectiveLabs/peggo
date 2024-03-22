package orchestrator

import (
	"context"
	"sort"
	"time"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
)

const (
	// Minimum number of confirmations for an Ethereum block to be considered valid
	ethBlockConfirmationDelay uint64 = 12

	// Maximum block range for Ethereum event query. If the orchestrator has been offline for a long time,
	// the oracle loop can potentially run longer than defaultLoopDur due to a surge of events. This usually happens
	// when there are more than ~50 events to claim in a single run.
	defaultBlocksToSearch uint64 = 2000

	// Auto re-sync to catch up the validator's last observed event nonce. Reasons why event nonce fall behind:
	// 1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as block produced, there is a chance the event is missed.
	//  We need to re-scan this block to ensure events are not missed due to indexing delay.
	// 2. if validator was in UnBonding state, the claims broadcasted in last iteration are failed.
	// 3. if infura call failed while filtering events, the peggo missed to broadcast claim events occured in last iteration.
	// 4. if an event was sent to Injective successfully but didn't end up in a block, subsequently claimed events will be ignored until a manual restart or auto re-sync occur
	resyncInterval = 3 * time.Hour
)

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
func (s *PeggyOrchestrator) EthOracleMainLoop(
	ctx context.Context,
	inj cosmos.Network,
	eth ethereum.Network,
	lastObservedBlock uint64,
) error {
	oracle := ethOracle{
		PeggyOrchestrator:       s,
		Injective:               inj,
		Ethereum:                eth,
		LastObservedEthHeight:   lastObservedBlock,
		LastResyncWithInjective: time.Now(),
	}

	s.logger.WithField("loop_duration", defaultLoopDur.String()).Debugln("starting EthOracle...")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		return oracle.ObserveEthEvents(ctx)
	})
}

type ethOracle struct {
	*PeggyOrchestrator
	Injective               cosmos.Network
	Ethereum                ethereum.Network
	LastResyncWithInjective time.Time
	LastObservedEthHeight   uint64
}

func (l *ethOracle) Logger() log.Logger {
	return l.logger.WithField("loop", "EthOracle")
}

func (l *ethOracle) ObserveEthEvents(ctx context.Context) error {
	// check if validator is in the active set since claims will fail otherwise
	vs, err := l.Injective.CurrentValset(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get current valset on Injective")
	}

	bonded := false
	for _, v := range vs.Members {
		if l.ethAddr.Hex() == v.EthereumAddress {
			bonded = true
		}
	}

	if !bonded {
		l.Logger().WithFields(log.Fields{
			"orchestrator_addr":   l.injAddr.String(),
			"eth_addr":            l.ethAddr.String(),
			"latest_valset_block": vs.Height,
		}).Debugln("validator not in active set, returning...")
		return nil
	}

	latestHeight, err := l.getLatestEthHeight(ctx)
	if err != nil {
		return err
	}

	// not enough blocks on ethereum yet
	if latestHeight <= ethBlockConfirmationDelay {
		l.Logger().Debugln("not enough blocks on Ethereum")
		return nil
	}

	// ensure that latest block has minimum confirmations
	latestHeight = latestHeight - ethBlockConfirmationDelay
	if latestHeight <= l.LastObservedEthHeight {
		l.Logger().WithFields(log.Fields{"latest": latestHeight, "observed": l.LastObservedEthHeight}).Debugln("latest Ethereum height already observed")
		return nil
	}

	// ensure the block range is within defaultBlocksToSearch
	if latestHeight > l.LastObservedEthHeight+defaultBlocksToSearch {
		latestHeight = l.LastObservedEthHeight + defaultBlocksToSearch
	}

	events, err := l.getEthEvents(ctx, l.LastObservedEthHeight, latestHeight)
	if err != nil {
		return err
	}

	if err := l.sendNewEventClaims(ctx, events); err != nil {
		return err
	}

	l.Logger().WithFields(log.Fields{"block_start": l.LastObservedEthHeight, "block_end": latestHeight}).Debugln("scanned Ethereum blocks")
	l.LastObservedEthHeight = latestHeight

	if time.Since(l.LastResyncWithInjective) >= resyncInterval {
		if err := l.autoResync(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (l *ethOracle) getEthEvents(ctx context.Context, startBlock, endBlock uint64) (ethEvents, error) {
	events := ethEvents{}
	scanEthEventsFn := func() error {
		legacyDeposits, err := l.Ethereum.GetSendToCosmosEvents(startBlock, endBlock)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToCosmos events")
		}

		deposits, err := l.Ethereum.GetSendToInjectiveEvents(startBlock, endBlock)
		if err != nil {
			return errors.Wrap(err, "failed to get SendToInjective events")
		}

		withdrawals, err := l.Ethereum.GetTransactionBatchExecutedEvents(startBlock, endBlock)
		if err != nil {
			return errors.Wrap(err, "failed to get TransactionBatchExecuted events")
		}

		erc20Deployments, err := l.Ethereum.GetPeggyERC20DeployedEvents(startBlock, endBlock)
		if err != nil {
			return errors.Wrap(err, "failed to get ERC20Deployed events")
		}

		valsetUpdates, err := l.Ethereum.GetValsetUpdatedEvents(startBlock, endBlock)
		if err != nil {
			return errors.Wrap(err, "failed to get ValsetUpdated events")
		}

		events.OldDeposits = legacyDeposits
		events.Deposits = deposits
		events.Withdrawals = withdrawals
		events.ValsetUpdates = valsetUpdates
		events.ERC20Deployments = erc20Deployments

		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), scanEthEventsFn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return ethEvents{}, err
	}

	return events, nil
}

func (l *ethOracle) getLatestEthHeight(ctx context.Context) (uint64, error) {
	latestHeight := uint64(0)
	fn := func() error {
		h, err := l.Ethereum.GetHeaderByNumber(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get latest ethereum header")
		}

		latestHeight = h.Number.Uint64()
		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), fn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return 0, err
	}

	return latestHeight, nil
}

func (l *ethOracle) sendNewEventClaims(ctx context.Context, events ethEvents) error {
	sendEventsFn := func() error {
		lastClaim, err := l.Injective.LastClaimEventByAddr(ctx, l.injAddr)
		if err != nil {
			return err
		}

		newEvents := events.Filter(lastClaim.EthereumEventNonce).Sort()
		if len(newEvents) == 0 {
			l.Logger().WithField("last_claimed_event_nonce", lastClaim.EthereumEventNonce).Infoln("no new events on Ethereum")
			return nil
		}

		if expected, actual := lastClaim.EthereumEventNonce+1, newEvents[0].Nonce(); expected != actual {
			l.Logger().WithFields(log.Fields{
				"expected_nonce":    expected,
				"actual_nonce":      newEvents[0].Nonce(),
				"time_until_resync": time.Until(l.LastResyncWithInjective.Add(resyncInterval)).String(),
			}).Infoln("orchestrator missed an event claim. Restart your peggo or wait until resync")

			return nil
		}

		for _, event := range newEvents {
			if err := l.sendEthEventClaim(ctx, event); err != nil {
				return err
			}

			// Considering blockTime=1s on Injective chain, adding Sleep to make sure new event is sent
			// only after previous event is executed successfully. Otherwise it will through `non contiguous event nonce` failing CheckTx.
			time.Sleep(1200 * time.Millisecond)
		}

		l.Logger().WithField("claims", len(newEvents)).Infoln("sent new event claims to Injective")

		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), sendEventsFn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	return nil
}

func (l *ethOracle) autoResync(ctx context.Context) error {
	latestHeight := uint64(0)
	fn := func() error {
		h, err := l.getLastClaimBlockHeight(ctx, l.Injective)
		if err != nil {
			return err
		}

		latestHeight = h
		return nil
	}

	if err := retryFnOnErr(ctx, l.Logger(), fn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.LastObservedEthHeight = latestHeight
	l.LastResyncWithInjective = time.Now()

	l.Logger().WithFields(log.Fields{"last_resync_time": l.LastResyncWithInjective.String(), "last_claimed_eth_height": l.LastObservedEthHeight}).Infoln("auto resync with last claimed event on Injective")

	return nil
}

func (l *ethOracle) sendEthEventClaim(ctx context.Context, ev event) error {
	switch e := ev.(type) {
	case *oldDeposit:
		ev := peggyevents.PeggySendToCosmosEvent(*e)
		return l.Injective.SendOldDepositClaim(ctx, &ev)
	case *deposit:
		ev := peggyevents.PeggySendToInjectiveEvent(*e)
		return l.Injective.SendDepositClaim(ctx, &ev)
	case *valsetUpdate:
		ev := peggyevents.PeggyValsetUpdatedEvent(*e)
		return l.Injective.SendValsetClaim(ctx, &ev)
	case *withdrawal:
		ev := peggyevents.PeggyTransactionBatchExecutedEvent(*e)
		return l.Injective.SendWithdrawalClaim(ctx, &ev)
	case *erc20Deployment:
		ev := peggyevents.PeggyERC20DeployedEvent(*e)
		return l.Injective.SendERC20DeployedClaim(ctx, &ev)
	default:
		panic(errors.Errorf("unknown ev type %T", e))
	}
}

type ethEvents struct {
	OldDeposits      []*peggyevents.PeggySendToCosmosEvent
	Deposits         []*peggyevents.PeggySendToInjectiveEvent
	Withdrawals      []*peggyevents.PeggyTransactionBatchExecutedEvent
	ValsetUpdates    []*peggyevents.PeggyValsetUpdatedEvent
	ERC20Deployments []*peggyevents.PeggyERC20DeployedEvent
}

func (e ethEvents) Num() int {
	return len(e.OldDeposits) + len(e.Deposits) + len(e.Withdrawals) + len(e.ValsetUpdates) + len(e.ERC20Deployments)
}

func (e ethEvents) Filter(nonce uint64) ethEvents {
	var oldDeposits []*peggyevents.PeggySendToCosmosEvent
	for _, d := range e.OldDeposits {
		if d.EventNonce.Uint64() > nonce {
			oldDeposits = append(oldDeposits, d)
		}
	}

	var deposits []*peggyevents.PeggySendToInjectiveEvent
	for _, d := range e.Deposits {
		if d.EventNonce.Uint64() > nonce {
			deposits = append(deposits, d)
		}
	}

	var withdrawals []*peggyevents.PeggyTransactionBatchExecutedEvent
	for _, w := range e.Withdrawals {
		if w.EventNonce.Uint64() > nonce {
			withdrawals = append(withdrawals, w)
		}
	}

	var valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent
	for _, vs := range e.ValsetUpdates {
		if vs.EventNonce.Uint64() > nonce {
			valsetUpdates = append(valsetUpdates, vs)
		}
	}

	var erc20Deployments []*peggyevents.PeggyERC20DeployedEvent
	for _, d := range e.ERC20Deployments {
		if d.EventNonce.Uint64() > nonce {
			erc20Deployments = append(erc20Deployments, d)
		}
	}

	return ethEvents{
		OldDeposits:      oldDeposits,
		Deposits:         deposits,
		Withdrawals:      withdrawals,
		ValsetUpdates:    valsetUpdates,
		ERC20Deployments: erc20Deployments,
	}
}

func (e ethEvents) Sort() []event {
	events := make([]event, 0, e.Num())

	for _, d := range e.OldDeposits {
		ev := oldDeposit(*d)
		events = append(events, &ev)
	}

	for _, d := range e.Deposits {
		ev := deposit(*d)
		events = append(events, &ev)
	}

	for _, w := range e.Withdrawals {
		ev := withdrawal(*w)
		events = append(events, &ev)
	}

	for _, deployment := range e.ERC20Deployments {
		ev := erc20Deployment(*deployment)
		events = append(events, &ev)
	}

	for _, vs := range e.ValsetUpdates {
		ev := valsetUpdate(*vs)
		events = append(events, &ev)
	}

	// sort by nonce
	sort.Slice(events, func(i, j int) bool {
		return events[i].Nonce() < events[j].Nonce()
	})

	return events
}

type (
	oldDeposit      peggyevents.PeggySendToCosmosEvent
	deposit         peggyevents.PeggySendToInjectiveEvent
	valsetUpdate    peggyevents.PeggyValsetUpdatedEvent
	withdrawal      peggyevents.PeggyTransactionBatchExecutedEvent
	erc20Deployment peggyevents.PeggyERC20DeployedEvent

	event interface {
		Nonce() uint64
	}
)

func (o *oldDeposit) Nonce() uint64 {
	return o.EventNonce.Uint64()
}

func (o *deposit) Nonce() uint64 {
	return o.EventNonce.Uint64()
}

func (o *valsetUpdate) Nonce() uint64 {
	return o.EventNonce.Uint64()
}

func (o *withdrawal) Nonce() uint64 {
	return o.EventNonce.Uint64()
}

func (o *erc20Deployment) Nonce() uint64 {
	return o.EventNonce.Uint64()
}
