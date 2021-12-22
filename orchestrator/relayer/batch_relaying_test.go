package relayer

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestIsBatchProfitable(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	peggyAddress := common.HexToAddress("0x3bdf8428734244c9e5d82c95d125081939d6d42d")
	fromAddress := common.HexToAddress("0xd8da6bf26964af9d7eed9e03e53415d37aa96045")
	erc20Address := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")

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
					DestAddress: common.HexToAddress("0x2").Hex(),
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
					DestAddress: common.HexToAddress("0x2").Hex(),
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
