package gravity

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// EncodeValsetConfirm takes the required input data and produces the required
// signature to confirm a validator set update on the Gravity Ethereum contract.
// This value will then be signed before being submitted to Cosmos, verified,
// and then relayed to Ethereum.
func EncodeValsetConfirm(gravityID string, valset types.Valset) ethcmn.Hash {
	// error case here should not occur outside of testing since the above is a constant
	contractAbi, err := abi.JSON(strings.NewReader(types.ValsetCheckpointABIJSON))
	if err != nil {
		panic(fmt.Sprintf("failed to JSON parse ABI: %s", err))
	}

	checkpointBytes := []uint8("checkpoint")
	var checkpoint [32]uint8
	copy(checkpoint[:], checkpointBytes)

	gravityIDBytes := []uint8(gravityID)
	var gravityIDBytes32 [32]uint8
	copy(gravityIDBytes32[:], gravityIDBytes)

	memberAddresses := make([]ethcmn.Address, len(valset.Members))
	convertedPowers := make([]*big.Int, len(valset.Members))
	for i, m := range valset.Members {
		memberAddresses[i] = ethcmn.HexToAddress(m.EthereumAddress)
		convertedPowers[i] = big.NewInt(int64(m.Power))
	}

	rewardToken := ethcmn.HexToAddress(valset.RewardToken)

	if valset.RewardAmount.BigInt() == nil {
		// this must be programmer error
		panic("invalid reward amount passed in valset GetCheckpoint!")
	}

	rewardAmount := valset.RewardAmount.BigInt()

	// The word 'checkpoint' needs to be the same as the 'name' above in the
	// checkpointAbiJson but other than that it's a constant that has no impact on
	// the output. This is because it gets encoded as a function name which we must
	// then discard.
	bytes, err := contractAbi.Pack(
		"checkpoint",
		gravityIDBytes32,
		checkpoint,
		big.NewInt(int64(valset.Nonce)),
		memberAddresses,
		convertedPowers,
		rewardAmount,
		rewardToken,
	)
	if err != nil {
		// This should never happen outside of test since any case that could crash
		// on encoding should be filtered above.
		panic(fmt.Sprintf("error packing checkpoint: %s", err))
	}

	// We hash the resulting encoded bytes discarding the first 4 bytes these 4
	// bytes are the constant method name 'checkpoint'. If you where to replace
	// the checkpoint constant in this code you would then need to adjust how many
	// bytes you truncate off the front to get the output of abi.encode().
	hash := crypto.Keccak256Hash(bytes[4:])
	return hash
}

// EncodeTxBatchConfirm takes the required input data and produces the required
// signature to confirm a transaction batch on the Gravity Ethereum contract.
// This value will then be signed before being submitted to Cosmos, verified,
// and then relayed to Ethereum.
func EncodeTxBatchConfirm(gravityID string, batch types.OutgoingTxBatch) ethcmn.Hash {
	abi, err := abi.JSON(strings.NewReader(types.OutgoingBatchTxCheckpointABIJSON))
	if err != nil {
		panic(fmt.Sprintf("failed to JSON parse ABI: %s", err))
	}

	// Create the methodName argument which salts the signature
	methodNameBytes := []uint8("transactionBatch")
	var batchMethodName [32]uint8
	copy(batchMethodName[:], methodNameBytes)

	gravityIDBytes := []uint8(gravityID)
	var gravityIDBytes32 [32]uint8
	copy(gravityIDBytes32[:], gravityIDBytes)

	// Run through the elements of the batch and serialize them
	txAmounts := make([]*big.Int, len(batch.Transactions))
	txDestinations := make([]ethcmn.Address, len(batch.Transactions))
	txFees := make([]*big.Int, len(batch.Transactions))
	for i, tx := range batch.Transactions {
		txAmounts[i] = tx.Erc20Token.Amount.BigInt()
		txDestinations[i] = ethcmn.HexToAddress(tx.DestAddress)
		txFees[i] = tx.Erc20Fee.Amount.BigInt()
	}

	// The methodName needs to be the same as the 'name' above in the
	// checkpointAbiJson but other than that it's a constant that has no impact on
	// the output. This is because it gets encoded as a function name which we must
	// then discard.
	abiEncodedBatch, err := abi.Pack("submitBatch",
		gravityIDBytes32,
		batchMethodName,
		txAmounts,
		txDestinations,
		txFees,
		big.NewInt(int64(batch.BatchNonce)),
		ethcmn.HexToAddress(batch.TokenContract),
		big.NewInt(int64(batch.BatchTimeout)),
	)
	if err != nil {
		// This should never happen outside of test since any case that could crash on
		// encoding should be filtered above.
		return ethcmn.Hash{}
	}

	hash := crypto.Keccak256Hash(abiEncodedBatch[4:])
	return hash
}
