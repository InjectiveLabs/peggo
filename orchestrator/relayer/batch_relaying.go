package relayer

import (
	"context"
	"math/big"
	"sort"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

type SubmittableBatch struct {
	Batch      types.OutgoingTxBatch
	Signatures []types.MsgConfirmBatch
}

// getBatchesAndSignatures retrieves the latest batches from the Cosmos module and then iterates through the signatures
// for each batch, determining if they are ready to submit. It is possible for a batch to not have valid signatures for
// two reasons one is that not enough signatures have been collected yet from the validators two is that the batch is
// old enough that the signatures do not reflect the current validator set on Ethereum. In both the later and the former
// case the correct solution is to wait through timeouts, new signatures, or a later valid batch being submitted old
// batches will always be resolved.
func (s *gravityRelayer) getBatchesAndSignatures(
	ctx context.Context,
	currentValset types.Valset,
) (map[ethcmn.Address][]SubmittableBatch, error) {
	possibleBatches := map[ethcmn.Address][]SubmittableBatch{}

	outTxBatches, err := s.cosmosQueryClient.OutgoingTxBatches(ctx, &types.QueryOutgoingTxBatchesRequest{})

	if err != nil {
		s.logger.Err(err).Msg("failed to get latest batches")
		return possibleBatches, err
	} else if outTxBatches == nil {
		s.logger.Info().Msg("no outgoing TX batches found")
		return possibleBatches, nil
	}

	for _, batch := range outTxBatches.Batches {

		// We might have already sent this same batch. Skip it.
		if s.lastSentBatchNonce >= batch.BatchNonce {
			continue
		}

		batchConfirms, err := s.cosmosQueryClient.BatchConfirms(ctx, &types.QueryBatchConfirmsRequest{
			Nonce:           batch.BatchNonce,
			ContractAddress: batch.TokenContract,
		})

		if err != nil || batchConfirms == nil {
			// If we can't get the signatures for a batch we will continue to the next batch.
			// Use Error() instead of Err() because the latter will print on info level instead of error if err == nil.
			s.logger.Error().
				AnErr("error", err).
				Uint64("batch_nonce", batch.BatchNonce).
				Str("token_contract", batch.TokenContract).
				Msg("failed to get batch's signatures")
			continue
		}

		// This checks that the signatures for the batch are actually possible to submit to the chain.
		// We only need to know if the signatures are good, we won't use the other returned value.
		_, err = s.gravityContract.EncodeTransactionBatch(ctx, currentValset, batch, batchConfirms.Confirms)

		if err != nil {
			// this batch is not ready to be relayed
			s.logger.
				Debug().
				AnErr("err", err).
				Uint64("batch_nonce", batch.BatchNonce).
				Str("token_contract", batch.TokenContract).
				Msg("batch can't be submitted yet, waiting for more signatures")

			// Do not return an error here, we want to continue to the next batch
			continue
		}

		// if the previous check didn't fail, we can add the batch to the list of possible batches
		possibleBatches[ethcmn.HexToAddress(batch.TokenContract)] = append(
			possibleBatches[ethcmn.HexToAddress(batch.TokenContract)],
			SubmittableBatch{Batch: batch, Signatures: batchConfirms.Confirms},
		)
	}

	// Order batches by nonce ASC. That means that the next/oldest batch is [0].
	for tokenAddress := range possibleBatches {
		tokenAddress := tokenAddress
		sort.SliceStable(possibleBatches[tokenAddress], func(i, j int) bool {
			return possibleBatches[tokenAddress][i].Batch.BatchNonce > possibleBatches[tokenAddress][j].Batch.BatchNonce
		})
	}

	return possibleBatches, nil
}

// RelayBatches attempts to submit batches with valid signatures, checking the state of the Ethereum chain to ensure
// that it is valid to submit a given batch, more specifically that the correctly signed batch has not timed out or
// already been submitted. The goal of this function is to submit batches in chronological order of their creation.
// This function estimates the cost of submitting a batch before submitting it to Ethereum, if it is determined that
// the ETH cost to submit is too high the batch will be skipped and a later, more profitable, batch may be submitted.
// Keep in mind that many other relayers are making this same computation and some may have different standards for
// their profit margin, therefore there may be a race not only to submit individual batches but also batches in
// different orders.
func (s *gravityRelayer) RelayBatches(
	ctx context.Context,
	currentValset types.Valset,
	possibleBatches map[ethcmn.Address][]SubmittableBatch,
) error {
	// first get current block height to check for any timeouts
	lastEthereumHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		s.logger.Err(err).Msg("failed to get last ethereum header")
		return err
	}

	ethBlockHeight := lastEthereumHeader.Number.Uint64()

	for tokenContract, batches := range possibleBatches {

		// Requests data from Ethereum only once per token type, this is valid because we are
		// iterating from oldest to newest, so submitting a batch earlier in the loop won't
		// ever invalidate submitting a batch later in the loop. Another relayer could always
		// do that though.
		latestEthereumBatch, err := s.gravityContract.GetTxBatchNonce(
			ctx,
			tokenContract,
			s.gravityContract.FromAddress(),
		)
		if err != nil {
			s.logger.Err(err).Msg("failed to get latest Ethereum batch")
			return err
		}

		// now we iterate through batches per token type
		for _, batch := range batches {
			if batch.Batch.BatchTimeout < ethBlockHeight {
				s.logger.Debug().
					Uint64("batch_nonce", batch.Batch.BatchNonce).
					Str("token_contract", batch.Batch.TokenContract).
					Uint64("batch_timeout", batch.Batch.BatchTimeout).
					Uint64("eth_block_height", ethBlockHeight).
					Msg("batch has timed out and can't be submitted")
				continue
			}

			// if the batch is newer than the latest Ethereum batch, we can submit it
			if batch.Batch.BatchNonce <= latestEthereumBatch.Uint64() {
				continue
			}

			txData, err := s.gravityContract.EncodeTransactionBatch(ctx, currentValset, batch.Batch, batch.Signatures)
			if err != nil {
				return err
			}

			if txData == nil {
				continue
			}

			estimatedGasCost, gasPrice, err := s.gravityContract.EstimateGas(ctx, s.gravityContract.Address(), txData)
			if err != nil {
				s.logger.Err(err).Msg("failed to estimate gas cost")
				return err
			}

			// If the batch is not profitable, move on to the next one.
			if !s.IsBatchProfitable(ctx, batch.Batch, estimatedGasCost, gasPrice, s.profitMultiplier) {
				continue
			}

			// Checking in pending txs(mempool) if tx with same input is already submitted
			// We have to check this at the last moment because any other relayer could have submitted.
			if s.gravityContract.IsPendingTxInput(txData, s.pendingTxWait) {
				s.logger.Debug().
					Msg("Transaction with same batch input data is already present in mempool")
				continue
			}

			s.logger.Info().
				Uint64("latest_batch", batch.Batch.BatchNonce).
				Uint64("latest_ethereum_batch", latestEthereumBatch.Uint64()).
				Msg("we have detected a newer profitable batch; sending an update")

			txHash, err := s.gravityContract.SendTx(ctx, s.gravityContract.Address(), txData, estimatedGasCost, gasPrice)
			if err != nil {
				s.logger.Err(err).Str("tx_hash", txHash.Hex()).Msg("failed to sign and submit (Gravity submitBatch) to EVM")
				return err
			}

			s.logger.Info().Str("tx_hash", txHash.Hex()).Msg("sent Tx (Gravity submitBatch)")

			// update our local tracker of the latest batch
			s.lastSentBatchNonce = batch.Batch.BatchNonce
		}

	}

	return nil
}

// IsBatchProfitable gets the current prices in USD of ETH and the ERC20 token and compares the value of the estimated
// gas cost of the transaction to the fees paid by the batch. If the estimated gas cost is greater than the batch's
// fees, the batch is not profitable and should not be submitted.
func (s *gravityRelayer) IsBatchProfitable(
	ctx context.Context,
	batch types.OutgoingTxBatch,
	ethGasCost uint64,
	gasPrice *big.Int,
	profitMultiplier float64,
) bool {
	if s.priceFeeder == nil || profitMultiplier == 0 {
		return true
	}

	// First we get the cost of the transaction in USD
	usdEthPrice, err := s.priceFeeder.QueryETHUSDPrice()
	if err != nil {
		s.logger.Err(err).Msg("failed to get ETH price")
		return false
	}
	usdEthPriceDec := decimal.NewFromFloat(usdEthPrice)
	totalETHcost := big.NewInt(0).Mul(gasPrice, big.NewInt(int64(ethGasCost)))

	// Ethereum decimals are 18 and that's a constant.
	gasCostInUSDDec := decimal.NewFromBigInt(totalETHcost, -18).Mul(usdEthPriceDec)

	// Then we get the fees of the batch in USD
	decimals, err := s.gravityContract.GetERC20Decimals(
		ctx,
		ethcmn.HexToAddress(batch.TokenContract),
		s.gravityContract.FromAddress(),
	)
	if err != nil {
		s.logger.Err(err).Str("token_contract", batch.TokenContract).Msg("failed to get token decimals")
		return false
	}

	s.logger.Debug().
		Uint8("decimals", decimals).
		Str("token_contract", batch.TokenContract).
		Msg("got token decimals")

	usdTokenPrice, err := s.priceFeeder.QueryUSDPrice(ethcmn.HexToAddress(batch.TokenContract))
	if err != nil {
		return false
	}

	// We calculate the total fee in ERC20 tokens
	totalBatchFees := big.NewInt(0)
	for _, tx := range batch.Transactions {
		totalBatchFees = totalBatchFees.Add(tx.Erc20Fee.Amount.BigInt(), totalBatchFees)
	}

	usdTokenPriceDec := decimal.NewFromFloat(usdTokenPrice)
	// Decimals (uint8) can be safely casted into int32 because the max uint8 is 255 and the max int32 is 2147483647.
	totalFeeInUSDDec := decimal.NewFromBigInt(totalBatchFees, -int32(decimals)).Mul(usdTokenPriceDec)

	// Simplified: totalFee > (gasCost * profitMultiplier).
	isProfitable := totalFeeInUSDDec.GreaterThanOrEqual(gasCostInUSDDec.Mul(decimal.NewFromFloat(profitMultiplier)))

	s.logger.Debug().
		Str("token_contract", batch.TokenContract).
		Float64("token_price_in_usd", usdTokenPrice).
		Int64("total_fees", totalBatchFees.Int64()).
		Float64("total_fee_in_usd", totalFeeInUSDDec.InexactFloat64()).
		Float64("gas_cost_in_usd", gasCostInUSDDec.InexactFloat64()).
		Float64("profit_multiplier", profitMultiplier).
		Bool("is_profitable", isProfitable).
		Msg("checking if batch is profitable")

	return isProfitable

}
