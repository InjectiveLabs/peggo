package peggy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestEncodeValsetUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)

	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), common.HexToAddress("0x0")).Return(uint64(0), nil)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		common.Address{},
		1.0,
		nil,
		mockEvmProvider,
	)

	valset := &types.Valset{
		Nonce:  1,
		Height: 1111,
		Members: []*types.BridgeValidator{
			{
				EthereumAddress: common.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: common.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
			{
				EthereumAddress: common.HexToAddress("0x2").Hex(),
				Power:           123456,
			},
		},
		RewardAmount: sdk.NewInt(0),
	}

	newValset := &types.Valset{
		Nonce:  2,
		Height: 2222,
		Members: []*types.BridgeValidator{
			{
				EthereumAddress: common.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: common.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
		},
		RewardAmount: sdk.NewInt(0),
	}

	confirms := []*types.MsgValsetConfirm{
		{
			EthAddress: common.HexToAddress("0x0").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthAddress: common.HexToAddress("0x1").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	ethPeggy, _ := wrappers.NewPeggy(common.Address{}, ethCommitter.Provider())

	peggyContract, _ := NewPeggyContract(logger, ethCommitter, common.Address{}, ethPeggy)
	txData, err := peggyContract.EncodeValsetUpdate(
		context.Background(),
		valset,
		newValset,
		confirms,
	)

	assert.Nil(t, err)

	// Let's check the hash of the TX data instead of the entire thing
	txDataHash := sha256.Sum256(txData)
	assert.Equal(t, "9660c5094ac2015c1ff2ce2d6e96a0705307014c5d04b33dc6c59ec861323edc", hex.EncodeToString(txDataHash[:]))

}

func TestValidatorsAndPowers(t *testing.T) {
	valset := &types.Valset{
		Members: []*types.BridgeValidator{
			{
				EthereumAddress: common.HexToAddress("0x0").Hex(),
				Power:           123456,
			},
			{
				EthereumAddress: common.HexToAddress("0x1").Hex(),
				Power:           7891011,
			},
		},
	}
	validators, powers := validatorsAndPowers(valset)

	expectedValidators := []common.Address{
		common.HexToAddress("0x0"),
		common.HexToAddress("0x1"),
	}

	expectedPowers := []*big.Int{
		big.NewInt(123456),
		big.NewInt(7891011),
	}

	assert.Equal(t, expectedValidators, validators)
	assert.Equal(t, expectedPowers, powers)

}

func TestCheckValsetSigsAndRepack(t *testing.T) {
	// TODO: These are not real signatures. Would be cool to use real data here.

	valset := &types.Valset{
		Members: []*types.BridgeValidator{
			{
				EthereumAddress: common.HexToAddress("0x0").Hex(),
				Power:           1111111111,
			},
			{
				EthereumAddress: common.HexToAddress("0x1").Hex(),
				Power:           2212121212,
			},
			{
				EthereumAddress: common.HexToAddress("0x2").Hex(),
				Power:           123456,
			},
		},
	}

	confirms := []*types.MsgValsetConfirm{
		{
			EthAddress: common.HexToAddress("0x0").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthAddress: common.HexToAddress("0x1").Hex(),
			Signature:  "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	repackedSigs, err := checkValsetSigsAndRepack(valset, confirms)
	assert.Nil(t, err)

	assert.Equal(t, []common.Address{common.HexToAddress("0x0"), common.HexToAddress("0x1"), common.HexToAddress("0x2")}, repackedSigs.validators)
	assert.Equal(t, []*big.Int{big.NewInt(1111111111), big.NewInt(2212121212), big.NewInt(123456)}, repackedSigs.powers)

}
