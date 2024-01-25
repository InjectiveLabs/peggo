package orchestrator

import (
	"context"
	"time"

	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

const defaultLoopDur = 60 * time.Second

// PriceFeed provides token price for a given contract address
type PriceFeed interface {
	QueryUSDPrice(address gethcommon.Address) (float64, error)
}

type PeggyOrchestrator struct {
	logger  log.Logger
	svcTags metrics.Tags

	inj              cosmos.Network
	orchestratorAddr cosmostypes.AccAddress

	eth       EthereumNetwork
	pricefeed PriceFeed

	erc20ContractMapping map[gethcommon.Address]string
	relayValsetOffsetDur time.Duration
	relayBatchOffsetDur  time.Duration
	minBatchFeeUSD       float64
	maxAttempts          uint // max number of times a retry func will be called before exiting

	valsetRelayEnabled      bool
	batchRelayEnabled       bool
	periodicBatchRequesting bool
}

func NewPeggyOrchestrator(
	orchestratorAddr cosmostypes.AccAddress,
	injective cosmos.Network,
	ethereum EthereumNetwork,
	priceFeed PriceFeed,
	erc20ContractMapping map[gethcommon.Address]string,
	minBatchFeeUSD float64,
	valsetRelayingEnabled,
	batchRelayingEnabled bool,
	valsetRelayingOffset,
	batchRelayingOffset string,
) (*PeggyOrchestrator, error) {
	o := &PeggyOrchestrator{
		logger:               log.DefaultLogger,
		svcTags:              metrics.Tags{"svc": "peggy_orchestrator"},
		inj:                  injective,
		orchestratorAddr:     orchestratorAddr,
		eth:                  ethereum,
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

		o.relayValsetOffsetDur = dur
	}

	if batchRelayingEnabled {
		dur, err := time.ParseDuration(batchRelayingOffset)
		if err != nil {
			return nil, errors.Wrapf(err, "batch relaying enabled but offset duration is not properly set")
		}

		o.relayBatchOffsetDur = dur
	}

	return o, nil
}

// Run starts all major loops required to make
// up the Orchestrator, all of these are async loops.
func (s *PeggyOrchestrator) Run(ctx context.Context) error {
	if !s.hasDelegateValidator(ctx) {
		return s.startRelayerMode(ctx)
	}

	return s.startValidatorMode(ctx)
}

func (s *PeggyOrchestrator) hasDelegateValidator(ctx context.Context) bool {
	subCtx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	validator, err := s.inj.GetValidatorAddress(subCtx, s.eth.FromAddress())
	if err != nil {
		s.logger.WithError(err).Debugln("no delegate validator address found")
		return false
	}

	s.logger.WithField("addr", validator.String()).Debugln("found delegate validator")

	return true
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *PeggyOrchestrator) startValidatorMode(ctx context.Context) error {
	log.Infoln("running orchestrator in validator mode")

	// get gethcommon block observed by this validator
	lastObservedEthBlock, _ := s.getLastClaimBlockHeight(ctx)
	if lastObservedEthBlock == 0 {
		peggyParams, err := s.inj.PeggyParams(ctx)
		if err != nil {
			s.logger.WithError(err).Fatalln("unable to query peggy module params, is injectived running?")
		}

		lastObservedEthBlock = peggyParams.BridgeContractStartHeight
	}

	// get peggy ID from contract
	peggyContractID, err := s.eth.GetPeggyID(ctx)
	if err != nil {
		s.logger.WithError(err).Fatalln("unable to query peggy ID from contract")
	}

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.EthOracleMainLoop(ctx, lastObservedEthBlock) })
	pg.Go(func() error { return s.BatchRequesterLoop(ctx) })
	pg.Go(func() error { return s.EthSignerMainLoop(ctx, peggyContractID) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx) })

	return pg.Wait()
}

// startRelayerMode runs orchestrator processes that only relay specific
// messages that do not require a validator's signature. This mode is run
// alongside a non-validator injective node
func (s *PeggyOrchestrator) startRelayerMode(ctx context.Context) error {
	log.Infoln("running orchestrator in relayer mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.BatchRequesterLoop(ctx) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx) })

	return pg.Wait()
}

func (s *PeggyOrchestrator) getLastClaimBlockHeight(ctx context.Context) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	claim, err := s.inj.LastClaimEventByAddr(ctx, s.orchestratorAddr)
	if err != nil {
		return 0, err
	}

	return claim.EthereumEventHeight, nil
}
