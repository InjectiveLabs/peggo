package relayer

import (
	"context"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/pkg/errors"
)

// RelayValsets checks the last validator set on Ethereum, if it's lower than our latest validator
// set then we should package and submit the update as an Ethereum transaction
func (s *gravityRelayer) RelayValsets(ctx context.Context, currentValset types.Valset) error {
	// This works by checking if the latest valset update can be sent by the current valset stored on Ethereum.
	// If so, there's no need to relay anything, given that the current valset on Eth shares enough signers with the
	// latest valset on Cosmos.
	// If the latest valset on Cosmos (latestValsets.Valsets[0]) can't be sent by the current valset on Ethereum,
	// then we need to relay the latest valid valset on Cosmos; that means the one that shares enough signers with
	// the valset stored on Ethereum.

	latestValsets, err := s.cosmosQueryClient.LastValsetRequests(ctx, &types.QueryLastValsetRequestsRequest{})
	if err != nil {
		err = errors.Wrap(err, "failed to fetch latest valsets from cosmos")
		return err
	} else if latestValsets == nil {
		return errors.New("no valsets found")
	}

	// latestValidValset means the latest valset that can be sent using the current valset on Ethereum.
	latestValidValset, latestValidValsetSigs, err := s.findLatestValidValset(
		ctx,
		currentValset,
		latestValsets.Valsets[0].Nonce,
	)

	if err != nil {
		// findLatestValidValset returns an error only in cases we would like to retry.
		return err
	}

	if latestValidValset == nil && err == nil {
		s.logger.Info().Msg("no valset updates to relay")
		return nil
	}

	if s.lastSentValsetNonce >= latestValidValset.Nonce {
		s.logger.Debug().Msg("already relayed this valset; skipping")
		return nil
	}

	s.logger.Debug().
		Uint64("current_eth_valset_nonce", currentValset.Nonce).
		Uint64("latest_cosmos_confirmed_nonce", latestValidValset.Nonce).
		Msg("found latest valsets")

	latestEthereumValsetNonce, err := s.gravityContract.GetValsetNonce(ctx, s.gravityContract.FromAddress())
	if err != nil {
		err = errors.Wrap(err, "failed to get latest Valset nonce")
		return err
	}

	// Check if latestCosmosConfirmed already submitted by other validators in mean time
	if latestValidValset.Nonce <= latestEthereumValsetNonce.Uint64() {
		// This valset update is already confirmed.
		return nil
	}

	// We might not need to relay this valset update unless the user explicitly specified it.
	if s.valsetRelayMode == ValsetRelayModeMinimum && latestValidValset.Nonce == latestValsets.Valsets[0].Nonce {
		s.logger.Debug().Msg("not relaying because nonces match")
		return nil
	}

	s.logger.Info().
		Uint64("latest_cosmos_confirmed_nonce", latestValidValset.Nonce).
		Uint64("latest_ethereum_valset_nonce", latestEthereumValsetNonce.Uint64()).
		Msg("detected latest cosmos valset nonce, but latest valset on Ethereum is different. Sending update to Ethereum")

	txData, err := s.gravityContract.EncodeValsetUpdate(
		ctx,
		currentValset,
		*latestValidValset,
		latestValidValsetSigs,
	)
	if err != nil {
		s.logger.Err(err).Msg("failed to encode valset update")
		return nil
	}

	if txData == nil {
		return nil
	}

	estimatedGasCost, gasPrice, err := s.gravityContract.EstimateGas(ctx, s.gravityContract.Address(), txData)
	if err != nil {
		s.logger.Err(err).Msg("failed to estimate gas cost")
		return nil
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
		// TODO: not sure if I should return this error or not
		return err
	}

	s.logger.Info().Str("tx_hash", txHash.Hex()).Msg("sent Tx (Gravity updateValset)")

	// update our local tracker of the latest valset
	s.lastSentValsetNonce = latestValidValset.Nonce

	return nil
}

func (s *gravityRelayer) findLatestValidValset(
	ctx context.Context,
	currentValset types.Valset,
	latestNonceOnCosmos uint64,
) (
	*types.Valset,
	[]types.MsgValsetConfirm,
	error,
) {
	for latestNonce := latestNonceOnCosmos; latestNonce > currentValset.Nonce; latestNonce-- {
		var err error
		valsetRes, err := s.cosmosQueryClient.ValsetRequest(ctx, &types.QueryValsetRequestRequest{
			Nonce: latestNonce,
		})

		if err != nil {
			return nil, nil, err
		}

		if valsetRes == nil {
			return nil, nil, errors.New("no valset found")
		}

		confirmsRes, err := s.cosmosQueryClient.ValsetConfirmsByNonce(ctx, &types.QueryValsetConfirmsByNonceRequest{
			Nonce: valsetRes.Valset.Nonce,
		})

		if err != nil {
			err = errors.Wrapf(err, "failed to get valset confirms at nonce %d", valsetRes.Valset.Nonce)
			return nil, nil, err
		}

		if confirmsRes == nil {
			continue
		}

		// Use this function to check if the valset is confirmed and valid.
		_, err = s.gravityContract.EncodeValsetUpdate(
			ctx,
			currentValset,
			*valsetRes.Valset,
			confirmsRes.Confirms,
		)

		if err == nil {
			// This valset update is confirmed and valid.
			return valsetRes.Valset, confirmsRes.Confirms, nil
		}

	}

	// If we couldn't find a valid valset with a greater nonce, then that means we are up to date.
	return nil, nil, nil
}
