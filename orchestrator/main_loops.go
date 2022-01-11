package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/avast/retry-go"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/umee-network/peggo/orchestrator/loops"
)

const (
	// We define loops durations based on multipliers of average block times from Eth and Cosmos.
	//
	// Ref: https://github.com/umee-network/peggo/issues/55

	// Run every approximately 5 Ethereum blocks to allow time to receive new blocks.
	// If we run this faster we wouldn't be getting new blocks, which is not efficient.
	ethOracleLoopMultiplier = 5

	// Run every approximately 3 Cosmos blocks; so we sign batches and valset updates ASAP but not run these requests
	// too often that we make too many requests to Cosmos.
	ethSignerLoopMultiplier = 3
)

// Start combines the all major roles required to make
// up the Orchestrator, all of these are async loops.
func (p *gravityOrchestrator) Start(ctx context.Context) error {
	var pg loops.ParanoidGroup

	pg.Go(func() error {
		return p.EthOracleMainLoop(ctx)
	})
	pg.Go(func() error {
		return p.BatchRequesterLoop(ctx)
	})
	pg.Go(func() error {
		return p.EthSignerMainLoop(ctx)
	})
	pg.Go(func() error {
		return p.RelayerMainLoop(ctx)
	})

	return pg.Wait()
}

// EthOracleMainLoop is responsible for making sure that Ethereum events are retrieved from the Ethereum blockchain
// and ferried over to Cosmos where they will be used to issue tokens or process batches.
func (p *gravityOrchestrator) EthOracleMainLoop(ctx context.Context) (err error) {
	logger := p.logger.With().Str("loop", "EthOracleMainLoop").Logger()
	lastResync := time.Now()

	var (
		lastCheckedBlock uint64
		gravityParams    types.Params
	)

	if err := retry.Do(func() (err error) {
		gravityParamsResp, err := p.cosmosQueryClient.Params(ctx, &types.QueryParamsRequest{})
		if err != nil || gravityParamsResp == nil {
			logger.Fatal().Err(err).Msg("failed to query Gravity params, is umeed running?")
		}

		gravityParams = gravityParamsResp.Params

		return err
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.Err(err).Uint("retry", n).Msg("failed to get Gravity params; retrying...")
	})); err != nil {
		logger.Err(err).Msg("got error, loop exits")
		return err
	}

	// Wait until the contract is available
	if p.bridgeStartHeight != 0 {
		for {
			latestHeader, err := p.ethProvider.HeaderByNumber(ctx, nil)
			if err != nil {
				logger.Err(err).Msg("failed to get latest header, loop exits")
				return err
			}

			currentBlock := latestHeader.Number.Uint64() - getEthBlockDelay(gravityParams.BridgeChainId)

			if currentBlock < p.bridgeStartHeight {
				wait := p.ethereumBlockTime * time.Duration(p.bridgeStartHeight-currentBlock)
				logger.Error().
					Uint64("current_block", currentBlock).
					Uint64("start_height", p.bridgeStartHeight).
					Uint64("blocks_left", p.bridgeStartHeight-currentBlock).
					Dur("wait_time", wait).
					Msg("waiting for contract to be available")
				time.Sleep(wait)
				continue
			}

			logger.Info().Msg("contract is available; oracle loop starts")
			break
		}
	}

	if err := retry.Do(func() (err error) {
		lastCheckedBlock, err = p.GetLastCheckedBlock(ctx, getEthBlockDelay(gravityParams.BridgeChainId))
		return err
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.Err(err).Uint("retry", n).Msg("failed to get last checked block; retrying...")
	})); err != nil {
		logger.Err(err).Msg("got error, loop exits")
		return err
	}

	logger.Info().Uint64("last_checked_block", lastCheckedBlock).Msg("start scanning for events")

	return loops.RunLoop(ctx, p.logger, p.ethereumBlockTime*ethOracleLoopMultiplier, func() error {
		// Relays events from Ethereum -> Cosmos
		var currentBlock uint64
		if err := retry.Do(func() (err error) {
			currentBlock, err = p.CheckForEvents(ctx, lastCheckedBlock, getEthBlockDelay(gravityParams.BridgeChainId))
			return err
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.Err(err).Uint("retry", n).Msg("error during Eth event checking; retrying...")
		})); err != nil {
			logger.Err(err).Msg("got error, loop exits")
			return err
		}

		lastCheckedBlock = currentBlock

		// Auto re-sync to catch up the nonce. Reasons why event nonce fall behind.
		//	1. It takes some time for events to be indexed on Ethereum. So if peggo queried events immediately as
		//	   block produced, there is a chance the event is missed. We need to re-scan this block to ensure events
		//	   are not missed due to indexing delay.
		//	2. If validator was in UnBonding state, the claims broadcasted in last iteration are failed.
		//	3. If the ETH call failed while filtering events, the peggo missed to broadcast claim events occurred in
		//	   last iteration.
		if time.Since(lastResync) >= 48*time.Hour {
			if err := retry.Do(func() (err error) {
				lastCheckedBlock, err = p.GetLastCheckedBlock(ctx, getEthBlockDelay(gravityParams.BridgeChainId))
				return err
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.Err(err).Uint("retry", n).Msg("failed to get last checked block; retrying...")
			})); err != nil {
				logger.Err(err).Msg("got error, loop exits")
				return err
			}

			lastResync = time.Now()
			logger.Info().
				Time("last_resync", lastResync).
				Uint64("last_checked_block", lastCheckedBlock).
				Msg("auto resync")
		}

		return nil
	})
}

// EthSignerMainLoop simply signs off on any batches or validator sets provided by the validator
// since these are provided directly by a trusted Cosmsos node they can simply be assumed to be
// valid and signed off on.
func (p *gravityOrchestrator) EthSignerMainLoop(ctx context.Context) (err error) {
	logger := p.logger.With().Str("loop", "EthSignerMainLoop").Logger()

	var gravityID string
	if err := retry.Do(func() (err error) {
		gravityID, err = p.gravityContract.GetGravityID(ctx, p.gravityContract.FromAddress())
		return err
	}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
		logger.Err(err).Uint("retry", n).Msg("failed to get GravityID from Ethereum contract; retrying...")
	})); err != nil {
		logger.Err(err).Msg("got error, loop exits")
		return err
	}

	logger.Debug().Str("gravityID", gravityID).Msg("received gravityID")

	return loops.RunLoop(ctx, p.logger, p.cosmosBlockTime*ethSignerLoopMultiplier, func() error {
		var oldestUnsignedValsets []types.Valset
		if err := retry.Do(func() error {
			oldestValsets, err := p.cosmosQueryClient.LastPendingValsetRequestByAddr(
				ctx,
				&types.QueryLastPendingValsetRequestByAddrRequest{
					Address: p.gravityBroadcastClient.AccFromAddress().String(),
				},
			)

			if err != nil {
				return err
			}

			if oldestValsets == nil || oldestValsets.Valsets == nil {
				logger.Debug().Msg("no Valset waiting to be signed")
				return nil
			}

			oldestUnsignedValsets = oldestValsets.Valsets
			return nil
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.Err(err).Uint("retry", n).Msg("failed to get unsigned Valset for signing; retrying...")
		})); err != nil {
			logger.Err(err).Msg("got error, loop exits")
			return err
		}

		for _, oldestValset := range oldestUnsignedValsets {
			logger.Info().Uint64("oldest_valset_nonce", oldestValset.Nonce).Msg("sending Valset confirm for nonce")
			valset := oldestValset

			if err := retry.Do(func() error {
				return p.gravityBroadcastClient.SendValsetConfirm(ctx, p.ethFrom, gravityID, valset)
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.Err(err).
					Uint("retry", n).
					Msg("failed to sign and send Valset confirmation to Cosmos; retrying...")
			})); err != nil {
				logger.Err(err).Msg("got error, loop exits")
				return err
			}
		}

		var oldestUnsignedTransactionBatch []types.OutgoingTxBatch
		if err := retry.Do(func() error {
			// sign the last unsigned batch, TODO check if we already have signed this
			txBatch, err := p.cosmosQueryClient.LastPendingBatchRequestByAddr(
				ctx,
				&types.QueryLastPendingBatchRequestByAddrRequest{
					Address: p.gravityBroadcastClient.AccFromAddress().String(),
				},
			)

			if err != nil {
				return err
			}

			if txBatch == nil || txBatch.Batch == nil {
				logger.Debug().Msg("no TransactionBatch waiting to be signed")
				return nil
			}

			oldestUnsignedTransactionBatch = txBatch.Batch
			return nil
		}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
			logger.Err(err).
				Uint("retry", n).
				Msg("failed to get unsigned TransactionBatch for signing; retrying...")
		})); err != nil {
			logger.Err(err).Msg("got error, loop exits")
			return err
		}

		for _, batch := range oldestUnsignedTransactionBatch {
			batch := batch
			logger.Info().
				Uint64("batch_nonce", batch.BatchNonce).
				Msg("sending TransactionBatch confirm for BatchNonce")
			if err := retry.Do(func() error {
				return p.gravityBroadcastClient.SendBatchConfirm(ctx, p.ethFrom, gravityID, batch)
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.Err(err).
					Uint("retry", n).
					Msg("failed to sign and send TransactionBatch confirmation to Cosmos; retrying...")
			})); err != nil {
				logger.Err(err).Msg("got error, loop exits")
				return err
			}
		}

		return nil
	})
}

func (p *gravityOrchestrator) BatchRequesterLoop(ctx context.Context) (err error) {
	logger := p.logger.With().Str("loop", "BatchRequesterLoop").Logger()

	return loops.RunLoop(ctx, p.logger, p.batchRequesterLoopDuration, func() error {
		// Each loop performs the following:
		//
		// - get All the denominations
		// - broadcast Request batch
		var pg loops.ParanoidGroup

		pg.Go(func() error {
			var unbatchedTokensWithFees []types.BatchFees

			if err := retry.Do(func() (err error) {
				batchFeesResp, err := p.cosmosQueryClient.BatchFees(ctx, &types.QueryBatchFeeRequest{})

				if err != nil {
					return err
				}

				unbatchedTokensWithFees = batchFeesResp.GetBatchFees()

				return
			}, retry.Context(ctx), retry.OnRetry(func(n uint, err error) {
				logger.Err(err).Uint("retry", n).Msg("failed to get UnbatchedTokensWithFees; retrying...")
			})); err != nil {
				// non-fatal, just alert
				logger.Warn().Msg("unable to get UnbatchedTokensWithFees for the token")
				return nil
			}

			for _, unbatchedToken := range unbatchedTokensWithFees {
				unbatchedToken := unbatchedToken
				tokenAddr := ethcmn.HexToAddress(unbatchedToken.Token)

				denom, err := p.ERC20ToDenom(ctx, tokenAddr)
				if err != nil {
					// do not return error, just continue with the next unbatched tx
					logger.Err(err).Str("token_contract", tokenAddr.String()).Msg("failed to get denom; will not request a batch")
					return nil
				}

				logger.Info().Str("token_contract", tokenAddr.String()).Str("denom", denom).Msg("sending batch request")

				if err := p.gravityBroadcastClient.SendRequestBatch(ctx, denom); err != nil {
					logger.Err(err).Msg("failed to send batch request")
				}
			}

			return nil
		})

		return pg.Wait()
	})
}

func (p *gravityOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	if p.relayer != nil {
		return p.relayer.Start(ctx)
	}

	return errors.New("relayer is nil")
}

// ERC20ToDenom attempts to return the denomination that maps to an ERC20 token
// contract on the Cosmos chain. First, we check the cache. If the token address
// does not exist in the cache, we query the Cosmos chain and cache the result.
func (p *gravityOrchestrator) ERC20ToDenom(ctx context.Context, tokenAddr ethcmn.Address) (string, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	tokenAddrStr := tokenAddr.String()
	denom, ok := p.erc20DenomCache[tokenAddrStr]
	if ok {
		return denom, nil
	}

	resp, err := p.cosmosQueryClient.ERC20ToDenom(ctx, &types.QueryERC20ToDenomRequest{Erc20: tokenAddrStr})
	if err != nil {
		return "", err
	}

	if resp == nil {
		return "", errors.New("no denom found for token")
	}

	if p.erc20DenomCache == nil {
		p.erc20DenomCache = map[string]string{}
	}

	p.erc20DenomCache[tokenAddrStr] = resp.Denom
	return resp.Denom, nil
}

// getEthBlockDelay returns the right amount of Ethereum blocks to wait until we
// consider a block final. This depends on the chain we are talking to.
// Copying from https://github.com/Gravity-Bridge/Gravity-Bridge/blob/main/orchestrator/orchestrator/src/ethereum_event_watcher.rs#L248
// DO NOT MODIFY. Changing any of these values puts the network in DANGER and
// does not provide any advantage over other validators. This is a safety
// mechanism to prevent relaying an event that is not yet considered final.
func getEthBlockDelay(chainID uint64) uint64 {
	switch chainID {
	// Mainline Ethereum, Ethereum classic, or the Ropsten, Kotti, Mordor testnets
	// all POW Chains
	case 1, 3, 6, 7:
		return 13

	// Dev, our own Gravity Ethereum testnet, and Hardhat respectively
	// all single signer chains with no chance of any reorgs
	case 2018, 15, 31337:
		return 0

	// Rinkeby and Goerli use Clique (POA) Consensus, finality takes
	// up to num validators blocks. Number is higher than Ethereum based
	// on experience with operational issues
	case 4, 5:
		return 10

	// assume the safe option (POW) where we don't know
	default:
		return 13
	}
}
