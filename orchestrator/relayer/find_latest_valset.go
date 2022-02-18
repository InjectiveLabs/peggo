package relayer

import (
	"context"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/pkg/errors"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/types"

	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

const defaultBlocksToSearch = 2000

// FindLatestValset finds the latest valset on the Gravity contract by looking back through the event
// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
// as the latest update will be in recent blockchain history and the search moves from the present
// backwards in time. In the case that the validator set has not been updated for a very long time
// this will take longer.
func (s *gravityRelayer) FindLatestValset(ctx context.Context) (*types.Valset, error) {
	latestHeader, err := s.ethProvider.HeaderByNumber(ctx, nil)
	if err != nil {
		err = errors.Wrap(err, "failed to get latest header")
		return nil, err
	}
	currentBlock := latestHeader.Number.Uint64()

	gravityFilterer, err := wrappers.NewGravityFilterer(s.gravityContract.Address(), s.ethProvider)
	if err != nil {
		err = errors.Wrap(err, "failed to init Gravity events filterer")
		return nil, err
	}

	latestEthereumValsetNonce, err := s.gravityContract.GetValsetNonce(ctx, s.gravityContract.FromAddress())
	if err != nil {
		err = errors.Wrap(err, "failed to get latest Valset nonce")
		return nil, err
	}

	cosmosValset, err := s.cosmosQueryClient.ValsetRequest(ctx, &types.QueryValsetRequestRequest{
		Nonce: latestEthereumValsetNonce.Uint64(),
	})

	if err != nil {
		err = errors.Wrap(err, "failed to get cosmos Valset")
		return nil, err
	} else if cosmosValset == nil {
		return nil, errors.New("failed to get cosmos Valset, empty response")
	}

	for currentBlock > 0 {
		var endSearchBlock uint64
		if currentBlock <= defaultBlocksToSearch {
			endSearchBlock = 0
		} else {
			endSearchBlock = currentBlock - defaultBlocksToSearch
		}

		var valsetUpdatedEvents []*wrappers.GravityValsetUpdatedEvent
		iter, err := gravityFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
			Start: endSearchBlock,
			End:   &currentBlock,
		}, nil)

		if err != nil {
			err = errors.Wrap(err, "failed to filter past ValsetUpdated events from Ethereum")
			return nil, err
		}

		for iter.Next() {
			valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
		}

		iter.Close()

		// by default the lowest found valset goes first, we want the highest
		//
		// TODO(xlab): this follows the original impl, but sort might be skipped there:
		// we could access just the latest element later.
		sort.Sort(sort.Reverse(GravityValsetUpdatedEvents(valsetUpdatedEvents)))

		s.logger.Debug().
			Int("valset_updated_events_num", len(valsetUpdatedEvents)).
			Msg("found ValsetUpdated events")

		// we take only the first event if we find any at all.
		if len(valsetUpdatedEvents) > 0 {
			event := valsetUpdatedEvents[0]
			valset := &types.Valset{
				Nonce:        event.NewValsetNonce.Uint64(),
				Members:      make([]types.BridgeValidator, 0, len(event.Powers)),
				RewardAmount: sdk.NewIntFromBigInt(event.RewardAmount),
				RewardToken:  event.RewardToken.Hex(),
			}

			for idx, p := range event.Powers {
				valset.Members = append(valset.Members, types.BridgeValidator{
					Power:           p.Uint64(),
					EthereumAddress: event.Validators[idx].Hex(),
				})
			}

			s.checkIfValsetsDiffer(cosmosValset.Valset, valset)
			return valset, nil
		}

		currentBlock = endSearchBlock
	}

	return nil, ErrNotFound
}

var ErrNotFound = errors.New("not found")

type GravityValsetUpdatedEvents []*wrappers.GravityValsetUpdatedEvent

func (a GravityValsetUpdatedEvents) Len() int { return len(a) }
func (a GravityValsetUpdatedEvents) Less(i, j int) bool {
	return a[i].NewValsetNonce.Cmp(a[j].NewValsetNonce) < 0
}
func (a GravityValsetUpdatedEvents) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// This function exists to provide a warning if Cosmos and Ethereum have different validator sets
// for a given nonce. In the mundane version of this warning the validator sets disagree on sorting order
// which can happen if some relayer uses an unstable sort, or in a case of a mild griefing attack.
// The Gravity contract validates signatures in order of highest to lowest power. That way it can exit
// the loop early once a vote has enough power, if a relayer where to submit things in the reverse order
// they could grief users of the contract into paying more in gas.
// The other (and far worse) way a disagreement here could occur is if validators are colluding to steal
// funds from the Gravity contract and have submitted a hijacking update. If slashing for off Cosmos chain
// Ethereum signatures is implemented you would put that handler here.
func (s *gravityRelayer) checkIfValsetsDiffer(cosmosValset, ethereumValset *types.Valset) {
	if cosmosValset == nil && ethereumValset.Nonce == 0 {
		// bootstrapping case
		return
	} else if cosmosValset == nil {
		s.logger.Error().
			Uint64("eth_valset_nonce", ethereumValset.Nonce).
			Msg("cosmos does not have a valset for nonce from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if cosmosValset.Nonce != ethereumValset.Nonce {

		s.logger.Error().
			Uint64("eth_valset_nonce", ethereumValset.Nonce).
			Uint64("cosmos_valset_nonce", cosmosValset.Nonce).
			Msg("cosmos does have a wrong valset nonce, differs from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if len(cosmosValset.Members) != len(ethereumValset.Members) {
		s.logger.Error().
			Int("eth_valset", len(ethereumValset.Members)).
			Int("cosmos_valset", len(cosmosValset.Members)).
			Msg("cosmos and Ethereum Valsets have different length. Possible bridge hijacking!")
		return
	}

	BridgeValidators(cosmosValset.Members).Sort()
	BridgeValidators(ethereumValset.Members).Sort()

	for idx, member := range cosmosValset.Members {
		if ethereumValset.Members[idx].EthereumAddress != member.EthereumAddress {
			s.logger.Error().Msg("valsets are different, a sorting error?")
		}
		if ethereumValset.Members[idx].Power != member.Power {
			s.logger.Error().Msg("valsets are different, a sorting error?")
		}
	}
}

type BridgeValidators []types.BridgeValidator

// Sort sorts the validators by power
func (b BridgeValidators) Sort() {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Power == b[j].Power {
			// Secondary sort on eth address in case powers are equal
			addrI, _ := types.NewEthAddress(b[i].EthereumAddress)
			addrJ, _ := types.NewEthAddress(b[j].EthereumAddress)
			return types.EthAddrLessThan(*addrI, *addrJ)
		}
		return b[i].Power > b[j].Power
	})
}

// HasDuplicates returns true if there are duplicates in the set
func (b BridgeValidators) HasDuplicates() bool {
	m := make(map[string]struct{}, len(b))
	for i := range b {
		m[b[i].EthereumAddress] = struct{}{}
	}
	return len(m) != len(b)
}

// GetPowers returns only the power values for all members
func (b BridgeValidators) GetPowers() []uint64 {
	r := make([]uint64, len(b))
	for i := range b {
		r[i] = b[i].Power
	}
	return r
}
