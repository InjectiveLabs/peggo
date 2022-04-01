package gravity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"os"
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

func TestEncodeValsetUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)

	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	valset := types.Valset{
		Nonce:  1,
		Height: 1111,
		Members: []types.BridgeValidator{
			{
				EthereumAddress: ethcmn.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x2").Hex(),
				Power:           123456,
			},
		},
		RewardAmount: sdk.NewInt(0),
	}

	newValset := types.Valset{
		Nonce:  2,
		Height: 2222,
		Members: []types.BridgeValidator{
			{
				EthereumAddress: ethcmn.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
		},
		RewardAmount: sdk.NewInt(0),
	}

	confirms := []types.MsgValsetConfirm{
		{
			EthAddress: ethcmn.HexToAddress("0x0").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthAddress: ethcmn.HexToAddress("0x1").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	ethGravity, _ := wrappers.NewGravity(ethcmn.Address{}, ethCommitter.Provider())

	gravityContract, _ := NewGravityContract(logger, ethCommitter, ethcmn.Address{}, ethGravity)
	txData, err := gravityContract.EncodeValsetUpdate(
		context.Background(),
		valset,
		newValset,
		confirms,
	)

	assert.Nil(t, err)

	// Let's check the hash of the TX data instead of the entire thing
	txDataHash := sha256.Sum256(txData)
	assert.Equal(t, "f22e880adca043d34ea5af87cb0024e28c0cc0b5767b478ec6eb949705765015", hex.EncodeToString(txDataHash[:]))

}

func TestValidatorsAndPowers(t *testing.T) {
	valset := types.Valset{
		Members: []types.BridgeValidator{
			{
				EthereumAddress: ethcmn.HexToAddress("0x0").Hex(),
				Power:           123456,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x1").Hex(),
				Power:           7891011,
			},
		},
	}
	validators, powers := validatorsAndPowers(valset)

	expectedValidators := []ethcmn.Address{
		ethcmn.HexToAddress("0x0"),
		ethcmn.HexToAddress("0x1"),
	}

	expectedPowers := []*big.Int{
		big.NewInt(123456),
		big.NewInt(7891011),
	}

	assert.Equal(t, expectedValidators, validators)
	assert.Equal(t, expectedPowers, powers)

}

func TestCheckValsetSigsAndRepack(t *testing.T) {
	valset := types.Valset{
		Members: []types.BridgeValidator{
			{
				EthereumAddress: ethcmn.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
			{
				EthereumAddress: ethcmn.HexToAddress("0x2").Hex(),
				Power:           123456,
			},
		},
	}

	confirms := []types.MsgValsetConfirm{
		{
			EthAddress: ethcmn.HexToAddress("0x0").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthAddress: ethcmn.HexToAddress("0x1").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	repackedSigs, err := checkValsetSigsAndRepack(valset, confirms)
	assert.Nil(t, err)

	assert.Equal(t, []ethcmn.Address{ethcmn.HexToAddress("0x0"), ethcmn.HexToAddress("0x1"), ethcmn.HexToAddress("0x2")}, repackedSigs.validators)
	assert.Equal(t, []*big.Int{big.NewInt(1111111111), big.NewInt(2212121212), big.NewInt(123456)}, repackedSigs.powers)

}
