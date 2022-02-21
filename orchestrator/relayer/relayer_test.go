package relayer

import (
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	gravityMocks "github.com/umee-network/peggo/mocks/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"
)

func TestNewGravityRelayer(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	mockQClient := mocks.NewMockQueryClient(mockCtrl)
	mockGravityContract := gravityMocks.NewMockContract(mockCtrl)
	mockGravityContract.EXPECT().Provider().Return(provider.NewEVMProvider(nil))

	relayer := NewGravityRelayer(logger,
		mockQClient,
		mockGravityContract,
		ValsetRelayModeMinimum,
		true,
		time.Minute,
		time.Minute,
		1.0,
		SetPriceFeeder(nil),
	)

	assert.NotNil(t, relayer)
}
