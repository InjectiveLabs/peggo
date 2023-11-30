package orchestrator

import (
	"context"
	"github.com/shopspring/decimal"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/avast/retry-go"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/util"
	"github.com/InjectiveLabs/peggo/orchestrator/loops"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

func (s *PeggyOrchestrator) RelayerMainLoop(ctx context.Context) (err error) {
	rel := &relayer{
		log:                  log.WithField("loop", "Relayer"),
		retries:              s.maxAttempts,
		relayValsetOffsetDur: s.relayValsetOffsetDur,
		relayBatchOffsetDur:  s.relayBatchOffsetDur,
		valsetRelaying:       s.valsetRelayEnabled,
		batchRelaying:        s.batchRelayEnabled,
	}

	return loops.RunLoop(
		ctx,
		defaultLoopDur,
		func() error { return rel.run(ctx, s.injective, s.ethereum) },
	)
}

type relayer struct {
	log                  log.Logger
	retries              uint
	relayValsetOffsetDur time.Duration
	relayBatchOffsetDur  time.Duration
	valsetRelaying       bool
	batchRelaying        bool
}

func (r *relayer) run(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) error {
	var pg loops.ParanoidGroup

	if r.valsetRelaying {
		pg.Go(func() error {
			return retry.Do(
				func() error { return r.relayValsets(ctx, injective, ethereum) },
				retry.Context(ctx),
				retry.Attempts(r.retries),
				retry.OnRetry(func(n uint, err error) {
					r.log.WithError(err).Warningf("failed to relay valsets, will retry (%d)", n)
				}),
			)
		})
	}

	if r.batchRelaying {
		pg.Go(func() error {
			return retry.Do(func() error { return r.relayBatches(ctx, injective, ethereum) },
				retry.Context(ctx),
				retry.Attempts(r.retries),
				retry.OnRetry(func(n uint, err error) {
					r.log.WithError(err).Warningf("failed to relay batches, will retry (%d)", n)
				}),
			)
		})
	}

	if pg.Initialized() {
		if err := pg.Wait(); err != nil {
			r.log.WithError(err).Errorln("got error, loop exits")
			return err
		}
	}

	return nil
}

func (r *relayer) relayValsets(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) error {
	// we should determine if we need to relay one
	// to Ethereum for that we will find the latest confirmed valset and compare it to the ethereum chain
	latestValsets, err := injective.LatestValsets(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get latest valset updates from Injective")
	}

	var (
		oldestConfirmedValset     *types.Valset
		oldestConfirmedValsetSigs []*types.MsgValsetConfirm
	)

	for _, set := range latestValsets {
		sigs, err := injective.AllValsetConfirms(ctx, set.Nonce)
		if err != nil {
			return errors.Wrapf(err, "failed to get valset confirmations for nonce %d", set.Nonce)
		} else if len(sigs) == 0 {
			continue
		}

		oldestConfirmedValsetSigs = sigs
		oldestConfirmedValset = set
		break
	}

	if oldestConfirmedValset == nil {
		r.log.Debugln("no valset update to relay")
		return nil
	}

	currentEthValset, err := r.findLatestValsetOnEth(ctx, injective, ethereum)
	if err != nil {
		return errors.Wrap(err, "failed to find latest confirmed valset update on Ethereum")
	}

	r.log.WithFields(log.Fields{
		"inj_valset_nonce": oldestConfirmedValset.Nonce,
		"eth_valset_nonce": currentEthValset.Nonce,
	}).Debugln("latest valset updates")

	if oldestConfirmedValset.Nonce <= currentEthValset.Nonce {
		return nil
	}

	latestEthereumValsetNonce, err := ethereum.GetValsetNonce(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get latest valset nonce from Ethereum")
	}

	// Check if other validators already updated the valset
	if oldestConfirmedValset.Nonce <= latestEthereumValsetNonce.Uint64() {
		return nil
	}

	// Check custom time delay offset
	blockResult, err := injective.GetBlock(ctx, int64(oldestConfirmedValset.Height))
	if err != nil {
		return errors.Wrapf(err, "failed to get block %d from Injective", oldestConfirmedValset.Height)
	}

	if timeElapsed := time.Since(blockResult.Block.Time); timeElapsed <= r.relayValsetOffsetDur {
		timeRemaining := r.relayBatchOffsetDur - timeElapsed
		r.log.WithField("time_remaining", timeRemaining.String()).Debugln("valset relay offset duration not expired")
		return nil
	}

	r.log.WithFields(log.Fields{
		"inj_valset_nonce": oldestConfirmedValset.Nonce,
		"eth_valset_nonce": latestEthereumValsetNonce,
	}).Infoln("detected new valset update on Injective")

	txHash, err := ethereum.SendEthValsetUpdate(
		ctx,
		currentEthValset,
		oldestConfirmedValset,
		oldestConfirmedValsetSigs,
	)
	if err != nil {
		return err
	}

	r.log.WithFields(log.Fields{
		"nonce":         oldestConfirmedValset.Nonce,
		"members":       len(oldestConfirmedValset.Members),
		"confirmations": len(oldestConfirmedValsetSigs),
		"tx_hash":       txHash.String(),
	}).Infoln("sent new valset update to Ethereum")

	return nil
}

func (r *relayer) relayBatches(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) error {
	latestValsetEth, err := r.findLatestValsetOnEth(ctx, injective, ethereum)
	if err != nil {
		return errors.Wrap(err, "failed to find latest valset")
	} else if latestValsetEth == nil {
		return errors.Wrap(err, "latest valset not found")
	}

	outgoingBatches, err := injective.LatestTransactionBatches(ctx)
	if err != nil {
		return err
	}

	var (
		earliestConfirmedBatch     *types.OutgoingTxBatch
		earliestConfirmedBatchSigs []*types.MsgConfirmBatch
	)

	// find the earliest batch that has sufficient number of confirmations
	for _, batch := range outgoingBatches {
		sigs, err := injective.TransactionBatchSignatures(ctx, batch.BatchNonce, common.HexToAddress(batch.TokenContract))
		if err != nil {
			return err
		}

		if !majorityConfirms(latestValsetEth, sigs) {
			// log
			continue
		}

		earliestConfirmedBatch = batch
		earliestConfirmedBatchSigs = sigs
		break
	}

	if earliestConfirmedBatch == nil {
		// log
		return nil
	}

	latestBatchNonceEth, err := ethereum.GetTxBatchNonce(ctx, common.HexToAddress(earliestConfirmedBatch.TokenContract))
	if err != nil {
		return err
	}

	r.log.WithFields(log.Fields{
		"inj_batch_nonce": earliestConfirmedBatch.BatchNonce,
		"eth_batch_nonce": latestBatchNonceEth.Uint64(),
	}).Debugln("latest token batches")

	// Check if ethereum batch was updated by other relayers
	if earliestConfirmedBatch.BatchNonce <= latestBatchNonceEth.Uint64() {
		return nil
	}

	// Check if batch relay offset has expired
	blockResult, err := injective.GetBlock(ctx, int64(earliestConfirmedBatch.Block))
	if err != nil {
		return errors.Wrapf(err, "failed to get block %d from Injective", earliestConfirmedBatch.Block)
	}

	if timeElapsed := time.Since(blockResult.Block.Time); timeElapsed <= r.relayBatchOffsetDur {
		timeRemaining := time.Duration(int64(r.relayBatchOffsetDur) - int64(timeElapsed))
		r.log.WithField("time_remaining", timeRemaining.String()).Debugln("batch relay offset not expired")
		return nil
	}

	r.log.WithFields(log.Fields{
		"inj_batch_nonce": earliestConfirmedBatch.BatchNonce,
		"eth_batch_nonce": latestBatchNonceEth.Uint64(),
	}).Infoln("detected new token batch on Injective")

	// Send SendTransactionBatch to Ethereum
	txHash, err := ethereum.SendTransactionBatch(ctx, latestValsetEth, earliestConfirmedBatch, earliestConfirmedBatchSigs)
	if err != nil {
		return err
	}

	r.log.WithFields(log.Fields{
		"nonce":         earliestConfirmedBatch.BatchNonce,
		"batch_txs":     len(earliestConfirmedBatch.Transactions),
		"confirmations": len(earliestConfirmedBatchSigs),
		"tx_hash":       txHash.String(),
	}).Infoln("sent new token batch to Ethereum")

	return nil
}

func (r *relayer) shouldRelayBatch(t time.Time) bool {
	if timeElapsed := time.Since(t); timeElapsed <= r.relayBatchOffsetDur {
		timeRemaining := time.Duration(int64(r.relayBatchOffsetDur) - int64(timeElapsed))

		r.log.WithField("time_remaining", timeRemaining.String()).Debugln("batch relay offset duration not expired")
		return false
	}

	return true
}

func (r *relayer) getOldestBatchConfirmedByMajority(
	ctx context.Context,
	injective InjectiveNetwork,
	valset *types.Valset,
) (
	*types.OutgoingTxBatch,
	[]*types.MsgConfirmBatch,
	error,
) {
	latestBatches, err := injective.LatestTransactionBatches(ctx)
	if err != nil {
		return nil, nil, err
	}

	var (
		oldestConfirmedBatch     *types.OutgoingTxBatch
		oldestConfirmedBatchSigs []*types.MsgConfirmBatch
	)

	for _, batch := range latestBatches {
		sigs, err := injective.TransactionBatchSignatures(ctx, batch.BatchNonce, common.HexToAddress(batch.TokenContract))
		// make sure the batch is signed by majority
		if err != nil {
			return nil, nil, err
		}

		if !majorityConfirms(valset, sigs) {
			continue
		}

		oldestConfirmedBatch = batch
		oldestConfirmedBatchSigs = sigs

		break
	}

	return oldestConfirmedBatch, oldestConfirmedBatchSigs, nil
}

func majorityConfirms(vs *types.Valset, confirms []*types.MsgConfirmBatch) bool {
	if len(confirms) == 0 {
		return false
	}

	validatorConfirms := make(map[string]*types.MsgConfirmBatch, len(confirms))
	for _, c := range confirms {
		validatorConfirms[c.EthSigner] = c
	}

	votingPower := big.NewInt(0)
	for _, validator := range vs.Members {
		confirm, ok := validatorConfirms[validator.EthereumAddress]
		if !ok || confirm.EthSigner != validator.EthereumAddress {
			continue
		}

		votingPower.Add(votingPower, big.NewInt(0).SetUint64(validator.Power))
	}

	powerPercentage, _ := decimal.NewFromBigInt(votingPower, 0).Div(decimal.NewFromInt(math.MaxUint32)).Shift(2).Float64()
	if float32(powerPercentage) < 66 {
		return false
	}

	return true
}

const valsetBlocksToSearch = 2000

// FindLatestValset finds the latest valset on the Peggy contract by looking back through the event
// history and finding the most recent ValsetUpdatedEvent. Most of the time this will be very fast
// as the latest update will be in recent blockchain history and the search moves from the present
// backwards in time. In the case that the validator set has not been updated for a very long time
// this will take longer.
func (r *relayer) findLatestValsetOnEth(
	ctx context.Context,
	injective InjectiveNetwork,
	ethereum EthereumNetwork,
) (*types.Valset, error) {
	latestHeader, err := ethereum.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest eth header")
	}

	latestEthereumValsetNonce, err := ethereum.GetValsetNonce(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get latest valset nonce on Ethereum")
	}

	cosmosValset, err := injective.ValsetAt(ctx, latestEthereumValsetNonce.Uint64())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Injective valset")
	}

	currentBlock := latestHeader.Number.Uint64()

	for currentBlock > 0 {
		var startSearchBlock uint64
		if currentBlock <= valsetBlocksToSearch {
			startSearchBlock = 0
		} else {
			startSearchBlock = currentBlock - valsetBlocksToSearch
		}

		valsetUpdatedEvents, err := ethereum.GetValsetUpdatedEvents(startSearchBlock, currentBlock)
		if err != nil {
			return nil, errors.Wrap(err, "failed to filter past ValsetUpdated events from Ethereum")
		}

		// by default the lowest found valset goes first, we want the highest
		//
		// TODO(xlab): this follows the original impl, but sort might be skipped there:
		// we could access just the latest element later.
		sort.Sort(sort.Reverse(PeggyValsetUpdatedEvents(valsetUpdatedEvents)))

		if len(valsetUpdatedEvents) == 0 {
			currentBlock = startSearchBlock
			continue
		}

		// we take only the first event if we find any at all.
		event := valsetUpdatedEvents[0]
		valset := &types.Valset{
			Nonce:        event.NewValsetNonce.Uint64(),
			Members:      make([]*types.BridgeValidator, 0, len(event.Powers)),
			RewardAmount: sdk.NewIntFromBigInt(event.RewardAmount),
			RewardToken:  event.RewardToken.Hex(),
		}

		for idx, p := range event.Powers {
			valset.Members = append(valset.Members, &types.BridgeValidator{
				Power:           p.Uint64(),
				EthereumAddress: event.Validators[idx].Hex(),
			})
		}

		checkIfValsetsDiffer(cosmosValset, valset)

		return valset, nil

	}

	return nil, ErrNotFound
}

var ErrNotFound = errors.New("not found")

type PeggyValsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent

func (a PeggyValsetUpdatedEvents) Len() int { return len(a) }
func (a PeggyValsetUpdatedEvents) Less(i, j int) bool {
	return a[i].NewValsetNonce.Cmp(a[j].NewValsetNonce) < 0
}
func (a PeggyValsetUpdatedEvents) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// This function exists to provide a warning if Cosmos and Ethereum have different validator sets
// for a given nonce. In the mundane version of this warning the validator sets disagree on sorting order
// which can happen if some relayer uses an unstable sort, or in a case of a mild griefing attack.
// The Peggy contract validates signatures in order of highest to lowest power. That way it can exit
// the loop early once a vote has enough power, if a relayer where to submit things in the reverse order
// they could grief users of the contract into paying more in gas.
// The other (and far worse) way a disagreement here could occur is if validators are colluding to steal
// funds from the Peggy contract and have submitted a hijacking update. If slashing for off Cosmos chain
// Ethereum signatures is implemented you would put that handler here.
func checkIfValsetsDiffer(cosmosValset, ethereumValset *types.Valset) {
	if cosmosValset == nil && ethereumValset.Nonce == 0 {
		// bootstrapping case
		return
	} else if cosmosValset == nil {
		log.WithField(
			"eth_valset_nonce",
			ethereumValset.Nonce,
		).Errorln("Cosmos does not have a valset for nonce from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if cosmosValset.Nonce != ethereumValset.Nonce {
		log.WithFields(log.Fields{
			"cosmos_valset_nonce": cosmosValset.Nonce,
			"eth_valset_nonce":    ethereumValset.Nonce,
		}).Errorln("Cosmos does have a wrong valset nonce, differs from Ethereum chain. Possible bridge hijacking!")
		return
	}

	if len(cosmosValset.Members) != len(ethereumValset.Members) {
		log.WithFields(log.Fields{
			"cosmos_valset": len(cosmosValset.Members),
			"eth_valset":    len(ethereumValset.Members),
		}).Errorln("Cosmos and Ethereum Valsets have different length. Possible bridge hijacking!")
		return
	}

	BridgeValidators(cosmosValset.Members).Sort()
	BridgeValidators(ethereumValset.Members).Sort()

	for idx, member := range cosmosValset.Members {
		if ethereumValset.Members[idx].EthereumAddress != member.EthereumAddress {
			log.Errorln("Valsets are different, a sorting error?")
		}
		if ethereumValset.Members[idx].Power != member.Power {
			log.Errorln("Valsets are different, a sorting error?")
		}
	}
}

type BridgeValidators []*types.BridgeValidator

// Sort sorts the validators by power
func (b BridgeValidators) Sort() {
	sort.Slice(b, func(i, j int) bool {
		if b[i].Power == b[j].Power {
			// Secondary sort on eth address in case powers are equal
			return util.EthAddrLessThan(b[i].EthereumAddress, b[j].EthereumAddress)
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
