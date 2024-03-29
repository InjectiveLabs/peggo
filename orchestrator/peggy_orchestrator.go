package orchestrator

import (
	"context"
	"time"

	"github.com/avast/retry-go"
	cosmostypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/metrics"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

const (
	defaultLoopDur = 60 * time.Second
)

var (
	maxRetryAttempts uint = 10
)

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
	RelayerMode          bool
}

type Orchestrator struct {
	logger  log.Logger
	svcTags metrics.Tags

	injAddr cosmostypes.AccAddress
	ethAddr gethcommon.Address

	priceFeed            PriceFeed
	erc20ContractMapping map[gethcommon.Address]string
	relayValsetOffsetDur time.Duration
	relayBatchOffsetDur  time.Duration
	minBatchFeeUSD       float64
	isRelayer            bool
}

func NewPeggyOrchestrator(
	orchestratorAddr cosmostypes.AccAddress,
	ethAddr gethcommon.Address,
	priceFeed PriceFeed,
	cfg Config,
) (*Orchestrator, error) {
	o := &Orchestrator{
		logger:               log.DefaultLogger,
		svcTags:              metrics.Tags{"svc": "peggy_orchestrator"},
		injAddr:              orchestratorAddr,
		ethAddr:              ethAddr,
		priceFeed:            priceFeed,
		erc20ContractMapping: cfg.ERC20ContractMapping,
		minBatchFeeUSD:       cfg.MinBatchFeeUSD,
		isRelayer:            cfg.RelayerMode,
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
func (s *Orchestrator) Run(ctx context.Context, inj cosmos.Network, eth ethereum.Network) error {
	if s.isRelayer {
		return s.startRelayerMode(ctx, inj, eth)
	}

	return s.startValidatorMode(ctx, inj, eth)
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *Orchestrator) startValidatorMode(ctx context.Context, inj cosmos.Network, eth ethereum.Network) error {
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

	pg.Go(func() error { return s.runEthOracle(ctx, inj, eth, lastObservedEthBlock) })
	pg.Go(func() error { return s.runEthSigner(ctx, inj, peggyContractID) })
	pg.Go(func() error { return s.runBatchRequester(ctx, inj, eth) })
	pg.Go(func() error { return s.runRelayer(ctx, inj, eth) })

	return pg.Wait()
}

// startRelayerMode runs orchestrator processes that only relay specific
// messages that do not require a validator's signature. This mode is run
// alongside a non-validator injective node
func (s *Orchestrator) startRelayerMode(ctx context.Context, inj cosmos.Network, eth ethereum.Network) error {
	log.Infoln("running orchestrator in relayer mode")

	var pg loops.ParanoidGroup

	pg.Go(func() error { return s.runBatchRequester(ctx, inj, eth) })
	pg.Go(func() error { return s.runRelayer(ctx, inj, eth) })

	return pg.Wait()
}

func (s *Orchestrator) getLastClaimBlockHeight(ctx context.Context, inj cosmos.Network) (uint64, error) {
	metrics.ReportFuncCall(s.svcTags)
	doneFn := metrics.ReportFuncTiming(s.svcTags)
	defer doneFn()

	claim, err := inj.LastClaimEventByAddr(ctx, s.injAddr)
	if err != nil {
		return 0, err
	}

	return claim.EthereumEventHeight, nil
}

func retryFnOnErr(ctx context.Context, log log.Logger, fn func() error) error {
	return retry.Do(fn,
		retry.Context(ctx),
		retry.Attempts(maxRetryAttempts),
		retry.OnRetry(func(n uint, err error) {
			log.WithError(err).Warningf("encountered error, retrying (%d)", n)
		}),
	)
}
