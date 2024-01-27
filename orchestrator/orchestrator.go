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

type Config struct {
	MinBatchFeeUSD       float64
	ERC20ContractMapping map[gethcommon.Address]string
	RelayValsetOffsetDur string
	RelayBatchOffsetDur  string
	RelayValsets         bool
	RelayBatches         bool
}

type PeggyOrchestrator struct {
	logger  log.Logger
	svcTags metrics.Tags

	injAddr cosmostypes.AccAddress
	ethAddr gethcommon.Address

	priceFeed            PriceFeed
	erc20ContractMapping map[gethcommon.Address]string
	relayValsetOffsetDur time.Duration
	relayBatchOffsetDur  time.Duration
	minBatchFeeUSD       float64
	maxAttempts          uint // max number of times a retry func will be called before exiting
}

func NewPeggyOrchestrator(
	orchestratorAddr cosmostypes.AccAddress,
	ethAddr gethcommon.Address,
	priceFeed PriceFeed,
	cfg Config,
) (*PeggyOrchestrator, error) {
	o := &PeggyOrchestrator{
		logger:               log.DefaultLogger,
		svcTags:              metrics.Tags{"svc": "peggy_orchestrator"},
		injAddr:              orchestratorAddr,
		ethAddr:              ethAddr,
		priceFeed:            priceFeed,
		erc20ContractMapping: cfg.ERC20ContractMapping,
		minBatchFeeUSD:       cfg.MinBatchFeeUSD,
		maxAttempts:          10, // default for retry pkg
	}

	if cfg.RelayValsets {
		dur, err := time.ParseDuration(cfg.RelayValsetOffsetDur)
		if err != nil {
			return nil, errors.Wrapf(err, "valset relaying enabled but offset duration is not properly set")
		}

		o.relayValsetOffsetDur = dur
	}

	if cfg.RelayBatches {
		dur, err := time.ParseDuration(cfg.RelayBatchOffsetDur)
		if err != nil {
			return nil, errors.Wrapf(err, "batch relaying enabled but offset duration is not properly set")
		}

		o.relayBatchOffsetDur = dur
	}

	return o, nil
}

// Run starts all major loops required to make
// up the Orchestrator, all of these are async loops.
func (s *PeggyOrchestrator) Run(ctx context.Context, inj cosmos.Network, eth EthereumNetwork) error {
	if !s.hasDelegateValidator(ctx, inj) {
		return s.startRelayerMode(ctx, inj, eth)
	}

	return s.startValidatorMode(ctx, inj, eth)
}

func (s *PeggyOrchestrator) hasDelegateValidator(ctx context.Context, inj cosmos.Network) bool {
	subCtx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	validator, err := inj.GetValidatorAddress(subCtx, s.ethAddr)
	if err != nil {
		s.logger.WithError(err).Debugln("no delegate validator address found")
		return false
	}

	s.logger.WithField("addr", validator.String()).Debugln("found delegate validator")

	return true
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *PeggyOrchestrator) startValidatorMode(ctx context.Context, inj cosmos.Network, eth EthereumNetwork) error {
	log.Infoln("running orchestrator in validator mode")

	// get gethcommon block observed by this validator
	lastObservedEthBlock, _ := s.getLastClaimBlockHeight(ctx, inj)
	if lastObservedEthBlock == 0 {
		peggyParams, err := inj.PeggyParams(ctx)
		if err != nil {
			s.logger.WithError(err).Fatalln("unable to query peggy module params, is injectived running?")
		}

		lastObservedEthBlock = peggyParams.BridgeContractStartHeight
	}

	// get peggy ID from contract
	peggyContractID, err := eth.GetPeggyID(ctx)
	if err != nil {
		s.logger.WithError(err).Fatalln("unable to query peggy ID from contract")
	}

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.EthOracleMainLoop(ctx, inj, eth, lastObservedEthBlock) })
	pg.Go(func() error { return s.BatchRequesterLoop(ctx, inj) })
	pg.Go(func() error { return s.EthSignerMainLoop(ctx, inj, peggyContractID) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx, inj, eth) })

	return pg.Wait()
}

// startRelayerMode runs orchestrator processes that only relay specific
// messages that do not require a validator's signature. This mode is run
// alongside a non-validator injective node
func (s *PeggyOrchestrator) startRelayerMode(ctx context.Context, inj cosmos.Network, eth EthereumNetwork) error {
	log.Infoln("running orchestrator in relayer mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.BatchRequesterLoop(ctx, inj) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx, inj, eth) })

	return pg.Wait()
}

func (s *PeggyOrchestrator) getLastClaimBlockHeight(ctx context.Context, inj cosmos.Network) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	claim, err := inj.LastClaimEventByAddr(ctx, s.injAddr)
	if err != nil {
		return 0, err
	}

	return claim.EthereumEventHeight, nil
}
