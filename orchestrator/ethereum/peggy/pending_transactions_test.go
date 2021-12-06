package peggy

import (
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
)

func TestAddPendingTxInput(t *testing.T) {
	txList := PendingTxInputList{}

	// add a submitBatch tx
	txList.AddPendingTxInput(&RPCTransaction{
		Input: hexutil.MustDecode("0x8174741800000000"),
	})

	// add a updateValset tx
	txList.AddPendingTxInput(&RPCTransaction{
		Input: hexutil.MustDecode("0xa5352f5b00000000"),
	})

	// try to add a sendToCosmos tx
	txList.AddPendingTxInput(&RPCTransaction{
		Input: hexutil.MustDecode("0x1ffbe7f900000000"),
	})

	// Only the first 2 TXs should have been added
	assert.Len(t, txList, 2)

	for i := 0; i < 110; i++ {
		txList.AddPendingTxInput(&RPCTransaction{
			Input: hexutil.MustDecode("0x8174741800000000"),
		})
	}

	// The list should be at full capacity now
	assert.Len(t, txList, 100)
}

func TestIsPendingTxInput(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), common.HexToAddress("0x0")).Return(uint64(0), nil)

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
	ethCommitter, _ := committer.NewEthCommitter(
		logger,
		common.Address{},
		1.0,
		nil,
		mockEvmProvider,
	)

	ethPeggy, _ := wrappers.NewPeggy(common.Address{}, ethCommitter.Provider())

	peggyContract, _ := NewPeggyContract(logger, ethCommitter, common.Address{}, ethPeggy)
	peggyContract.IsPendingTxInput([]byte{}, time.Second)

	// Add a TX
	peggyContract.GetPendingTxInputList().AddPendingTxInput(&RPCTransaction{
		Input: hexutil.MustDecode("0xa5352f5b00000000"),
	})

	// Check if the tx is pending (with a generous 1m timeout)
	assert.True(t, peggyContract.IsPendingTxInput(hexutil.MustDecode("0xa5352f5b00000000"), time.Minute))
	time.Sleep(time.Millisecond * 1)

	// Now let's check back again with only a 1µs timeout after having waited 1ms. Should be marked as timed out
	assert.False(t, peggyContract.IsPendingTxInput(hexutil.MustDecode("0xa5352f5b00000000"), time.Microsecond))

}

// TODO: check if we can actually test this. Maybe move the Fatal call to the caller.
// func TestSubscribeToPendingTxs(t *testing.T) {
// 	mockCtrl := gomock.NewController(t)
// 	defer mockCtrl.Finish()
// 	mockEvmProvider := mocks.NewMockEVMProviderWithRet(mockCtrl)
// 	mockEvmProvider.EXPECT().PendingNonceAt(gomock.Any(), common.HexToAddress("0x0")).Return(uint64(0), nil)

// 	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr})
// 	ethCommitter, _ := committer.NewEthCommitter(
// 		logger,
// 		common.Address{},
// 		1.0,
// 		nil,
// 		mockEvmProvider,
// 	)
// 	peggyContract, _ := NewPeggyContract(logger, ethCommitter, common.Address{})

// 	err := peggyContract.SubscribeToPendingTxs(context.Background(), "invalidURL")

// 	assert.NotNil(t, err)

// }
