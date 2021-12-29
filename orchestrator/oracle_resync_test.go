package orchestrator

import (
	"context"
	"os"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/cosmos"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestGetLastCheckedBlock(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		peggyAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().LastEventByAddr(gomock.Any(), &types.QueryLastEventByAddrRequest{
			Address: sdk.AccAddress{}.String(),
		}).Return(&types.QueryLastEventByAddrResponse{
			LastClaimEvent: &types.LastClaimEvent{
				EthereumEventNonce:  1,
				EthereumEventHeight: 123,
			},
		}, nil)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)

		ethGasPriceAdjustment := 1.0
		ethCommitter, _ := committer.NewEthCommitter(
			logger,
			fromAddress,
			ethGasPriceAdjustment,
			1.0,
			nil,
			ethProvider,
		)

		peggyContract, _ := peggy.NewPeggyContract(logger, ethCommitter, peggyAddress, nil)

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()
		mockPersonalSignFn := func(account ethcmn.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		peggyBroadcastClient := cosmos.NewPeggyBroadcastClient(
			logger,
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		orch := NewPeggyOrchestrator(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			mockQClient,
			peggyBroadcastClient,
			peggyContract,
			fromAddress,
			nil,
			nil,
			nil,
			time.Second,
			time.Second,
			time.Second,
			100,
		)

		block, err := orch.GetLastCheckedBlock(context.Background())
		assert.Nil(t, err)
		assert.Equal(t, uint64(123), block)
	})

	t.Run("error from cosmosQueryClient", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		peggyAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().LastEventByAddr(gomock.Any(), &types.QueryLastEventByAddrRequest{
			Address: sdk.AccAddress{}.String(),
		}).Return(&types.QueryLastEventByAddrResponse{
			LastClaimEvent: &types.LastClaimEvent{
				EthereumEventNonce:  1,
				EthereumEventHeight: 123,
			},
		}, errors.New("some error"))

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)

		ethGasPriceAdjustment := 1.0
		ethCommitter, _ := committer.NewEthCommitter(
			logger,
			fromAddress,
			ethGasPriceAdjustment,
			1.0,
			nil,
			ethProvider,
		)

		peggyContract, _ := peggy.NewPeggyContract(logger, ethCommitter, peggyAddress, nil)

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()
		mockPersonalSignFn := func(account ethcmn.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		peggyBroadcastClient := cosmos.NewPeggyBroadcastClient(
			logger,
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		orch := NewPeggyOrchestrator(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			mockQClient,
			peggyBroadcastClient,
			peggyContract,
			fromAddress,
			nil,
			nil,
			nil,
			time.Second,
			time.Second,
			time.Second,
			100,
		)

		block, err := orch.GetLastCheckedBlock(context.Background())
		assert.EqualError(t, err, "some error")
		assert.Equal(t, uint64(0), block)
	})

	t.Run("error. no response", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		peggyAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().LastEventByAddr(gomock.Any(), &types.QueryLastEventByAddrRequest{
			Address: sdk.AccAddress{}.String(),
		}).Return(nil, nil)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)

		ethGasPriceAdjustment := 1.0
		ethCommitter, _ := committer.NewEthCommitter(
			logger,
			fromAddress,
			ethGasPriceAdjustment,
			1.0,
			nil,
			ethProvider,
		)

		peggyContract, _ := peggy.NewPeggyContract(logger, ethCommitter, peggyAddress, nil)

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()
		mockPersonalSignFn := func(account ethcmn.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		peggyBroadcastClient := cosmos.NewPeggyBroadcastClient(
			logger,
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		orch := NewPeggyOrchestrator(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			mockQClient,
			peggyBroadcastClient,
			peggyContract,
			fromAddress,
			nil,
			nil,
			nil,
			time.Second,
			time.Second,
			time.Second,
			100,
		)

		block, err := orch.GetLastCheckedBlock(context.Background())
		assert.EqualError(t, err, "no last event response returned")
		assert.Equal(t, uint64(0), block)
	})
}
