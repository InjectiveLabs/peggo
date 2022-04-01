package relayer

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/umee-network/peggo/mocks"
	gravityMocks "github.com/umee-network/peggo/mocks/gravity"
)

func TestRelayValsets(t *testing.T) {
	t.Run("ok", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(&types.QueryLastValsetRequestsResponse{
				Valsets: []types.Valset{
					{
						Nonce: 3,
						Members: []types.BridgeValidator{
							{
								Power:           1000,
								EthereumAddress: "0x0000000000000000000000000000000000000000",
							},
							{
								Power:           1000,
								EthereumAddress: "0x1000000000000000000000000000000000000000",
							},
						},
						Height: 0,
					},
				},
			}, nil)

		mockQClient.EXPECT().
			ValsetRequest(gomock.Any(), &types.QueryValsetRequestRequest{Nonce: 3}).
			Return(&types.QueryValsetRequestResponse{
				Valset: &types.Valset{
					Nonce: 3,
					Members: []types.BridgeValidator{
						{
							Power:           1000,
							EthereumAddress: "0x0000000000000000000000000000000000000000",
						},
						{
							Power:           1000,
							EthereumAddress: "0x1000000000000000000000000000000000000000",
						},
					},
					Height: 0,
				},
			}, nil)

		mockQClient.EXPECT().ValsetConfirmsByNonce(
			gomock.Any(),
			&types.QueryValsetConfirmsByNonceRequest{
				Nonce: 3,
			}).Return(&types.QueryValsetConfirmsByNonceResponse{
			Confirms: []types.MsgValsetConfirm{
				{
					Nonce:        0,
					Orchestrator: "aaa",
					EthAddress:   "0x0000000000000000000000000000000000000000",
					Signature:    "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				},
			},
		}, nil)

		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)
		mockGravityContract.EXPECT().GetValsetNonce(gomock.Any(), fromAddress).Return(big.NewInt(2), nil)
		mockGravityContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockGravityContract.EXPECT().
			EncodeValsetUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]byte{1, 2, 3}, nil).Times(2)
		mockGravityContract.EXPECT().Address().Return(gravityAddress).AnyTimes()
		mockGravityContract.EXPECT().
			EstimateGas(gomock.Any(), gravityAddress, []byte{1, 2, 3}).
			Return(uint64(1000), big.NewInt(100), nil)

		mockGravityContract.EXPECT().IsPendingTxInput([]byte{1, 2, 3}, gomock.Any()).Return(false)

		mockGravityContract.EXPECT().SendTx(
			gomock.Any(),
			gravityAddress,
			[]byte{1, 2, 3},
			uint64(1000),
			big.NewInt(100),
		).Return(ethcmn.HexToHash("0x01010101"), nil)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		assert.Nil(t, relayer.RelayValsets(context.Background(), types.Valset{}))
	})

	t.Run("error. no valsets found", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(nil, nil)

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{})
		assert.EqualError(t, err, "no valsets found")
	})

	t.Run("error. failed to fetch latest valsets from cosmos", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(nil, errors.New("some error"))

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{})
		assert.EqualError(t, err, "failed to fetch latest valsets from cosmos: some error")
	})

	t.Run("error. failed to get valset confirms at nonce", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(&types.QueryLastValsetRequestsResponse{
				Valsets: []types.Valset{
					{
						Nonce: 3,
						Members: []types.BridgeValidator{
							{
								Power:           1000,
								EthereumAddress: "0x0000000000000000000000000000000000000000",
							},
							{
								Power:           1000,
								EthereumAddress: "0x1000000000000000000000000000000000000000",
							},
						},
						Height: 0,
					},
				},
			}, nil)

		mockQClient.EXPECT().
			ValsetRequest(gomock.Any(), &types.QueryValsetRequestRequest{Nonce: 3}).
			Return(&types.QueryValsetRequestResponse{
				Valset: &types.Valset{
					Nonce: 3,
					Members: []types.BridgeValidator{
						{
							Power:           1000,
							EthereumAddress: "0x0000000000000000000000000000000000000000",
						},
						{
							Power:           1000,
							EthereumAddress: "0x1000000000000000000000000000000000000000",
						},
					},
					Height: 0,
				},
			}, nil)

		mockQClient.EXPECT().ValsetConfirmsByNonce(
			gomock.Any(),
			&types.QueryValsetConfirmsByNonceRequest{
				Nonce: 3,
			}).Return(&types.QueryValsetConfirmsByNonceResponse{
			Confirms: []types.MsgValsetConfirm{
				{
					Nonce:        0,
					Orchestrator: "aaa",
					EthAddress:   "0x0000000000000000000000000000000000000000",
					Signature:    "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				},
			},
		}, errors.New("some error"))

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{})
		assert.EqualError(t, err, "failed to get valset confirms at nonce 3: some error")
	})

	t.Run("ok. up to date", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(&types.QueryLastValsetRequestsResponse{
				Valsets: []types.Valset{
					{
						Nonce: 3,
						Members: []types.BridgeValidator{
							{
								Power:           1000,
								EthereumAddress: "0x0000000000000000000000000000000000000000",
							},
							{
								Power:           1000,
								EthereumAddress: "0x1000000000000000000000000000000000000000",
							},
						},
						Height: 0,
					},
				},
			}, nil)

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{Nonce: 3})
		assert.Nil(t, err)
	})

	t.Run("no error. no valset confirms found; skip to the previous nonce", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(&types.QueryLastValsetRequestsResponse{
				Valsets: []types.Valset{
					{
						Nonce: 3,
						Members: []types.BridgeValidator{
							{
								Power:           1000,
								EthereumAddress: "0x0000000000000000000000000000000000000000",
							},
							{
								Power:           1000,
								EthereumAddress: "0x1000000000000000000000000000000000000000",
							},
						},
						Height: 0,
					},
				},
			}, nil)

		mockQClient.EXPECT().
			ValsetRequest(gomock.Any(), &types.QueryValsetRequestRequest{Nonce: 3}).
			Return(&types.QueryValsetRequestResponse{
				Valset: &types.Valset{
					Nonce: 3,
					Members: []types.BridgeValidator{
						{
							Power:           1000,
							EthereumAddress: "0x0000000000000000000000000000000000000000",
						},
						{
							Power:           1000,
							EthereumAddress: "0x1000000000000000000000000000000000000000",
						},
					},
					Height: 0,
				},
			}, nil).Times(1)

		mockQClient.EXPECT().
			ValsetRequest(gomock.Any(), &types.QueryValsetRequestRequest{Nonce: 2}).
			Return(&types.QueryValsetRequestResponse{
				Valset: &types.Valset{
					Nonce: 2,
					Members: []types.BridgeValidator{
						{
							Power:           1000,
							EthereumAddress: "0x0000000000000000000000000000000000000000",
						},
						{
							Power:           1000,
							EthereumAddress: "0x1000000000000000000000000000000000000000",
						},
					},
					Height: 0,
				},
			}, nil).Times(1)

		mockQClient.EXPECT().ValsetConfirmsByNonce(
			gomock.Any(),
			&types.QueryValsetConfirmsByNonceRequest{
				Nonce: 3,
			}).Return(nil, nil).Times(1)

		mockQClient.EXPECT().ValsetConfirmsByNonce(
			gomock.Any(),
			&types.QueryValsetConfirmsByNonceRequest{
				Nonce: 2,
			}).Return(&types.QueryValsetConfirmsByNonceResponse{
			Confirms: []types.MsgValsetConfirm{
				{
					Nonce:        2,
					Orchestrator: "aaa",
					EthAddress:   "0x0000000000000000000000000000000000000000",
					Signature:    "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				},
			},
		}, nil).Times(1)

		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		mockGravityContract.EXPECT().GetValsetNonce(gomock.Any(), fromAddress).Return(big.NewInt(1), nil)
		mockGravityContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockGravityContract.EXPECT().Address().Return(gravityAddress).AnyTimes()
		mockGravityContract.EXPECT().
			EncodeValsetUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]byte{1, 2, 3}, nil).Times(2)
		mockGravityContract.EXPECT().
			EstimateGas(gomock.Any(), gravityAddress, []byte{1, 2, 3}).
			Return(uint64(1000), big.NewInt(100), nil)

		mockGravityContract.EXPECT().IsPendingTxInput([]byte{1, 2, 3}, gomock.Any()).Return(false)

		mockGravityContract.EXPECT().SendTx(
			gomock.Any(),
			gravityAddress,
			[]byte{1, 2, 3},
			uint64(1000),
			big.NewInt(100),
		).Return(ethcmn.HexToHash("0x0"), nil)

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{})
		assert.Nil(t, err)
	})

	t.Run("error while sending tx", func(t *testing.T) {

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			LastValsetRequests(gomock.Any(), &types.QueryLastValsetRequestsRequest{}).
			Return(&types.QueryLastValsetRequestsResponse{
				Valsets: []types.Valset{
					{
						Nonce: 3,
						Members: []types.BridgeValidator{
							{
								Power:           1000,
								EthereumAddress: "0x0000000000000000000000000000000000000000",
							},
							{
								Power:           1000,
								EthereumAddress: "0x1000000000000000000000000000000000000000",
							},
						},
						Height: 0,
					},
				},
			}, nil)

		mockQClient.EXPECT().
			ValsetRequest(gomock.Any(), &types.QueryValsetRequestRequest{Nonce: 3}).
			Return(&types.QueryValsetRequestResponse{
				Valset: &types.Valset{
					Nonce: 3,
					Members: []types.BridgeValidator{
						{
							Power:           1000,
							EthereumAddress: "0x0000000000000000000000000000000000000000",
						},
						{
							Power:           1000,
							EthereumAddress: "0x1000000000000000000000000000000000000000",
						},
					},
					Height: 0,
				},
			}, nil)

		mockQClient.EXPECT().ValsetConfirmsByNonce(
			gomock.Any(),
			&types.QueryValsetConfirmsByNonceRequest{
				Nonce: 3,
			}).Return(&types.QueryValsetConfirmsByNonceResponse{
			Confirms: []types.MsgValsetConfirm{
				{
					Nonce:        3,
					Orchestrator: "aaa",
					EthAddress:   "0x0000000000000000000000000000000000000000",
					Signature:    "0x000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				},
			},
		}, nil)

		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)
		mockGravityContract.EXPECT().GetValsetNonce(gomock.Any(), fromAddress).Return(big.NewInt(2), nil)
		mockGravityContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockGravityContract.EXPECT().
			EncodeValsetUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]byte{1, 2, 3}, nil).Times(2)
		mockGravityContract.EXPECT().Address().Return(gravityAddress).AnyTimes()
		mockGravityContract.EXPECT().
			EstimateGas(gomock.Any(), gravityAddress, []byte{1, 2, 3}).
			Return(uint64(1000), big.NewInt(100), nil)

		mockGravityContract.EXPECT().IsPendingTxInput([]byte{1, 2, 3}, gomock.Any()).Return(false)

		mockGravityContract.EXPECT().SendTx(
			gomock.Any(),
			gravityAddress,
			[]byte{1, 2, 3},
			uint64(1000),
			big.NewInt(100),
		).Return(ethcmn.HexToHash("0x0"), errors.New("some error while sending tx"))

		relayer := gravityRelayer{
			gravityContract:   mockGravityContract,
			cosmosQueryClient: mockQClient,
		}

		err := relayer.RelayValsets(context.Background(), types.Valset{})
		assert.EqualError(t, err, "some error while sending tx")
	})

}
