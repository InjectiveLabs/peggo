package relayer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	"github.com/umee-network/peggo/mocks"
	gravityMocks "github.com/umee-network/peggo/mocks/gravity"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	"github.com/umee-network/peggo/orchestrator/ethereum/gravity"
)

type mockOracle struct {
	prices map[string]sdk.Dec
}

func (m mockOracle) GetPrices(baseSymbols ...string) (map[string]sdk.Dec, error) {
	return m.prices, nil
}

func (m mockOracle) GetPrice(baseSymbol string) (sdk.Dec, error) {
	return m.prices[baseSymbol], nil
}

func (m mockOracle) SubscribeSymbols(baseSymbols ...string) error {
	return nil
}

func NewMockOracle() Oracle {
	return mockOracle{
		prices: map[string]sdk.Dec{
			"ETH":  sdk.MustNewDecFromStr("4271.57"),
			"USDT": sdk.MustNewDecFromStr("0.998233"),
		},
	}
}

func TestIsBatchProfitable(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
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

	gravityContract, _ := gravity.NewGravityContract(logger, ethCommitter, gravityAddress, nil)

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.URL.Path, "/coins/ethereum/contract/0xdAC17F958D2ee523a2206206994597C13D831ec7") {
			fmt.Fprint(w, `{"symbol": "usdt"}`)
			return
		}
		if r.URL.Query().Get("contract_addresses") != "" {
			fmt.Fprint(w, `{"0xdac17f958d2ee523a2206206994597c13d831ec7":{"usd":0.998233}}`)
		}
		fmt.Fprint(w, `{"ethereum": {"usd": 4271.57}}`)

	}))
	defer svr.Close()
	coingeckoFeed := coingecko.NewCoingecko(logger, &coingecko.Config{BaseURL: svr.URL})
	mockOracle := NewMockOracle()

	relayer := gravityRelayer{
		gravityContract:  gravityContract,
		symbolRetriever:  coingeckoFeed,
		profitMultiplier: 1.1,
		oracle:           mockOracle,
	}

	isProfitable := relayer.IsBatchProfitable(
		context.Background(),
		types.OutgoingTxBatch{
			TokenContract: erc20Address.Hex(),
			Transactions: []types.OutgoingTransferTx{
				{
					DestAddress: ethcmn.HexToAddress("0x2").Hex(),
					Erc20Token: types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(10000),
					},
					Erc20Fee: types.ERC20Token{
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
		types.OutgoingTxBatch{
			TokenContract: erc20Address.Hex(),
			Transactions: []types.OutgoingTransferTx{
				{
					DestAddress: ethcmn.HexToAddress("0x2").Hex(),
					Erc20Token: types.ERC20Token{
						Contract: erc20Address.Hex(),
						Amount:   sdk.NewInt(10000),
					},
					Erc20Fee: types.ERC20Token{
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
		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		mockQClient.EXPECT().
			OutgoingTxBatches(gomock.Any(), &types.QueryOutgoingTxBatchesRequest{}).
			Return(&types.QueryOutgoingTxBatchesResponse{
				Batches: []types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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
			Confirms: []types.MsgConfirmBatch{
				{
					Nonce:         11,
					TokenContract: "0x0",
					EthSigner:     "0x5",
					Orchestrator:  "",
					Signature:     "0x111",
				},
			},
		}, nil).Times(2)

		mockGravityContract.EXPECT().
			EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).Times(2)

		relayer := gravityRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			gravityContract:   mockGravityContract,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), types.Valset{})
		assert.NoError(t, err)
		assert.Len(t, submittableBatches[ethcmn.HexToAddress("0x0")], 2)

	})

	t.Run("not ready to be relayed, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		mockQClient.EXPECT().
			OutgoingTxBatches(gomock.Any(), &types.QueryOutgoingTxBatchesRequest{}).
			Return(&types.QueryOutgoingTxBatchesResponse{
				Batches: []types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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
			Confirms: []types.MsgConfirmBatch{
				{
					Nonce:         11,
					TokenContract: "0x0",
					EthSigner:     "0x5",
					Orchestrator:  "",
					Signature:     "0x111",
				},
			},
		}, nil).Times(2)

		mockGravityContract.EXPECT().
			EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("not enough signatures")).Times(2)

		relayer := gravityRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			gravityContract:   mockGravityContract,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), types.Valset{})
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
				Batches: []types.OutgoingTxBatch{
					{
						BatchNonce:   11,
						BatchTimeout: 111111,
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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
						Transactions: []types.OutgoingTransferTx{
							{
								Id:          0,
								Sender:      "0x1",
								DestAddress: "0x2",
								Erc20Token: types.ERC20Token{
									Contract: "0x0",
									Amount:   sdk.NewInt(10000),
								},
								Erc20Fee: types.ERC20Token{
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

		relayer := gravityRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
		}

		submittableBatches, err := relayer.getBatchesAndSignatures(context.Background(), types.Valset{})
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
		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		gravityAddress := ethcmn.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(112),
		}, nil)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil).AnyTimes()

		mockGravityContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockGravityContract.EXPECT().GetTxBatchNonce(gomock.Any(), gomock.Any(), gomock.Any()).Return(big.NewInt(1), nil)
		mockGravityContract.EXPECT().EncodeTransactionBatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte{}, nil)
		mockGravityContract.EXPECT().Address().Return(gravityAddress).AnyTimes()
		mockGravityContract.EXPECT().EstimateGas(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(99999), big.NewInt(1), nil)
		mockGravityContract.EXPECT().IsPendingTxInput(gomock.Any(), gomock.Any()).Return(false)

		mockGravityContract.EXPECT().SendTx(
			gomock.Any(),
			gravityAddress,
			[]byte{},
			uint64(99999),
			big.NewInt(1),
		).Return(ethcmn.HexToHash("0x01010101"), nil)

		relayer := gravityRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			gravityContract:   mockGravityContract,
			ethProvider:       ethProvider,
		}

		possibleBatches := map[ethcmn.Address][]SubmittableBatch{
			ethcmn.HexToAddress("0x0"): {
				{
					Batch: types.OutgoingTxBatch{
						BatchTimeout: 113,
						BatchNonce:   2,
					},
					Signatures: []types.MsgConfirmBatch{},
				},
			},
		}

		err := relayer.RelayBatches(context.Background(), types.Valset{}, possibleBatches)
		assert.NoError(t, err)
		assert.Equal(t, uint64(2), relayer.lastSentBatchNonce)
	})

	t.Run("batch timeout, no error", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		ethProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
		mockGravityContract := gravityMocks.NewMockContract(mockCtrl)

		fromAddress := ethcmn.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")

		ethProvider.EXPECT().HeaderByNumber(gomock.Any(), nil).Return(&ethtypes.Header{
			Number: big.NewInt(112),
		}, nil)
		ethProvider.EXPECT().PendingNonceAt(gomock.Any(), fromAddress).Return(uint64(0), nil).AnyTimes()

		mockGravityContract.EXPECT().FromAddress().Return(fromAddress).AnyTimes()
		mockGravityContract.EXPECT().GetTxBatchNonce(gomock.Any(), gomock.Any(), gomock.Any()).Return(big.NewInt(1), nil)

		relayer := gravityRelayer{
			logger:            logger,
			cosmosQueryClient: mockQClient,
			gravityContract:   mockGravityContract,
			ethProvider:       ethProvider,
		}

		possibleBatches := map[ethcmn.Address][]SubmittableBatch{
			ethcmn.HexToAddress("0x0"): {
				{
					Batch: types.OutgoingTxBatch{
						BatchTimeout: 100,
						BatchNonce:   2,
					},
					Signatures: []types.MsgConfirmBatch{},
				},
			},
		}

		err := relayer.RelayBatches(context.Background(), types.Valset{}, possibleBatches)
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), relayer.lastSentBatchNonce)
	})
}
