package peggy

import (
	"context"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
)

func TestPeggyPowerToPercent(t *testing.T) {
	percent := peggyPowerToPercent(big.NewInt(213192100))
	assert.Equal(t, percent, float32(4.9637656))

}

func TestGetTxBatchNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nonceHex := hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000123")
	nonceBigInt := big.NewInt(0).SetBytes(nonceHex)

	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)
	mockEvmProvider.EXPECT().
		CallContract(
			gomock.Any(),
			gomock.AssignableToTypeOf(ethereum.CallMsg{}),
			nil,
		).
		Return(
			nonceHex,
			nil,
		)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)
	nonce, err := peggyContract.GetTxBatchNonce(context.Background(), ethcmn.HexToAddress("0x0"), ethcmn.HexToAddress("0x0"))

	assert.Nil(t, err)
	assert.Equal(t, nonce, nonceBigInt)

}

func TestGetValsetNonce(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	nonceHex := hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000123")
	nonceBigInt := big.NewInt(0).SetBytes(nonceHex)

	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)
	mockEvmProvider.EXPECT().
		CallContract(
			gomock.Any(),
			gomock.AssignableToTypeOf(ethereum.CallMsg{}),
			nil,
		).
		Return(
			nonceHex,
			nil,
		)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)
	nonce, err := peggyContract.GetValsetNonce(context.Background(), ethcmn.HexToAddress("0x0"))

	assert.Nil(t, err)
	assert.Equal(t, nonce, nonceBigInt)

}

func TestGetGetPeggyID(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	peggyID := ethcmn.HexToHash("0x756d65652d706567677969640000000000000000000000000000000000000000")

	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)
	mockEvmProvider.EXPECT().
		CallContract(
			gomock.Any(),
			gomock.AssignableToTypeOf(ethereum.CallMsg{}),
			nil,
		).
		Return(
			peggyID.Bytes(),
			nil,
		)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)
	res, err := peggyContract.GetPeggyID(context.Background(), ethcmn.HexToAddress("0x0"))

	assert.Nil(t, err)
	assert.Equal(t, peggyID, res)

}

func TestGetERC20Symbol(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)

	zeroAddress := ethcmn.HexToAddress("0x0")
	oneAddress := ethcmn.HexToAddress("0x1")

	mockEvmProvider.EXPECT().
		CallContract(
			gomock.Any(),
			ethereum.CallMsg{
				From: zeroAddress,
				To:   &oneAddress,
				Data: hexutil.MustDecode("0x95d89b41"),
			},
			nil,
		).
		Return(
			// This was calculated with https://abi.hashex.org/
			hexutil.MustDecode("0x000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000045553444300000000000000000000000000000000000000000000000000000000"),
			nil,
		).AnyTimes()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)
	symbol, err := peggyContract.GetERC20Symbol(context.Background(), ethcmn.HexToAddress("0x1"), ethcmn.HexToAddress("0x0"))

	assert.Nil(t, err)
	assert.Equal(t, "USDC", symbol)

}

func TestGetERC20Decimals(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), ethcmn.HexToAddress("0x0")).Return(uint64(0), nil)

	zeroAddress := ethcmn.HexToAddress("0x0")
	oneAddress := ethcmn.HexToAddress("0x1")

	mockEvmProvider.EXPECT().
		CallContract(
			gomock.Any(),
			ethereum.CallMsg{
				From: zeroAddress,
				To:   &oneAddress,
				Data: hexutil.MustDecode("0x313ce567"),
			},
			nil,
		).
		Return(
			hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000012"),
			nil,
		).AnyTimes()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		ethcmn.Address{},
		1.0,
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)
	decimals, err := peggyContract.GetERC20Decimals(context.Background(), ethcmn.HexToAddress("0x1"), ethcmn.HexToAddress("0x0"))

	assert.Nil(t, err)
	assert.Equal(t, uint8(18), decimals)

}

func TestAddress(t *testing.T) {
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

	ethPeggy, _ := wrappers.NewPeggy(ethcmn.Address{}, ethCommitter.Provider())
	peggyContract, _ := NewPeggyContract(logger, ethCommitter, ethcmn.Address{}, ethPeggy)

	assert.Equal(t, ethcmn.Address{}, peggyContract.Address())
}
