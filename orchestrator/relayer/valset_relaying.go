package relayer

import (
	"context"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/pkg/errors"
)

// RelayValsets checks the last validator set on Ethereum, if it's lower than our latest validator
// set then we should package and submit the update as an Ethereum transaction
func (s *gravityRelayer) RelayValsets(ctx context.Context, currentValset types.Valset) error {
	// we should determine if we need to relay one
	// to Ethereum for that we will find the latest confirmed valset and compare it to the ethereum chain
	latestValsets, err := s.cosmosQueryClient.LastValsetRequests(ctx, &types.QueryLastValsetRequestsRequest{})
	if err != nil {
		err = errors.Wrap(err, "failed to fetch latest valsets from cosmos")
		return err
	} else if latestValsets == nil {
		return errors.New("no valsets found")
	}

	var latestCosmosSigs []types.MsgValsetConfirm
	var latestCosmosConfirmed *types.Valset
	for i, set := range latestValsets.Valsets {
		sigs, err := s.cosmosQueryClient.ValsetConfirmsByNonce(ctx, &types.QueryValsetConfirmsByNonceRequest{
			Nonce: set.Nonce,
		})

		if err != nil {
			err = errors.Wrapf(err, "failed to get valset confirms at nonce %d", set.Nonce)
			return err
		}
		if sigs == nil {
			return errors.New("no valset confirms found")
		}

		if len(sigs.Confirms) == 0 {
			continue
		}

		latestCosmosSigs = sigs.Confirms
		latestCosmosConfirmed = &latestValsets.Valsets[i]
		break
	}

	if latestCosmosConfirmed == nil {
		s.logger.Debug().Msg("no confirmed valsets found, nothing to relay")
		return nil
	}

	s.logger.Debug().
		Uint64("current_eth_valset_nonce", currentValset.Nonce).
		Uint64("latest_cosmos_confirmed_nonce", latestCosmosConfirmed.Nonce).
		Msg("found latest valsets")

	if s.lastSentValsetNonce >= latestCosmosConfirmed.Nonce {
		s.logger.Debug().Msg("already relayed this valset; skipping")
		return nil
	}

	if latestCosmosConfirmed.Nonce > currentValset.Nonce {
		latestEthereumValsetNonce, err := s.gravityContract.GetValsetNonce(ctx, s.gravityContract.FromAddress())
		if err != nil {
			err = errors.Wrap(err, "failed to get latest Valset nonce")
			return err
		}

		// Check if latestCosmosConfirmed already submitted by other validators in mean time
		if latestCosmosConfirmed.Nonce > latestEthereumValsetNonce.Uint64() {
			s.logger.Info().
				Uint64("latest_cosmos_confirmed_nonce", latestCosmosConfirmed.Nonce).
				Uint64("latest_ethereum_valset_nonce", latestEthereumValsetNonce.Uint64()).
				Msg("detected latest cosmos valset nonce, but latest valset on Ethereum is different. Sending update to Ethereum")

			txData, err := s.gravityContract.EncodeValsetUpdate(
				ctx,
				currentValset,
				*latestCosmosConfirmed,
				latestCosmosSigs,
			)
			if err != nil {
				return err
			}

			estimatedGasCost, gasPrice, err := s.gravityContract.EstimateGas(ctx, s.gravityContract.Address(), txData)
			if err != nil {
				s.logger.Err(err).Msg("failed to estimate gas cost")
				return err
			}

			// TODO: Estimate profitability using "valset reward" param.
			//
			// Ref: https://github.com/umee-network/peggo/issues/56

			// Checking in pending txs (mempool) if tx with same input is already submitted.
			// We have to check this at the very last moment because any other relayer could have submitted.
			if s.gravityContract.IsPendingTxInput(txData, s.pendingTxWait) {
				s.logger.Error().
					Msg("Transaction with same valset input data is already present in mempool")
				return nil
			}

			// Send Valset Update to Ethereum
			txHash, err := s.gravityContract.SendTx(ctx, s.gravityContract.Address(), txData, estimatedGasCost, gasPrice)
			if err != nil {
				s.logger.Err(err).
					Str("tx_hash", txHash.Hex()).
					Msg("failed to sign and submit (Gravity updateValset) to EVM")
				return err
			}

			s.logger.Info().Str("tx_hash", txHash.Hex()).Msg("sent Tx (Gravity updateValset)")

			// update our local tracker of the latest valset
			s.lastSentValsetNonce = latestCosmosConfirmed.Nonce
		}

	}

	return nil
}
