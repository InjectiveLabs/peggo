package orchestrator

import (
	"context"
	"time"

	eth "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

const defaultLoopDur = 60 * time.Second

type PeggyOrchestrator struct {
	logger  log.Logger
	svcTags metrics.Tags

	inj       InjectiveNetwork
	eth       EthereumNetwork
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
	o := &PeggyOrchestrator{
		logger:               log.DefaultLogger,
		svcTags:              metrics.Tags{"svc": "peggy_orchestrator"},
		inj:                  injective,
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
	if !s.hasRegisteredETHAddress(ctx) {
		return s.startRelayerMode(ctx)
	}

	return s.startValidatorMode(ctx)
}

func (s *PeggyOrchestrator) hasRegisteredETHAddress(ctx context.Context) bool {
	subCtx, cancelFn := context.WithTimeout(ctx, 5*time.Second)
	defer cancelFn()

	ok, _ := s.inj.HasRegisteredEthAddress(subCtx, s.eth.FromAddress())
	return ok
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *PeggyOrchestrator) startValidatorMode(ctx context.Context) error {
	log.WithFields(log.Fields{
		"batch_requesting":   true,
		"eth_event_tracking": true,
		"batch_signing":      true,
		"valset_signing":     true,
		"valset_relaying":    s.valsetRelayEnabled,
		"batch_relaying":     s.batchRelayEnabled,
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
		"batch_requesting": true,
		"valset_relaying":  s.valsetRelayEnabled,
		"batch_relaying":   s.batchRelayEnabled,
	}).Infoln("running in relayer mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.BatchRequesterLoop(ctx) })
	pg.Go(func() error { return s.RelayerMainLoop(ctx) })

	return pg.Wait()
}
