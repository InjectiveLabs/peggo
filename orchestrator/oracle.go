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
		LoopDuration:            defaultLoopDur,
		LastObservedEthHeight:   lastObservedBlock,
		LastResyncWithInjective: time.Now(),
	}

	s.logger.WithField("loop_duration", oracle.LoopDuration.String()).Debugln("starting EthOracle...")

	return loops.RunLoop(ctx, oracle.LoopDuration, func() error {
		return oracle.ObserveEthEvents(ctx)
	})
}

type ethOracle struct {
	*PeggyOrchestrator
	Injective               cosmos.Network
	Ethereum                ethereum.Network
	LoopDuration            time.Duration
	LastResyncWithInjective time.Time
	LastObservedEthHeight   uint64
}

func (l *ethOracle) Logger() log.Logger {
	return l.logger.WithField("loop", "EthOracle")
}

func (l *ethOracle) ObserveEthEvents(ctx context.Context) error {
	latestHeight, err := l.getLatestEthHeight(ctx)
	if err != nil {
		return err
	}

	// not enough blocks on ethereum yet
	if latestHeight <= ethBlockConfirmationDelay {
		return nil
	}

	// ensure that latest block has minimum confirmations
	latestHeight = latestHeight - ethBlockConfirmationDelay
	if latestHeight <= l.LastObservedEthHeight {
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

	/** Auto re-sync to catch up the nonce. Reasons why event nonce fall behind.
		1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as block produced, there is a chance the event is missed.
	   	we need to re-scan this block to ensure events are not missed due to indexing delay.
		2. if validator was in UnBonding state, the claims broadcasted in last iteration are failed.
		3. if infura call failed while filtering events, the peggo missed to broadcast claim events occured in last iteration.
	*/
	if time.Since(l.LastResyncWithInjective) >= 48*time.Hour {
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

	if err := retryOnErr(ctx, l.Logger(), scanEthEventsFn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return ethEvents{}, err
	}

	return events, nil
}

func (l *ethOracle) getLatestEthHeight(ctx context.Context) (uint64, error) {
	var latestHeight uint64
	if err := retryOnErr(ctx, l.Logger(), func() error {
		latestHeader, err := l.Ethereum.GetHeaderByNumber(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get latest ethereum header")
		}

		latestHeight = latestHeader.Number.Uint64()
		return nil
	}); err != nil {
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

		newEvents := events.Filter(lastClaim.EthereumEventNonce)
		if newEvents.Num() == 0 {
			l.Logger().WithField("last_claimed_event_nonce", lastClaim.EthereumEventNonce).Infoln("no new events on Ethereum")
			return nil
		}

		sortedEvents := newEvents.Sort()
		for _, event := range sortedEvents {
			if err := l.sendEthEventClaim(ctx, event); err != nil {
				return err
			}

			// Considering blockTime=1s on Injective chain, adding Sleep to make sure new event is sent
			// only after previous event is executed successfully. Otherwise it will through `non contiguous event nonce` failing CheckTx.
			time.Sleep(1200 * time.Millisecond)
		}

		l.Logger().WithField("claims", len(sortedEvents)).Infoln("sent new event claims to Injective")

		return nil
	}

	if err := retryOnErr(ctx, l.Logger(), sendEventsFn); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	return nil
}

func (l *ethOracle) autoResync(ctx context.Context) error {
	var latestHeight uint64
	if err := retryOnErr(ctx, l.Logger(), func() (err error) {
		latestHeight, err = l.getLastClaimBlockHeight(ctx, l.Injective)
		return
	}); err != nil {
		l.Logger().WithError(err).Errorln("got error, loop exits")
		return err
	}

	l.LastObservedEthHeight = latestHeight
	l.LastResyncWithInjective = time.Now()

	l.Logger().WithFields(log.Fields{"last_resync_time": l.LastResyncWithInjective.String(), "last_claimed_eth_height": l.LastObservedEthHeight}).Infoln("auto resync with last claimed event on Injective")

	return nil
}

func (l *ethOracle) sendEthEventClaim(ctx context.Context, event any) error {
	switch e := event.(type) {
	case *peggyevents.PeggySendToCosmosEvent:
		return l.Injective.SendOldDepositClaim(ctx, e)
	case *peggyevents.PeggySendToInjectiveEvent:
		return l.Injective.SendDepositClaim(ctx, e)
	case *peggyevents.PeggyValsetUpdatedEvent:
		return l.Injective.SendValsetClaim(ctx, e)
	case *peggyevents.PeggyTransactionBatchExecutedEvent:
		return l.Injective.SendWithdrawalClaim(ctx, e)
	case *peggyevents.PeggyERC20DeployedEvent:
		return l.Injective.SendERC20DeployedClaim(ctx, e)
	default:
		panic(errors.Errorf("unknown event type %T", e))
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

func (e ethEvents) Sort() []any {
	events := make([]any, 0, e.Num())

	for _, deposit := range e.OldDeposits {
		events = append(events, deposit)
	}

	for _, deposit := range e.Deposits {
		events = append(events, deposit)
	}

	for _, withdrawal := range e.Withdrawals {
		events = append(events, withdrawal)
	}

	for _, deployment := range e.ERC20Deployments {
		events = append(events, deployment)
	}

	for _, vs := range e.ValsetUpdates {
		events = append(events, vs)
	}

	eventNonce := func(event any) uint64 {
		switch e := event.(type) {
		case *peggyevents.PeggySendToCosmosEvent:
			return e.EventNonce.Uint64()
		case *peggyevents.PeggySendToInjectiveEvent:
			return e.EventNonce.Uint64()
		case *peggyevents.PeggyValsetUpdatedEvent:
			return e.EventNonce.Uint64()
		case *peggyevents.PeggyTransactionBatchExecutedEvent:
			return e.EventNonce.Uint64()
		case *peggyevents.PeggyERC20DeployedEvent:
			return e.EventNonce.Uint64()
		default:
			panic(errors.Errorf("unknown event type %T", e))
		}
	}

	// sort by nonce
	sort.Slice(events, func(i, j int) bool {
		return eventNonce(events[i]) < eventNonce(events[j])
	})

	return events
}
