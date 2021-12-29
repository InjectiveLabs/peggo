package relayer

import (
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	peggyMocks "github.com/umee-network/peggo/mocks/peggy"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"
)

func TestNewPeggyRelayer(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	mockQClient := mocks.NewMockQueryClient(mockCtrl)
	mockPeggyContract := peggyMocks.NewMockContract(mockCtrl)
	mockPeggyContract.EXPECT().Provider().Return(provider.NewEVMProvider(nil))

	relayer := NewPeggyRelayer(logger,
		mockQClient,
		mockPeggyContract,
		true,
		true,
		time.Minute,
		time.Minute,
		1.0,
		SetPriceFeeder(nil),
	)

	assert.NotNil(t, relayer)
}
