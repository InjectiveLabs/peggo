package gravity

import (
	"context"
	"math/big"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

func (s *gravityContract) EncodeValsetUpdate(
	ctx context.Context,
	oldValset types.Valset,
	newValset types.Valset,
	confirms []types.MsgValsetConfirm,
) ([]byte, error) {
	if newValset.Nonce <= oldValset.Nonce {
		err := errors.New("new valset nonce should be greater than old valset nonce")
		return nil, err
	}

	s.logger.Info().
		Uint64("old_valset_nonce", oldValset.Nonce).
		Uint64("new_valset_nonce", newValset.Nonce).
		Msg("checking signatures and submitting validator set update to Ethereum")

	newValidators, newPowers := validatorsAndPowers(newValset)
	newValsetNonce := new(big.Int).SetUint64(newValset.Nonce)

	newValsetArgs := wrappers.ValsetArgs{
		Validators:   newValidators,
		Powers:       newPowers,
		ValsetNonce:  newValsetNonce,
		RewardAmount: newValset.RewardAmount.BigInt(),
		RewardToken:  ethcmn.HexToAddress(newValset.RewardToken),
	}

	// we need to use the old valset here because our signatures need to match the current
	// members of the validator set in the contract.
	sigs, err := checkValsetSigsAndRepack(oldValset, confirms)
	if err != nil {
		err = errors.Wrap(err, "confirmations check failed")
		return nil, err
	}
	currentValsetNonce := new(big.Int).SetUint64(oldValset.Nonce)
	currentValsetArgs := wrappers.ValsetArgs{
		Validators:   sigs.validators,
		Powers:       sigs.powers,
		ValsetNonce:  currentValsetNonce,
		RewardAmount: oldValset.RewardAmount.BigInt(),
		RewardToken:  ethcmn.HexToAddress(oldValset.RewardToken),
	}

	s.logger.Debug().
		Interface("current_validators", sigs.validators).
		Interface("current_powers", sigs.powers).
		Interface("current_valset_nonce", currentValsetNonce).
		Msg("sending updateValset Ethereum TX")

	sigArray := []wrappers.Signature{}
	for i := range sigs.v {
		sigArray = append(sigArray, wrappers.Signature{
			V: sigs.v[i],
			R: sigs.r[i],
			S: sigs.s[i],
		})
	}

	txData, err := gravityABI.Pack("updateValset",
		newValsetArgs,
		currentValsetArgs,
		sigArray,
	)

	if err != nil {
		s.logger.Err(err).Msg("ABI Pack (Gravity updateValset) method")
		return nil, err
	}

	return txData, nil
}

func validatorsAndPowers(valset types.Valset) (
	validators []ethcmn.Address,
	powers []*big.Int,
) {
	for _, m := range valset.Members {
		mPower := big.NewInt(0).SetUint64(m.Power)
		validators = append(validators, ethcmn.HexToAddress(m.EthereumAddress))
		powers = append(powers, mPower)
	}

	return
}

func checkValsetSigsAndRepack(valset types.Valset, confirms []types.MsgValsetConfirm) (*RepackedSigs, error) {
	if len(confirms) == 0 {
		return nil, errors.New("no signatures in valset confirmation")
	}

	genericConfirms := make([]genericConfirm, len(confirms))
	for i, c := range confirms {
		genericConfirms[i] = genericConfirm{
			EthSigner: c.EthAddress,
			Signature: c.Signature,
		}
	}

	return checkAndRepackSigs(valset, genericConfirms)
}
