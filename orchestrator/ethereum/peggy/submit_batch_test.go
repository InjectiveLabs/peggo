package peggy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestEncodeTransactionBatch(t *testing.T) {
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

	valset := &types.Valset{
		Nonce:  1,
		Height: 1111,
		Members: []*types.BridgeValidator{
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

	confirms := []*types.MsgConfirmBatch{
		{
			EthSigner: ethcmn.HexToAddress("0x0").Hex(),
			Signature: "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthSigner: ethcmn.HexToAddress("0x1").Hex(),
			Signature: "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	batch := &types.OutgoingTxBatch{
		BatchNonce:    1,
		BatchTimeout:  11111,
		Block:         1234567,
		TokenContract: ethcmn.HexToAddress("0x1").Hex(),
		Transactions: []*types.OutgoingTransferTx{
			{
				DestAddress: ethcmn.HexToAddress("0x2").Hex(),
				Erc20Token: &types.ERC20Token{
					Contract: ethcmn.HexToAddress("0x1").Hex(),
					Amount:   sdk.NewInt(10000),
				},
				Erc20Fee: &types.ERC20Token{
					Contract: ethcmn.HexToAddress("0x1").Hex(),
					Amount:   sdk.NewInt(100),
				},
			},
		},
	}

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)

	txData, err := peggyContract.EncodeTransactionBatch(
		context.Background(),
		valset,
		batch,
		confirms,
	)

	assert.Nil(t, err)

	// Let's check the hash of the TX data instead of the entire thing
	txDataHash := sha256.Sum256(txData)
	assert.Equal(t, "98523ac4d387f9cfd4483a0ba70cfde4f303943d540b27fa6c07b161b556124c", hex.EncodeToString(txDataHash[:]))
}

func TestGetBatchCheckpointValues(t *testing.T) {
	batch := &types.OutgoingTxBatch{
		Transactions: []*types.OutgoingTransferTx{
			{
				DestAddress: ethcmn.HexToAddress("0x2").Hex(),
				Erc20Token: &types.ERC20Token{
					Contract: ethcmn.HexToAddress("0x1").Hex(),
					Amount:   sdk.NewInt(10000),
				},
				Erc20Fee: &types.ERC20Token{
					Contract: ethcmn.HexToAddress("0x1").Hex(),
					Amount:   sdk.NewInt(100),
				},
			},
		},
	}

	amounts, destinations, fees := getBatchCheckpointValues(batch)
	assert.Equal(t, []*big.Int{big.NewInt(10000)}, amounts)
	assert.Equal(t, []ethcmn.Address{ethcmn.HexToAddress("0x2")}, destinations)
	assert.Equal(t, []*big.Int{big.NewInt(100)}, fees)
}

func TestCheckBatchSigsAndRepack(t *testing.T) {
	// TODO: These are not real signatures. Would be cool to use real data here.

	valset := &types.Valset{
		Members: []*types.BridgeValidator{
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

	confirms := []*types.MsgConfirmBatch{
		{
			EthSigner: ethcmn.HexToAddress("0x0").Hex(),
			Signature: "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
		{
			EthSigner: ethcmn.HexToAddress("0x1").Hex(),
			Signature: "0xaae54ee7e285fbb0275279143abc4c554e5314e7b417ecac83a5984a964facbaad68866a2841c3e83ddf125a2985566261c4014f9f960ec60253aebcda9513a9b4",
		},
	}

	repackedSigs, err := checkBatchSigsAndRepack(valset, confirms)
	assert.Nil(t, err)

	assert.Equal(t, []ethcmn.Address{ethcmn.HexToAddress("0x0"), ethcmn.HexToAddress("0x1"), ethcmn.HexToAddress("0x2")}, repackedSigs.validators)
	assert.Equal(t, []*big.Int{big.NewInt(1111111111), big.NewInt(2212121212), big.NewInt(123456)}, repackedSigs.powers)

}
