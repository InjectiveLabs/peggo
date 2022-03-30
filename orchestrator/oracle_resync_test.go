package orchestrator

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/types"

	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/cosmos"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
)

func TestGetLastCheckedBlock(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")

		mockQClient := mocks.NewMockQueryClient(mockCtrl)

		mockQClient.EXPECT().LastEventNonceByAddr(gomock.Any(), &types.QueryLastEventNonceByAddrRequest{
			Address: sdk.AccAddress{}.String(),
		}).Return(&types.QueryLastEventNonceByAddrResponse{
			EventNonce: 1,
		}, nil)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(1), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).
			Return(&ethtypes.Header{
				Number: big.NewInt(100),
			}, nil)

			// FilterSendToCosmosEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(0),
				ToBlock:   new(big.Int).SetUint64(100),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x9e9794dbf94b0a0aa31a480f5b38550eda7f89115ac8fbf4953fa4dd219900c9")}, {}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(0),
				ToBlock:   new(big.Int).SetUint64(100),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// TransactionBatchExecutedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(0),
				ToBlock:   new(big.Int).SetUint64(100),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x02c7e81975f8edb86e2a0c038b7b86a49c744236abf0f6177ff5afc6986ab708")}, {}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// FilterValsetUpdatedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(0),
				ToBlock:   new(big.Int).SetUint64(100),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x76d08978c024a4bf8cbb30c67fd78fcaa1827cbc533e4e175f36d07e64ccf96a")}, {}},
			})).
			Return(
				[]ethtypes.Log{
					{
						Address:     gravityAddress,
						Topics:      []ethcmn.Hash{ethcmn.HexToHash("0x76d08978c024a4bf8cbb30c67fd78fcaa1827cbc533e4e175f36d07e64ccf96a"), ethcmn.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")},
						Data:        hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000001000000000000000000000000facf66789dd2fa6d80a36353f900922cb6d990f100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000100000000"),
						BlockNumber: 3,
						TxHash:      ethcmn.HexToHash("0x0"),
						TxIndex:     2,
						BlockHash:   ethcmn.HexToHash("0x0"),
						Index:       1,
						Removed:     false,
					},
				},

				nil,
			).Times(1)

		ethGasPriceAdjustment := 1.0
		ethCommitter, _ := committer.NewEthCommitter(
			logger,
			fromAddress,
			ethGasPriceAdjustment,
			1.0,
			nil,
			ethProvider,
		)

		gravityContract, _ := gravity.NewGravityContract(logger, ethCommitter, gravityAddress, nil)

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()
		mockPersonalSignFn := func(account ethcmn.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		gravityBroadcastClient := cosmos.NewGravityBroadcastClient(
			logger,
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
			10,
		)

		orch := NewGravityOrchestrator(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			mockQClient,
			gravityBroadcastClient,
			gravityContract,
			fromAddress,
			nil,
			nil,
			nil,
			time.Second,
			time.Second,
			time.Second,
			100,
			0,
			nil,
			nil,
		)

		block, err := orch.GetLastCheckedBlock(context.Background(), 0)
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), block)
	})
}
