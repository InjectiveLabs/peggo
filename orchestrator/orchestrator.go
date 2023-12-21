package orchestrator

import (
	"context"
	"github.com/avast/retry-go"
	"math/big"
	"time"

	eth "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

type PriceFeed interface {
	QueryUSDPrice(address eth.Address) (float64, error)
}

type InjectiveNetwork interface {
	PeggyParams(ctx context.Context) (*peggytypes.Params, error)
	GetBlockCreationTime(ctx context.Context, height int64) (time.Time, error)

	// claims
	LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error)
	SendEthereumClaims(
		ctx context.Context,
		lastClaimEvent uint64,
		oldDeposits []*peggyevents.PeggySendToCosmosEvent,
		deposits []*peggyevents.PeggySendToInjectiveEvent,
		withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
		erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
		valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
	) error

	// batches
	UnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error)
	SendRequestBatch(ctx context.Context, denom string) error
	OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error)
	SendBatchConfirm(ctx context.Context, peggyID eth.Hash, batch *peggytypes.OutgoingTxBatch, ethFrom eth.Address) error
	LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error)
	TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract eth.Address) ([]*peggytypes.MsgConfirmBatch, error)

	// valsets
	OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	SendValsetConfirm(ctx context.Context, peggyID eth.Hash, valset *peggytypes.Valset, ethFrom eth.Address) error
	LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error)
	AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error)
	ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error)
}

type EthereumNetwork interface {
	FromAddress() eth.Address
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	GetPeggyID(ctx context.Context) (eth.Hash, error)

	// events
	GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToCosmosEvent, error)
	GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error)
	GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error)
	GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error)
	GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error)

	// valsets
	GetValsetNonce(ctx context.Context) (*big.Int, error)
	SendEthValsetUpdate(
		ctx context.Context,
		oldValset *peggytypes.Valset,
		newValset *peggytypes.Valset,
		confirms []*peggytypes.MsgValsetConfirm,
	) (*eth.Hash, error)

	// batches
	GetTxBatchNonce(
		ctx context.Context,
		erc20ContractAddress eth.Address,
	) (*big.Int, error)
	SendTransactionBatch(
		ctx context.Context,
		currentValset *peggytypes.Valset,
		batch *peggytypes.OutgoingTxBatch,
		confirms []*peggytypes.MsgConfirmBatch,
	) (*eth.Hash, error)
}

const defaultLoopDur = 60 * time.Second

type PeggyOrchestrator struct {
	logger  log.Logger
	svcTags metrics.Tags

	injective InjectiveNetwork
	ethereum  EthereumNetwork
	pricefeed PriceFeed

	erc20ContractMapping map[eth.Address]string
	relayValsetOffsetDur time.Duration
	relayBatchOffsetDur  time.Duration
	minBatchFeeUSD       float64
	maxAttempts          uint // max number of times a retry func will be called before exiting

	valsetRelayEnabled      bool
	batchRelayEnabled       bool
	periodicBatchRequesting bool
}

func NewPeggyOrchestrator(
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
	priceFeed PriceFeed,
	erc20ContractMapping map[eth.Address]string,
	minBatchFeeUSD float64,
	valsetRelayingEnabled,
	batchRelayingEnabled bool,
	valsetRelayingOffset,
	batchRelayingOffset string,
) (*PeggyOrchestrator, error) {
	orch := &PeggyOrchestrator{
		logger:               log.DefaultLogger,
		svcTags:              metrics.Tags{"svc": "peggy_orchestrator"},
		injective:            injective,
		ethereum:             ethereum,
		pricefeed:            priceFeed,
		erc20ContractMapping: erc20ContractMapping,
		minBatchFeeUSD:       minBatchFeeUSD,
		valsetRelayEnabled:   valsetRelayingEnabled,
		batchRelayEnabled:    batchRelayingEnabled,
		maxAttempts:          10, // default is 10 for retry pkg
	}

	if valsetRelayingEnabled {
		dur, err := time.ParseDuration(valsetRelayingOffset)
		if err != nil {
			return nil, errors.Wrapf(err, "valset relaying enabled but offset duration is not properly set")
		}

		orch.relayValsetOffsetDur = dur
	}

	if batchRelayingEnabled {
		dur, err := time.ParseDuration(batchRelayingOffset)
		if err != nil {
			return nil, errors.Wrapf(err, "batch relaying enabled but offset duration is not properly set")
		}

		orch.relayBatchOffsetDur = dur
	}

	return orch, nil
}

// Run starts all major loops required to make
// up the Orchestrator, all of these are async loops.
func (s *PeggyOrchestrator) Run(ctx context.Context, validatorMode bool) error {
	if !validatorMode {
		return s.startRelayerMode(ctx)
	}

	return s.startValidatorMode(ctx)
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *PeggyOrchestrator) startValidatorMode(ctx context.Context) error {
	log.WithFields(log.Fields{
		"BatchRequesterEnabled": true,
		"EthOracleEnabled":      true,
		"EthSignerEnabled":      true,
		"ValsetRelayerEnabled":  s.valsetRelayEnabled,
		"BatchRelayerEnabled":   s.batchRelayEnabled,
	}).Infoln("running in validator mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.EthOracleMainLoop(ctx) })
	pg.Go(func() error { return s.BatchRequesterLoop(ctx) })
	pg.Go(func() error { return s.EthSignerMainLoop(ctx) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx) })

	return pg.Wait()
}

// startRelayerMode runs orchestrator processes that only relay specific
// messages that do not require a validator's signature. This mode is run
// alongside a non-validator injective node
func (s *PeggyOrchestrator) startRelayerMode(ctx context.Context) error {
	log.WithFields(log.Fields{
		"BatchRequesterEnabled": true,
		"EthOracleEnabled":      false,
		"EthSignerEnabled":      false,
		"ValsetRelayerEnabled":  s.valsetRelayEnabled,
		"BatchRelayerEnabled":   s.batchRelayEnabled,
	}).Infoln("running in relayer mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.BatchRequesterLoop(ctx) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx) })

	return pg.Wait()
}

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
func (s *PeggyOrchestrator) EthOracleMainLoop(ctx context.Context) error {
	lastConfirmedEthHeight, err := s.getLastConfirmedEthHeightOnInjective(ctx)
	if err != nil {
		return err
	}

	s.logger.Infoln("scanning Ethereum events from block", lastConfirmedEthHeight)

	oracle := ethOracleLoop{
		PeggyOrchestrator:    s,
		loopDuration:         defaultLoopDur,
		lastCheckedEthHeight: lastConfirmedEthHeight,
	}

	return loops.RunLoop(ctx, defaultLoopDur, oracle.LoopFn(ctx, s.injective, s.ethereum))
}

func (s *PeggyOrchestrator) getLastConfirmedEthHeightOnInjective(ctx context.Context) (uint64, error) {
	var lastConfirmedEthHeight uint64
	getLastConfirmedEthHeightFn := func() error {
		lastClaimEvent, err := s.injective.LastClaimEvent(ctx)
		if err == nil && lastClaimEvent != nil && lastClaimEvent.EthereumEventHeight != 0 {
			lastConfirmedEthHeight = lastClaimEvent.EthereumEventHeight
			return nil

		}

		s.logger.WithError(err).Warningln("failed to get last claim from Injective. Querying peggy module params...")

		peggyParams, err := s.injective.PeggyParams(ctx)
		if err != nil {
			s.logger.WithError(err).Fatalln("failed to query peggy module params, is injectived running?")
			return err
		}

		lastConfirmedEthHeight = peggyParams.BridgeContractStartHeight
		return nil
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
