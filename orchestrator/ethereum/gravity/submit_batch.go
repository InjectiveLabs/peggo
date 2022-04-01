package gravity

import (
	"context"
	"math/big"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

type RepackedSigs struct {
	validators []ethcmn.Address
	powers     []*big.Int
	v          []uint8
	r          []ethcmn.Hash
	s          []ethcmn.Hash
}

// genericConfirm exists only to aid the check and repacking of signatures.
// This way both ValsetUpdates and Batch's signatures can be checked and repacked in the same function.
type genericConfirm struct {
	EthSigner string
	Signature string
}

func (s *gravityContract) EncodeTransactionBatch(
	ctx context.Context,
	currentValset types.Valset,
	batch types.OutgoingTxBatch,
	confirms []types.MsgConfirmBatch,
) ([]byte, error) {

	sigs, err := checkBatchSigsAndRepack(currentValset, confirms)
	if err != nil {
		s.logger.Debug().
			AnErr("err", err).
			Msg("confirmations check failed")
		return nil, nil
	}

	amounts, destinations, fees := getBatchCheckpointValues(batch)
	currentValsetNonce := new(big.Int).SetUint64(currentValset.Nonce)
	batchNonce := new(big.Int).SetUint64(batch.BatchNonce)
	batchTimeout := new(big.Int).SetUint64(batch.BatchTimeout)

	currentValsetArs := wrappers.ValsetArgs{
		Validators:   sigs.validators,
		Powers:       sigs.powers,
		ValsetNonce:  currentValsetNonce,
		RewardAmount: currentValset.RewardAmount.BigInt(),
		RewardToken:  ethcmn.HexToAddress(currentValset.RewardToken),
	}

	sigArray := []wrappers.Signature{}
	for i := range sigs.v {
		sigArray = append(sigArray, wrappers.Signature{
			V: sigs.v[i],
			R: sigs.r[i],
			S: sigs.s[i],
		})
	}

	txData, err := gravityABI.Pack("submitBatch",
		currentValsetArs,
		sigArray,
		amounts,
		destinations,
		fees,
		batchNonce,
		ethcmn.HexToAddress(batch.TokenContract),
		batchTimeout,
	)
	if err != nil {
		s.logger.Err(err).Msg("ABI Pack (Gravity submitBatch) method")
		return nil, err
	}

	return txData, nil
}

func getBatchCheckpointValues(batch types.OutgoingTxBatch) (
	amounts []*big.Int,
	destinations []ethcmn.Address,
	fees []*big.Int,
) {
	amounts = make([]*big.Int, len(batch.Transactions))
	destinations = make([]ethcmn.Address, len(batch.Transactions))
	fees = make([]*big.Int, len(batch.Transactions))

	for i, tx := range batch.Transactions {
		amounts[i] = tx.Erc20Token.Amount.BigInt()
		destinations[i] = ethcmn.HexToAddress(tx.DestAddress)
		fees[i] = tx.Erc20Fee.Amount.BigInt()
	}

	return
}

// checkBatchSigsAndRepack checks all the signatures for a batch (confirmations), assembles them into the expected
// format and checks if the power of the signatures would be enough to send this batch to Ethereum.
func checkBatchSigsAndRepack(valset types.Valset, confirms []types.MsgConfirmBatch) (*RepackedSigs, error) {
	if len(confirms) == 0 {
		return nil, errors.New("no signatures in batch confirmation")
	}

	genericConfirms := make([]genericConfirm, len(confirms))
	for i, c := range confirms {
		genericConfirms[i] = genericConfirm{
			EthSigner: c.EthSigner,
			Signature: c.Signature,
		}
	}

	return checkAndRepackSigs(valset, genericConfirms)
}

func checkAndRepackSigs(valset types.Valset, confirms []genericConfirm) (*RepackedSigs, error) {
	var err error

	sigs := &RepackedSigs{}

	signerToSig := make(map[string]genericConfirm, len(confirms))
	for _, sig := range confirms {
		signerToSig[sig.EthSigner] = sig
	}

	powerOfGoodSigs := new(big.Int)

	for _, m := range valset.Members {
		mPower := big.NewInt(0).SetUint64(m.Power)
		if sig, ok := signerToSig[m.EthereumAddress]; ok && sig.EthSigner == m.EthereumAddress {
			powerOfGoodSigs.Add(powerOfGoodSigs, mPower)

			sigs.validators = append(sigs.validators, ethcmn.HexToAddress(m.EthereumAddress))
			sigs.powers = append(sigs.powers, mPower)

			sigV, sigR, sigS := sigToVRS(sig.Signature)
			sigs.v = append(sigs.v, sigV)
			sigs.r = append(sigs.r, sigR)
			sigs.s = append(sigs.s, sigS)
		} else {
			sigs.validators = append(sigs.validators, ethcmn.HexToAddress(m.EthereumAddress))
			sigs.powers = append(sigs.powers, mPower)
			sigs.v = append(sigs.v, 0)
			sigs.r = append(sigs.r, [32]byte{})
			sigs.s = append(sigs.s, [32]byte{})
		}
	}

	if isEnoughPower(powerOfGoodSigs) {
		return sigs, err
	}

	err = ErrInsufficientVotingPowerToPass
	return sigs, err
}
