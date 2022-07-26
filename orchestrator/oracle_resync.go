package orchestrator

import (
	"context"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/pkg/errors"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

// GetLastCheckedBlock retrieves the Ethereum block height from the last claim event this oracle has relayed to Cosmos.
func (p *gravityOrchestrator) GetLastCheckedBlock(
	ctx context.Context,
	ethBlockConfirmationDelay uint64,
) (uint64, error) {

	lastEventResp, err := p.cosmosQueryClient.LastEventNonceByAddr(ctx, &types.QueryLastEventNonceByAddrRequest{
		Address: p.gravityBroadcastClient.AccFromAddress().String(),
	})

	if err != nil {
		return uint64(0), err
	}

	lastEventNonce := lastEventResp.EventNonce

	// zero indicates this oracle has never submitted an event before since there is no
	// zero event nonce (it's pre-incremented in the solidity contract) we have to go
	// and look for event nonce one.
	if lastEventNonce == 0 {
		lastEventNonce = 1
	}

	// add delay to ensure minimum confirmations are received and block is finalized
	currentBlock, err := p.getCurrentBlock(ctx, ethBlockConfirmationDelay)
	if err != nil {
		err = errors.Wrap(err, "failed to get latest header")
		return 0, err
	}

	for currentBlock > 0 {
		endSearch := uint64(0)
		if currentBlock < p.ethBlocksPerLoop {
			endSearch = 0
		} else {
			endSearch = currentBlock - p.ethBlocksPerLoop
		}

		gravityFilterer, err := wrappers.NewGravityFilterer(p.gravityContract.Address(), p.ethProvider)
		if err != nil {
			err = errors.Wrap(err, "failed to init Gravity events filterer")
			return 0, err
		}

		iterSendToCosmos, err := gravityFilterer.FilterSendToCosmosEvent(&bind.FilterOpts{
			Start: endSearch,
			End:   &currentBlock,
		}, nil, nil)
		if err != nil {
			p.logger.Err(err).
				Uint64("start", endSearch).
				Uint64("end", currentBlock).
				Msg("failed to scan past SendToCosmos events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
				return 0, err
			} else if iterSendToCosmos == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iterSendToCosmos.Next() {
			if iterSendToCosmos.Event.EventNonce.Uint64() == lastEventNonce {
				return iterSendToCosmos.Event.Raw.BlockNumber, nil
			}
		}

		iterSendToCosmos.Close()

		iterTXBatchExec, err := gravityFilterer.FilterTransactionBatchExecutedEvent(&bind.FilterOpts{
			Start: endSearch,
			End:   &currentBlock,
		}, nil, nil)
		if err != nil {
			p.logger.Err(err).
				Uint64("start", endSearch).
				Uint64("end", currentBlock).
				Msg("failed to scan past TransactionBatchExecuted events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past TransactionBatchExecuted events from Ethereum")
				return 0, err
			} else if iterTXBatchExec == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iterTXBatchExec.Next() {
			if iterTXBatchExec.Event.EventNonce.Uint64() == lastEventNonce {
				return iterTXBatchExec.Event.Raw.BlockNumber, nil
			}
		}

		iterTXBatchExec.Close()

		iterErc20Deploy, err := gravityFilterer.FilterERC20DeployedEvent(&bind.FilterOpts{
			Start: endSearch,
			End:   &currentBlock,
		}, nil)
		if err != nil {
			p.logger.Err(err).
				Uint64("start", endSearch).
				Uint64("end", currentBlock).
				Msg("failed to scan past ERC20Deployed events from Ethereum")

			if !isUnknownBlockErr(err) {
				err = errors.Wrap(err, "failed to scan past ERC20Deployed events from Ethereum")
				return 0, err
			} else if iterErc20Deploy == nil {
				return 0, errors.New("no iterator returned")
			}
		}

		for iterErc20Deploy.Next() {
			if iterErc20Deploy.Event.EventNonce.Uint64() == lastEventNonce {
				return iterErc20Deploy.Event.Raw.BlockNumber, nil
			}
		}

		iterErc20Deploy.Close()

		// This reverse solves a very specific bug, we use the properties of the first valsets for edgecase
		// handling here, but events come in chronological order, so if we don't reverse the iterator
		// we will encounter the first validator sets first and exit early and incorrectly.
		// Note that reversing everything won't actually get you that much of a performance gain
		// because this only involves events within the searching block range.
		var valsetUpdatedEvents []*wrappers.GravityValsetUpdatedEvent
		{
			iter, err := gravityFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
				Start: endSearch,
				End:   &currentBlock,
			}, nil)
			if err != nil {
				p.logger.Err(err).
					Uint64("start", endSearch).
					Uint64("end", currentBlock).
					Msg("failed to scan past ValsetUpdatedEvent events from Ethereum")

				if !isUnknownBlockErr(err) {
					err = errors.Wrap(err, "failed to scan past ValsetUpdatedEvent events from Ethereum")
					return 0, err
				} else if iter == nil {
					return 0, errors.New("no iterator returned")
				}
			}

			for iter.Next() {
				valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
			}

			iter.Close()
		}

		// There's no easy way to reverse the list, so we have to do it manually.
		for i := 0; i < len(valsetUpdatedEvents)/2; i++ {
			j := len(valsetUpdatedEvents) - i - 1
			valsetUpdatedEvents[i], valsetUpdatedEvents[j] = valsetUpdatedEvents[j], valsetUpdatedEvents[i]
		}

		for _, valset := range valsetUpdatedEvents {
			bootstrapping := valset.NewValsetNonce.Uint64() == 0 && lastEventNonce == 1
			commonCase := valset.EventNonce.Uint64() == lastEventNonce

			if commonCase || bootstrapping {
				return valset.Raw.BlockNumber, nil
			} else if valset.NewValsetNonce.Uint64() == 0 && lastEventNonce > 1 {
				// If another iterator is added below the valset iterator, this panic will be triggered. Add new
				// iterators above.
				p.logger.Panic().Msg("could not find the last event relayed")
			}
		}

		currentBlock = endSearch
	}

	return 0, errors.New("reached the end of block history without finding the Gravity contract deploy event")
}

// getCurrentBlock returns the latest block in the eth
// if the latest block in eth is less than the confirmation
// it returns 0, if is bigger than confirmation it removes
// the amount of confirmations
func (p *gravityOrchestrator) getCurrentBlock(
	ctx context.Context,
	ethBlockConfirmationDelay uint64,
) (uint64, error) {
	latestHeader, err := p.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		err = errors.Wrap(err, "failed to get latest header")
		return 0, err
	}

	latestBlock := latestHeader.Number.Uint64()

	// checks if the latest block is less than the amount of confirmation
	if latestBlock < ethBlockConfirmationDelay {
		return 0, nil
	}

	// add delay to ensure minimum confirmations are received and block is finalized
	return latestBlock - ethBlockConfirmationDelay, nil
}
