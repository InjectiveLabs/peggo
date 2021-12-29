package relayer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	peggyMocks "github.com/umee-network/peggo/mocks/peggy"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestIsBatchProfitable(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	peggyAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
	fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
	erc20Address := ethcmn.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")

	ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil)
	ethProvider.EXPECT().
		CallContract(
			gomock.Any(),
			ethereum.CallMsg{
				From: fromAddress,
				To:   &erc20Address,
				Data: hexutil.MustDecode("0x313ce567"),
			},
			nil,
		).
		Return(
			hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000006"),
			nil,
		).AnyTimes()

	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		fromAddress,
		1.0,
		1.0,
		nil,
		ethProvider,
	)

	peggyContract, _ := peggy.NewPeggyContract(logger, ethCommitter, peggyAddress, nil)

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("contract_addresses") != "" {
			fmt.Fprint(w, `{"0xdac17f958d2ee523a2206206994597c13d831ec7":{"usd":0.998233}}`)
		}
		fmt.Fprint(w, `{"ethereum": {"usd": 4271.57}}`)
	}))
	defer svr.Close()
	coingeckoFeed := coingecko.NewCoingeckoPriceFeed(logger, 100, &coingecko.Config{BaseURL: svr.URL})

	relayer := peggyRelayer{
		peggyContract:    peggyContract,
		priceFeeder:      coingeckoFeed,
		profitMultiplier: 1.1,
	}

	isProfitable := relayer.IsBatchProfitable(
		context.Background(),
		&types.OutgoingTxBatch{
			TokenContract: erc20Address.Hex(),
			Transactions: []*types.OutgoingTransferTx{
				{
					DestAddress: ethcmn.HexToAddress("0x2").Hex(),
					Erc20Token: &types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(10000),
					},
					Erc20Fee: &types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(100),
					},
				},
			},
		},
		99000,
		big.NewInt(100),
		1.1,
	)

	assert.True(t, isProfitable)

	isNotProfitable := relayer.IsBatchProfitable(
		context.Background(),
		&types.OutgoingTxBatch{
			TokenContract: erc20Address.Hex(),
			Transactions: []*types.OutgoingTransferTx{
				{
					DestAddress: ethcmn.HexToAddress("0x2").Hex(),
					Erc20Token: &types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(10000),
					},
					Erc20Fee: &types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(10),
					},
				},
			},
		},
		1000000,
		big.NewInt(100000000000),
		1.5,
	)

	assert.False(t, isNotProfitable)
}

func TestGetBatchesAndSignatures(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockPeggyContract := peggyMocks.NewMockContract(mockCtrl)

		mockQClient.EXPECT().
			OutgoingTxBatches(gomock.Any(), &types.QueryOutgoingTxBatchesRequest{}).
			Return(&types.QueryOutgoingTxBatchesResponse{
				Batches: []*types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
				},
			}, nil)

		mockQClient.EXPECT().BatchConfirms(gomock.Any(), &types.QueryBatchConfirmsRequest{
			Nonce:           11,
			ContractAddress: "0x0",
		}).Return(&types.QueryBatchConfirmsResponse{
			Confirms: []*types.MsgConfirmBatch{
				{
					Nonce:         11,
					TokenContract: "0x0",
					EthSigner:     "0x5",
					Orchestrator:  "",
					Signature:     "0x111",
				},
			},
		}, nil).Times(2)

		mockPeggyContract.EXPECT().
			EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).Times(2)

		relayer := peggyRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			peggyContract:     mockPeggyContract,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), &types.Valset{})
		assert.NoError(t, err)
		assert.Len(t, submittableBatches[ethcmn.HexToAddress("0x0")], 2)

	})

	t.Run("not ready to be relayed, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockPeggyContract := peggyMocks.NewMockContract(mockCtrl)

		mockQClient.EXPECT().
			OutgoingTxBatches(gomock.Any(), &types.QueryOutgoingTxBatchesRequest{}).
			Return(&types.QueryOutgoingTxBatchesResponse{
				Batches: []*types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
				},
			}, nil)

		mockQClient.EXPECT().BatchConfirms(gomock.Any(), &types.QueryBatchConfirmsRequest{
			Nonce:           11,
			ContractAddress: "0x0",
		}).Return(&types.QueryBatchConfirmsResponse{
			Confirms: []*types.MsgConfirmBatch{
				{
					Nonce:         11,
					TokenContract: "0x0",
					EthSigner:     "0x5",
					Orchestrator:  "",
					Signature:     "0x111",
				},
			},
		}, nil).Times(2)

		mockPeggyContract.EXPECT().
			EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("not enough signatures")).Times(2)

		relayer := peggyRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			peggyContract:     mockPeggyContract,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), &types.Valset{})
		assert.NoError(t, err)
		assert.Len(t, submittableBatches[ethcmn.HexToAddress("0x0")], 0)

	})

	t.Run("not ready to be relayed, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)

		mockQClient.EXPECT().
			OutgoingTxBatches(gomock.Any(), &types.QueryOutgoingTxBatchesRequest{}).
			Return(&types.QueryOutgoingTxBatchesResponse{
				Batches: []*types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []*types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: &types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10),
								},
							},
						},
						TokenContract: "0x0",
						Block:         0,
					},
				},
			}, nil)

		mockQClient.EXPECT().BatchConfirms(gomock.Any(), &types.QueryBatchConfirmsRequest{
			Nonce:           11,
			ContractAddress: "0x0",
		}).Return(nil, nil).Times(2)

		relayer := peggyRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), &types.Valset{})
		assert.NoError(t, err)
		assert.Len(t, submittableBatches[ethcmn.HexToAddress("0x0")], 0)

	})

}

func TestRelayBatches(t *testing.T) {

	t.Run("not ready to be relayed, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		mockPeggyContract := peggyMocks.NewMockContract(mockCtrl)

		peggyAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(112),
		}, nil)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil).AnyTimes()

		mockPeggyContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockPeggyContract.EXPECT().GetTxBatchNonce(gomock.Any(), gomock.Any(), gomock.Any()).Return(big.NewInt(1), nil)
		mockPeggyContract.EXPECT().EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte{}, nil)
		mockPeggyContract.EXPECT().Address().Return(peggyAddress).AnyTimes()
		mockPeggyContract.EXPECT().EstimateGas(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(99999), big.NewInt(1), nil)
		mockPeggyContract.EXPECT().IsPendingTxInput(gomock.Any(), gomock.Any()).Return(false)

		mockPeggyContract.EXPECT().SendTx(
			gomock.Any(),
			peggyAddress,
			[]byte{},
			uint64(99999),
			big.NewInt(1),
		).Return(ethcmn.HexToHash("0x01010101"), nil)

		relayer := peggyRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			peggyContract:     mockPeggyContract,
			ethProvider:       ethProvider,
		}

		possibleBatches := map[ethcmn.Address][]SubmittableBatch{
			ethcmn.HexToAddress("0x0"): {
				{
					Batch: &types.OutgoingTxBatch{
						BatchTimeout: 113,
						BatchNonce:   2,
					},
					Signatures: []*types.MsgConfirmBatch{},
				},
			},
		}

		err := relayer.RelayBatches(context.Background(), &types.Valset{}, possibleBatches)
		assert.NoError(t, err)
		assert.Equal(t, uint64(2), relayer.lastSentBatchNonce)
	})

	t.Run("batch timeout, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		mockPeggyContract := peggyMocks.NewMockContract(mockCtrl)

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(112),
		}, nil)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil).AnyTimes()

		mockPeggyContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockPeggyContract.EXPECT().GetTxBatchNonce(gomock.Any(), gomock.Any(), gomock.Any()).Return(big.NewInt(1), nil)

		relayer := peggyRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			peggyContract:     mockPeggyContract,
			ethProvider:       ethProvider,
		}

		possibleBatches := map[ethcmn.Address][]SubmittableBatch{
			ethcmn.HexToAddress("0x0"): {
				{
					Batch: &types.OutgoingTxBatch{
						BatchTimeout: 100,
						BatchNonce:   2,
					},
					Signatures: []*types.MsgConfirmBatch{},
				},
			},
		}

		err := relayer.RelayBatches(context.Background(), &types.Valset{}, possibleBatches)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), relayer.lastSentBatchNonce)
	})
}
