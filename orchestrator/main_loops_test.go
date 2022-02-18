package orchestrator

import (
	"context"
	"testing"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/types"

	"github.com/umee-network/peggo/mocks"
)

func TestERC20ToDenom(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			ERC20ToDenom(gomock.Any(), &types.QueryERC20ToDenomRequest{Erc20: "0x0000000000000000000000000000000000000000"}).
			Return(&types.QueryERC20ToDenomResponse{Denom: "umee"}, nil)

		orch := gravityOrchestrator{cosmosQueryClient: mockQClient}

		denom, err := orch.ERC20ToDenom(context.Background(), ethcmn.HexToAddress("0x0"))

		assert.NoError(t, err)
		assert.Equal(t, "umee", denom)

		// Call it again to get it from the cache
		denom, err = orch.ERC20ToDenom(context.Background(), ethcmn.HexToAddress("0x0"))
		assert.NoError(t, err)
		assert.Equal(t, "umee", denom)

	})

	t.Run("not found", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockQClient := mocks.NewMockQueryClient(mockCtrl)
		mockQClient.EXPECT().
			ERC20ToDenom(gomock.Any(), &types.QueryERC20ToDenomRequest{Erc20: "0x0000000000000000000000000000000000000000"}).
			Return(nil, nil)

		orch := gravityOrchestrator{cosmosQueryClient: mockQClient}

		denom, err := orch.ERC20ToDenom(context.Background(), ethcmn.HexToAddress("0x0"))

		assert.EqualError(t, err, "no denom found for token")
		assert.Equal(t, "", denom)
	})
}

func TestGetEthBlockDelay(t *testing.T) {
	assert.Equal(t, uint64(13), getEthBlockDelay(1))
	assert.Equal(t, uint64(0), getEthBlockDelay(2018))
	assert.Equal(t, uint64(10), getEthBlockDelay(5))
	assert.Equal(t, uint64(13), getEthBlockDelay(1235))
}
