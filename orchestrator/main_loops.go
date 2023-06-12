package orchestrator

import (
	"context"
	"errors"
	"math"
	"math/big"
	"time"

	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"

	"github.com/InjectiveLabs/peggo/orchestrator/loops"
)

const defaultLoopDur = 60 * time.Second

// Start combines the all major roles required to make
// up the Orchestrator, all of these are async loops.
func (s *PeggyOrchestrator) Start(ctx context.Context, validatorMode bool) error {
	if !validatorMode {
		log.Infoln("Starting peggo in relayer (non-validator) mode")
		return s.startRelayerMode(ctx)
	}

	log.Infoln("Starting peggo in validator mode")
	return s.startValidatorMode(ctx)
}

// This loop doesn't have a formal role per say, anyone can request a valset
// but there does need to be some strategy to ensure requests are made. Having it
// be a function of the orchestrator makes a lot of sense as they are already online
// and have all the required funds, keys, and rpc servers setup
//
// Exactly how to balance optimizing this versus testing is an interesting discussion
// in testing we want to make sure requests are made without any powers changing on the chain
// just to simplify the test environment. But in production that's somewhat wasteful. What this
// routine does it check the current valset versus the last requested valset, if power has changed
// significantly we send in a request.

/*
Not required any more. The valset request are generated in endblocker of peggy module automatically. Also MsgSendValsetRequest is removed on peggy module.

func (s *PeggyOrchestrator) ValsetRequesterLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "ValsetRequesterLoop")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		var latestValsets []*types.Valset
		var currentValset *types.Valset

		var pg loops.ParanoidGroup

		pg.Go(func() error {
			return retry.Do(func() (err error) {
				latestValsets, err = s.cosmosQueryClient.LatestValsets(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get latest valsets, will retry (%d)", n)
			}))
		})

		pg.Go(func() error {
			return retry.Do(func() (err error) {
				currentValset, err = s.cosmosQueryClient.CurrentValset(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get current valset, will retry (%d)", n)
			}))
		})

		if err := pg.Wait(); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		if len(latestValsets) == 0 {
			retry.Do(func() error {
				return s.peggyBroadcastClient.SendValsetRequest(ctx)
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to request Valset to be formed, will retry (%d)", n)
			}))
		} else {
			// if the power difference is more than 1% different than the last valset
			if valPowerDiff(latestValsets[0], currentValset) > 0.01 {
				log.Debugln("power difference is more than 1%% different than the last valset. Sending valset request")

				retry.Do(func() error {
					return s.peggyBroadcastClient.SendValsetRequest(ctx)
				}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
					logger.WithError(err).Warningf("failed to request Valset to be formed, will retry (%d)", n)
				}))
			}
		}

		return nil
	})
}
**/

func (s *PeggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	if s.relayer != nil {
		return s.relayer.Start(ctx)
	} else {
		return errors.New("relayer is nil")
	}
}

// valPowerDiff returns the difference in power between two bridge validator sets
// TODO: this needs to be potentially refactored
func valPowerDiff(old *types.Valset, new *types.Valset) float64 {
	powers := map[string]int64{}
	var totalB int64
	// loop over b and initialize the map with their powers
	for _, bv := range old.GetMembers() {
		powers[bv.EthereumAddress] = int64(bv.Power)
		totalB += int64(bv.Power)
	}

	// subtract c powers from powers in the map, initializing
	// uninitialized keys with negative numbers
	for _, bv := range new.GetMembers() {
		if val, ok := powers[bv.EthereumAddress]; ok {
			powers[bv.EthereumAddress] = val - int64(bv.Power)
		} else {
			powers[bv.EthereumAddress] = -int64(bv.Power)
		}
	}

	var delta float64
	for _, v := range powers {
		// NOTE: we care about the absolute value of the changes
		delta += math.Abs(float64(v))
	}

	return math.Abs(delta / float64(totalB))
}

func calculateTotalValsetPower(valset *types.Valset) *big.Int {
	totalValsetPower := new(big.Int)
	for _, m := range valset.Members {
		mPower := big.NewInt(0).SetUint64(m.Power)
		totalValsetPower.Add(totalValsetPower, mPower)
	}

	return totalValsetPower
}

// startValidatorMode runs all orchestrator processes. This is called
// when peggo is run alongside a validator injective node.
func (s *PeggyOrchestrator) startValidatorMode(ctx context.Context) error {
	var pg loops.ParanoidGroup

	pg.Go(func() error {
		return s.EthOracleMainLoop(ctx)
	})
	pg.Go(func() error {
		return s.BatchRequesterLoop(ctx)
	})
	pg.Go(func() error {
		return s.EthSignerMainLoop(ctx)
	})
	pg.Go(func() error {
		return s.RelayerMainLoop(ctx)
	})

	return pg.Wait()
}

// startRelayerMode runs orchestrator processes that only relay specific
// messages that do not require a validator's signature. This mode is run
// alongside a non-validator injective node
func (s *PeggyOrchestrator) startRelayerMode(ctx context.Context) error {
	var pg loops.ParanoidGroup

	pg.Go(func() error {
		return s.BatchRequesterLoop(ctx)
	})

	pg.Go(func() error {
		return s.RelayerMainLoop(ctx)
	})

	return pg.Wait()
}
