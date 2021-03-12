package orchestrator

import (
	"context"
	"math"
	"math/big"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/modules/peggy/types"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	"github.com/InjectiveLabs/peggo/orchestrator/relayer"

	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
)

const defaultLoopDur = 10 * time.Second

// Start combines the all major roles required to make
// up the Orchestrator, all of these are async loops.
func (s *peggyOrchestrator) Start(ctx context.Context) error {
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

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
//
// TODO this loop requires a method to bootstrap back to the correct event nonce when restarted
func (s *peggyOrchestrator) EthOracleMainLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "EthOracleMainLoop")

	var lastCheckedBlock uint64

	if err := retry.Do(func() (err error) {
		lastCheckedBlock, err = s.GetLastCheckedBlock(ctx)
		return
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.WithError(err).Warningf("failed to get last checked block, will retry (%d)", n)
	})); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	logger.Infoln("initial height sync complete")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		var latestCosmosBlock int64
		var latestEthBlock uint64

		var pg loops.ParanoidGroup

		pg.Go(func() error {
			return retry.Do(func() error {
				latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
				if err != nil {
					return err
				}

				latestEthBlock = latestHeader.Number.Uint64()
				return nil
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get latest header, will retry (%d)", n)
			}))
		})

		pg.Go(func() error {
			return retry.Do(func() (err error) {
				latestCosmosBlock, err = s.tmClient.GetLatestBlockHeight(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get latest header, will retry (%d)", n)
			}))
		})

		if err := pg.Wait(); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		logger.Debugf("latest Eth block %d, latest Cosmos block %d",
			latestEthBlock, latestCosmosBlock,
		)

		// Relays events from Ethereum -> Cosmos
		var currentBlock uint64
		if err := retry.Do(func() (err error) {
			currentBlock, err = s.CheckForEvents(ctx, lastCheckedBlock)
			return
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("error during Eth event checking, will retry (%d)", n)
		})); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		lastCheckedBlock = currentBlock
		return nil
	})
}

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Cosmsos node they can simply be assumed to be
// valid and signed off on.
func (s *peggyOrchestrator) EthSignerMainLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "EthSignerMainLoop")

	var peggyID common.Hash

	if err := retry.Do(func() (err error) {
		peggyID, err = s.peggyContract.GetPeggyID(ctx, s.peggyContract.FromAddress())
		return
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.WithError(err).Warningf("failed to get PeggyID from Ethereum contract, will retry (%d)", n)
	})); err != nil {
		logger.WithError(err).Errorln("got error, loop exits")
		return err
	}

	logger.Debugf("received peggyID %s", peggyID.Hex())

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		var latestCosmosBlock int64
		var latestEthBlock uint64

		var pg loops.ParanoidGroup

		pg.Go(func() error {
			return retry.Do(func() error {
				latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
				if err != nil {
					return err
				}

				latestEthBlock = latestHeader.Number.Uint64()
				return nil
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get latest header, will retry (%d)", n)
			}))
		})

		pg.Go(func() error {
			return retry.Do(func() (err error) {
				latestCosmosBlock, err = s.tmClient.GetLatestBlockHeight(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to get latest header, will retry (%d)", n)
			}))
		})

		if err := pg.Wait(); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		logger.Debugf("latest Eth block %d, latest Cosmos block %d",
			latestEthBlock, latestCosmosBlock,
		)

		var oldestUnsignedValsets []*types.Valset

		if err := retry.Do(func() error {
			oldestValsets, err := s.cosmosQueryClient.OldestUnsignedValsets(ctx, s.peggyBroadcastClient.ValFromAddress())
			if err != nil {
				if err == cosmos.ErrNotFound || oldestValsets == nil {
					logger.Debugln("no Valset waiting to be signed")
					return nil
				}

				return err
			}

			oldestUnsignedValsets = oldestValsets
			return nil
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to get unsigned Valset for signing, will retry (%d)", n)
		})); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		for _, oldestValset := range oldestUnsignedValsets {
			logger.Infoln("Sending Valset confirm for %d", oldestValset.Nonce)

			if err := retry.Do(func() error {
				return s.peggyBroadcastClient.SendValsetConfirm(ctx, s.ethFrom, peggyID, oldestValset)
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to sign and send Valset confirmation to Cosmos, will retry (%d)", n)
			})); err != nil {
				logger.WithError(err).Errorln("got error, loop exits")
				return err
			}
		}

		var oldestUnsignedTransactionBatch *types.OutgoingTxBatch

		if err := retry.Do(func() error {
			// sign the last unsigned batch, TODO check if we already have signed this

			txBatch, err := s.cosmosQueryClient.OldestUnsignedTransactionBatch(ctx, s.peggyBroadcastClient.ValFromAddress())
			if err != nil {
				if err == cosmos.ErrNotFound || txBatch == nil {
					logger.Debugln("no TransactionBatch waiting to be signed")
					return nil
				}

				return err
			}

			oldestUnsignedTransactionBatch = txBatch
			return nil
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.WithError(err).Warningf("failed to get unsigned TransactionBatch for signing, will retry (%d)", n)
		})); err != nil {
			logger.WithError(err).Errorln("got error, loop exits")
			return err
		}

		if oldestUnsignedTransactionBatch != nil {
			logger.Infoln("Sending TransactionBatch confirm for %d", oldestUnsignedTransactionBatch.BatchNonce)

			if err := retry.Do(func() error {
				return s.peggyBroadcastClient.SendBatchConfirm(ctx, s.ethFrom, peggyID, oldestUnsignedTransactionBatch)
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Warningf("failed to sign and send TransactionBatch confirmation to Cosmos, will retry (%d)", n)
			})); err != nil {
				logger.WithError(err).Errorln("got error, loop exits")
				return err
			}
		}

		return nil
	})
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

func (s *peggyOrchestrator) ValsetRequesterLoop(ctx context.Context) (err error) {
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

func (s *peggyOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	logger := log.WithField("loop", "BatchRequesterLoop")

	return loops.RunLoop(ctx, defaultLoopDur, func() error {
		// get All the denominations
		// check if threshold is met
		// broadcast Request batch

		var pg loops.ParanoidGroup

		pg.Go(func() error {

			var unbatchedTokensWithFees []*types.BatchFees

			if err := retry.Do(func() (err error) {
				unbatchedTokensWithFees, err = s.cosmosQueryClient.UnbatchedTokensWithFees(ctx)
				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.WithError(err).Errorf("failed to get UnbatchedTokensWithFees, will retry (%d)", n)
			})); err != nil {
				// non-fatal, just alert
				logger.Warningln("unable to get UnbatchedTokensWithFees for the token")
				return nil
			}

			for _, unbatchedToken := range unbatchedTokensWithFees {
				logger.Debugln("Sending batch request:", unbatchedToken.Token)
				return retry.Do(func() (err error) {
					// check if the token is present in cosmos denom. if so, send batch request with cosmosDenom
					tokenAddr := ethcmn.HexToAddress(unbatchedToken.Token)
					var denom string
					if cosmosDenom, ok := s.erc20ContractMapping[tokenAddr]; ok {
						// cosmos denom
						denom = cosmosDenom
					} else {
						// peggy denom
						denom = types.PeggyDenom(tokenAddr.Hex())
					}

					// send batch request only if fee is > 0. Add a threshold amount later through flags
					if unbatchedToken.TopOneHundred.GT(cosmtypes.NewInt(0)) {
						s.peggyBroadcastClient.SendRequestBatch(ctx, denom)
					}
					return
				}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
					logger.WithError(err).Errorf("failed to get LatestUnbatchOutgoingTx, will retry (%d)", n)
				}))
			}

			return nil
		})
		return pg.Wait()
	})
}

func (s *peggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	r := relayer.NewPeggyRelayer(s.cosmosQueryClient, s.peggyContract)
	return r.Start(ctx)
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
