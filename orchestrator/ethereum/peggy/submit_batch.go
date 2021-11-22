package peggy

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/umee-network/umee/x/peggy/types"
)

type RepackedBatchSigs struct {
	validators []common.Address
	powers     []*big.Int
	v          []uint8
	r          []common.Hash
	s          []common.Hash
}

func (s *peggyContract) EncodeTransactionBatch(
	ctx context.Context,
	currentValset *types.Valset,
	batch *types.OutgoingTxBatch,
	confirms []*types.MsgConfirmBatch,
) ([]byte, error) {

	sigs, err := CheckBatchSigsAndRepack(currentValset, confirms)
	if err != nil {
		err = errors.Wrap(err, "confirmations check failed")
		return nil, err
	}

	amounts, destinations, fees := getBatchCheckpointValues(batch)
	currentValsetNonce := new(big.Int).SetUint64(currentValset.Nonce)
	batchNonce := new(big.Int).SetUint64(batch.BatchNonce)
	batchTimeout := new(big.Int).SetUint64(batch.BatchTimeout)

	currentValsetArs := ValsetArgs{
		Validators:   sigs.validators,
		Powers:       sigs.powers,
		ValsetNonce:  currentValsetNonce,
		RewardAmount: currentValset.RewardAmount.BigInt(),
		RewardToken:  common.HexToAddress(currentValset.RewardToken),
	}

	txData, err := peggyABI.Pack("submitBatch",
		currentValsetArs,
		sigs.v, sigs.r, sigs.s,
		amounts,
		destinations,
		fees,
		batchNonce,
		common.HexToAddress(batch.TokenContract),
		batchTimeout,
	)
	if err != nil {
		s.logger.Err(err).Msg("ABI Pack (Peggy submitBatch) method")
		return nil, err
	}

	return txData, nil
}

func (s *peggyContract) SendTransactionBatch(
	ctx context.Context,
	txData []byte,
) (*common.Hash, error) {
	txHash, err := s.SendTx(ctx, s.peggyAddress, txData)
	if err != nil {
		s.logger.Err(err).Str("tx_hash", txHash.Hex()).Msg("failed to sign and submit (Peggy submitBatch) to EVM")
		return nil, err
	}

	s.logger.Info().Str("tx_hash", txHash.Hex()).Msg("sent Tx (Peggy submitBatch)")

	return &txHash, nil
}

func getBatchCheckpointValues(batch *types.OutgoingTxBatch) (
	amounts []*big.Int,
	destinations []common.Address,
	fees []*big.Int,
) {
	amounts = make([]*big.Int, len(batch.Transactions))
	destinations = make([]common.Address, len(batch.Transactions))
	fees = make([]*big.Int, len(batch.Transactions))

	for i, tx := range batch.Transactions {
		amounts[i] = tx.Erc20Token.Amount.BigInt()
		destinations[i] = common.HexToAddress(tx.DestAddress)
		fees[i] = tx.Erc20Fee.Amount.BigInt()
	}

	return
}

// CheckBatchSigsAndRepack checks all the signatures for a batch (confirmations), assembles them into the expected
// format and checks if the power of the signatures would be enough to send this batch to Ethereum.
func CheckBatchSigsAndRepack(valset *types.Valset, confirms []*types.MsgConfirmBatch) (*RepackedBatchSigs, error) {
	var err error

	if len(confirms) == 0 {
		err = errors.New("no signatures in batch confirmation")
		return nil, err
	}

	sigs := &RepackedBatchSigs{}

	signerToSig := make(map[string]*types.MsgConfirmBatch, len(confirms))
	for _, sig := range confirms {
		signerToSig[sig.EthSigner] = sig
	}

	powerOfGoodSigs := new(big.Int)

	for _, m := range valset.Members {
		mPower := big.NewInt(0).SetUint64(m.Power)
		if sig, ok := signerToSig[m.EthereumAddress]; ok && sig.EthSigner == m.EthereumAddress {
			powerOfGoodSigs.Add(powerOfGoodSigs, mPower)

			sigs.validators = append(sigs.validators, common.HexToAddress(m.EthereumAddress))
			sigs.powers = append(sigs.powers, mPower)

			sigV, sigR, sigS := sigToVRS(sig.Signature)
			sigs.v = append(sigs.v, sigV)
			sigs.r = append(sigs.r, sigR)
			sigs.s = append(sigs.s, sigS)
		} else {
			sigs.validators = append(sigs.validators, common.HexToAddress(m.EthereumAddress))
			sigs.powers = append(sigs.powers, mPower)
			sigs.v = append(sigs.v, 0)
			sigs.r = append(sigs.r, [32]byte{})
			sigs.s = append(sigs.s, [32]byte{})
		}
	}
	if peggyPowerToPercent(powerOfGoodSigs) < 66 {
		err = ErrInsufficientVotingPowerToPass
		return sigs, err
	}

	return sigs, err
}
