package orchestrator

import (
	"context"
	"fmt"
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
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

func TestCheckForEvents(t *testing.T) {

	t.Run("ok", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

		lastBlock := uint64(95)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(100),
		}, nil)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				// The test data is from a real tx: https://goerli.etherscan.io/tx/0x09310b8dcc615b0baab5c0c41e9e7633f513c23532d0f191509d65e5a28b4ed7#eventlog
				[]ethtypes.Log{
					{
						Address:     gravityAddress,
						Topics:      []ethcmn.Hash{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7"), ethcmn.HexToHash("0x00000000000000000000000053cf531308195be45981e75d1c217a61358f2c27")},
						Data:        hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000378000000000000000000000000000000000000000000000000000000000000000575756d65650000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d6565000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d656500000000000000000000000000000000000000000000000000000000"),
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

		// FilterSendToCosmosEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(95),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x9e9794dbf94b0a0aa31a480f5b38550eda7f89115ac8fbf4953fa4dd219900c9")}, {}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// TransactionBatchExecutedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
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
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x76d08978c024a4bf8cbb30c67fd78fcaa1827cbc533e4e175f36d07e64ccf96a")}, {}},
			})).
			Return(
				[]ethtypes.Log{},
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

		mockCosmos.EXPECT().SyncBroadcastMsg(gomock.Any()).Return(&sdk.TxResponse{}, nil).AnyTimes()

		gravityBroadcastClient := cosmos.NewGravityBroadcastClient(
			logger,
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
			10,
		)

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().LastEventNonceByAddr(gomock.Any(), &types.QueryLastEventNonceByAddrRequest{
			Address: gravityBroadcastClient.AccFromAddress().String(),
		}).Return(&types.QueryLastEventNonceByAddrResponse{
			EventNonce: 1,
		}, nil)

		orch := NewGravityOrchestrator(
			logger,
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
		)

		currentBlock, err := orch.CheckForEvents(context.Background(), 1, 5)
		assert.Nil(t, err)
		assert.Equal(t, uint64(lastBlock), currentBlock)
	})

	t.Run("error on FilterERC20DeployedEvent", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

		lastBlock := uint64(95)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(100),
		}, nil)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				nil,
				errors.New("some error"),
			).Times(1)

		ethCommitter, _ := committer.NewEthCommitter(
			logger,
			fromAddress,
			1.0,
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

		mockQClient := mocks.NewMockQueryClient(mockCtrl)

		orch := NewGravityOrchestrator(
			logger,
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
		)

		currentBlock, err := orch.CheckForEvents(context.Background(), 1, 5)
		assert.EqualError(t, err, "failed to scan past ERC20Deployed events from Ethereum: some error")
		assert.Equal(t, uint64(0), currentBlock)
	})

	t.Run("error on FilterSendToCosmosEvent", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

		lastBlock := uint64(95)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(100),
		}, nil)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				// The test data is from a real tx: https://goerli.etherscan.io/tx/0x09310b8dcc615b0baab5c0c41e9e7633f513c23532d0f191509d65e5a28b4ed7#eventlog
				[]ethtypes.Log{
					{
						Address:     gravityAddress,
						Topics:      []ethcmn.Hash{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7"), ethcmn.HexToHash("0x00000000000000000000000053cf531308195be45981e75d1c217a61358f2c27")},
						Data:        hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000378000000000000000000000000000000000000000000000000000000000000000575756d65650000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d6565000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d656500000000000000000000000000000000000000000000000000000000"),
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

		// FilterSendToCosmosEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(95),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x9e9794dbf94b0a0aa31a480f5b38550eda7f89115ac8fbf4953fa4dd219900c9")}, {}, {}},
			})).
			Return(
				nil,
				errors.New("some error"),
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

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		orch := NewGravityOrchestrator(
			logger,
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
		)

		currentBlock, err := orch.CheckForEvents(context.Background(), 1, 5)
		assert.EqualError(t, err, "failed to scan past SendToCosmos events from Ethereum: some error")
		assert.Equal(t, uint64(0), currentBlock)
	})

	t.Run("error on FilterTransactionBatchExecutedEvent", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

		lastBlock := uint64(95)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(100),
		}, nil)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				// The test data is from a real tx: https://goerli.etherscan.io/tx/0x09310b8dcc615b0baab5c0c41e9e7633f513c23532d0f191509d65e5a28b4ed7#eventlog
				[]ethtypes.Log{
					{
						Address:     gravityAddress,
						Topics:      []ethcmn.Hash{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7"), ethcmn.HexToHash("0x00000000000000000000000053cf531308195be45981e75d1c217a61358f2c27")},
						Data:        hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000378000000000000000000000000000000000000000000000000000000000000000575756d65650000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d6565000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d656500000000000000000000000000000000000000000000000000000000"),
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

		// FilterSendToCosmosEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(95),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x9e9794dbf94b0a0aa31a480f5b38550eda7f89115ac8fbf4953fa4dd219900c9")}, {}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// TransactionBatchExecutedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x02c7e81975f8edb86e2a0c038b7b86a49c744236abf0f6177ff5afc6986ab708")}, {}, {}},
			})).
			Return(
				nil,
				errors.New("some error"),
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

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		orch := NewGravityOrchestrator(
			logger,
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
		)

		currentBlock, err := orch.CheckForEvents(context.Background(), 1, 5)
		assert.EqualError(t, err, "failed to scan past TransactionBatchExecuted events from Ethereum: some error")
		assert.Equal(t, uint64(0), currentBlock)
	})

	t.Run("error on FilterValsetUpdatedEvent", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})

		lastBlock := uint64(95)

		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(100),
		}, nil)

		// FilterERC20DeployedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7")}, {}},
			})).
			Return(
				// The test data is from a real tx: https://goerli.etherscan.io/tx/0x09310b8dcc615b0baab5c0c41e9e7633f513c23532d0f191509d65e5a28b4ed7#eventlog
				[]ethtypes.Log{
					{
						Address:     gravityAddress,
						Topics:      []ethcmn.Hash{ethcmn.HexToHash("0x82fe3a4fa49c6382d0c085746698ddbbafe6c2bf61285b19410644b5b26287c7"), ethcmn.HexToHash("0x00000000000000000000000053cf531308195be45981e75d1c217a61358f2c27")},
						Data:        hexutil.MustDecode("0x00000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000012000000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000378000000000000000000000000000000000000000000000000000000000000000575756d65650000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d6565000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004756d656500000000000000000000000000000000000000000000000000000000"),
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

		// FilterSendToCosmosEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(95),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x9e9794dbf94b0a0aa31a480f5b38550eda7f89115ac8fbf4953fa4dd219900c9")}, {}, {}},
			})).
			Return(
				[]ethtypes.Log{},
				nil,
			).Times(1)

		// TransactionBatchExecutedEvent
		ethProvider.EXPECT().FilterLogs(
			gomock.Any(),
			MatchFilterQuery(ethereum.FilterQuery{
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
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
				FromBlock: new(big.Int).SetUint64(1),
				ToBlock:   new(big.Int).SetUint64(lastBlock),
				Addresses: []ethcmn.Address{gravityAddress},
				Topics:    [][]ethcmn.Hash{{ethcmn.HexToHash("0x76d08978c024a4bf8cbb30c67fd78fcaa1827cbc533e4e175f36d07e64ccf96a")}, {}},
			})).
			Return(
				nil,
				errors.New("some error"),
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

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		orch := NewGravityOrchestrator(
			logger,
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
		)

		currentBlock, err := orch.CheckForEvents(context.Background(), 1, 5)
		assert.EqualError(t, err, "failed to scan past ValsetUpdatedEvent events from Ethereum: some error")
		assert.Equal(t, uint64(0), currentBlock)
	})
}

func TestFilterSendToCosmosEventsByNonce(t *testing.T) {
	// In testEv we'll add 2 valid and 1 past event.
	// This should result in only 2 events after the filter.
	testEv := []*wrappers.GravitySendToCosmosEvent{
		{EventNonce: big.NewInt(3)},
		{EventNonce: big.NewInt(4)},
		{EventNonce: big.NewInt(5)},
	}
	nonce := uint64(3)

	assert.Len(t, filterSendToCosmosEventsByNonce(testEv, nonce), 2)
}

func TestFilterTransactionBatchExecutedEventsByNonce(t *testing.T) {
	// In testEv we'll add 2 valid and 1 past event.
	// This should result in only 2 events after the filter.
	testEv := []*wrappers.GravityTransactionBatchExecutedEvent{
		{EventNonce: big.NewInt(3)},
		{EventNonce: big.NewInt(4)},
		{EventNonce: big.NewInt(5)},
	}
	nonce := uint64(3)

	assert.Len(t, filterTransactionBatchExecutedEventsByNonce(testEv, nonce), 2)
}

func TestFilterValsetUpdateEventsByNonce(t *testing.T) {
	// In testEv we'll add 2 valid and 1 past event.
	// This should result in only 2 events after the filter.
	testEv := []*wrappers.GravityValsetUpdatedEvent{
		{EventNonce: big.NewInt(3)},
		{EventNonce: big.NewInt(4)},
		{EventNonce: big.NewInt(5)},
	}
	nonce := uint64(3)

	assert.Len(t, filterValsetUpdateEventsByNonce(testEv, nonce), 2)
}

func TestFilterERC20DeployedEventsByNonce(t *testing.T) {
	// In testEv we'll add 2 valid and 1 past event.
	// This should result in only 2 events after the filter.
	testEv := []*wrappers.GravityERC20DeployedEvent{
		{EventNonce: big.NewInt(3)},
		{EventNonce: big.NewInt(4)},
		{EventNonce: big.NewInt(5)},
	}
	nonce := uint64(3)

	assert.Len(t, filterERC20DeployedEventsByNonce(testEv, nonce), 2)
}

func TestIsUnknownBlockErr(t *testing.T) {
	gethErr := errors.New("unknown block")
	assert.True(t, isUnknownBlockErr(gethErr))

	parityErr := errors.New("One of the blocks specified in filter...")
	assert.True(t, isUnknownBlockErr(parityErr))

	otherErr := errors.New("other error")
	assert.False(t, isUnknownBlockErr(otherErr))
}

type matchFilterQuery struct {
	q ethereum.FilterQuery
}

func (m *matchFilterQuery) Matches(input interface{}) bool {
	q, ok := input.(ethereum.FilterQuery)
	if ok {

		if q.BlockHash != m.q.BlockHash {
			return false
		}

		if q.FromBlock.Int64() != m.q.FromBlock.Int64() {
			return false
		}

		if q.ToBlock.Int64() != m.q.ToBlock.Int64() {
			return false
		}

		if !assert.ObjectsAreEqual(q.Addresses, m.q.Addresses) {
			return false
		}

		// Comparing 2 slices of slices seems to be a bit tricky.

		if len(q.Topics) != len(m.q.Topics) {
			return false
		}

		for i := range q.Topics {
			if len(q.Topics[i]) != len(m.q.Topics[i]) {
				return false
			}

			for j := range q.Topics[i] {
				if q.Topics[i][j] != m.q.Topics[i][j] {
					return false
				}
			}
		}
		return true
	}

	return false
}

func (m *matchFilterQuery) String() string {
	return fmt.Sprintf("is equal to %v (%T)", m.q, m.q)
}

func MatchFilterQuery(q ethereum.FilterQuery) gomock.Matcher {
	return &matchFilterQuery{q: q}
}
