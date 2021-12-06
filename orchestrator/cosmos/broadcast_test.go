package cosmos

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/umee-network/peggo/mocks"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
	"github.com/umee-network/umee/x/peggy/types"
)

func TestSendValsetConfirm(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(nil)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, nil
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendValsetConfirm(context.Background(), common.Address{}, common.Hash{}, &types.Valset{
			RewardAmount: sdk.NewInt(0),
		})

		assert.Nil(t, err)
	})

	t.Run("failed to sign validator address", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendValsetConfirm(context.Background(), common.Address{}, common.Hash{}, &types.Valset{
			RewardAmount: sdk.NewInt(0),
		})

		assert.EqualError(t, err, "failed to sign validator address")
	})

	t.Run("error during broadcast", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(errors.New("some error during broadcast"))
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, nil
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendValsetConfirm(context.Background(), common.Address{}, common.Hash{}, &types.Valset{
			RewardAmount: sdk.NewInt(0),
		})

		assert.EqualError(t, err, "broadcasting MsgValsetConfirm failed: some error during broadcast")
	})

}

func TestSendBatchConfirm(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(nil)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, nil
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendBatchConfirm(context.Background(), common.Address{}, common.Hash{}, &types.OutgoingTxBatch{})

		assert.Nil(t, err)
	})

	t.Run("failed to sign validator address", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, errors.New("some error during signing")
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendBatchConfirm(context.Background(), common.Address{}, common.Hash{}, &types.OutgoingTxBatch{})

		assert.EqualError(t, err, "failed to sign validator address")
	})

	t.Run("error during broadcast", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(errors.New("some error during broadcast"))
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		mockPersonalSignFn := func(account common.Address, data []byte) (sig []byte, err error) {
			return []byte{}, nil
		}

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			mockPersonalSignFn,
		)

		err := s.SendBatchConfirm(context.Background(), common.Address{}, common.Hash{}, &types.OutgoingTxBatch{})

		assert.EqualError(t, err, "broadcasting MsgConfirmBatch failed: some error during broadcast")
	})

}

// Custom matcher for TestSendDepositClaims
type hasBiggerNonce struct {
	currentNonce uint64
}

func (m *hasBiggerNonce) Matches(input interface{}) bool {
	deposit, ok := input.(*types.MsgDepositClaim)
	if ok {
		if deposit.EventNonce > m.currentNonce {
			m.currentNonce = deposit.EventNonce

			return true
		}
		return false
	}

	withdraw, ok := input.(*types.MsgWithdrawClaim)
	if ok {
		if withdraw.EventNonce > m.currentNonce {
			m.currentNonce = withdraw.EventNonce
			return true
		}
	}

	valsetUpdate, ok := input.(*types.MsgValsetUpdatedClaim)
	if ok {
		if valsetUpdate.EventNonce > m.currentNonce {
			m.currentNonce = valsetUpdate.EventNonce
			return true
		}
	}

	erc20Deployed, ok := input.(*types.MsgERC20DeployedClaim)
	if ok {
		if erc20Deployed.EventNonce > m.currentNonce {
			m.currentNonce = erc20Deployed.EventNonce
			return true
		}
	}

	return false
}

func (m *hasBiggerNonce) String() string {
	return "nonce must be higher"
}

func HasBiggerNonce(initialNonce uint64) gomock.Matcher {
	return &hasBiggerNonce{currentNonce: initialNonce}
}

func TestSendEthereumClaims(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
	mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()

	mockCosmos.EXPECT().SyncBroadcastMsg(HasBiggerNonce(0)).Return(&sdk.TxResponse{}, nil).Times(8)

	s := NewPeggyBroadcastClient(
		zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
		nil,
		mockCosmos,
		nil,
		nil,
	)

	deposits := []*wrappers.PeggySendToCosmosEvent{
		{
			EventNonce: big.NewInt(2),
			Amount:     big.NewInt(123),
		},
		{
			EventNonce: big.NewInt(6),
			Amount:     big.NewInt(456),
		},
	}

	withdraws := []*wrappers.PeggyTransactionBatchExecutedEvent{
		{
			EventNonce: big.NewInt(1),
			BatchNonce: big.NewInt(0),
		},
		{
			EventNonce: big.NewInt(3),
			BatchNonce: big.NewInt(0),
		},
	}

	valsetUpdates := []*wrappers.PeggyValsetUpdatedEvent{
		{
			EventNonce:     big.NewInt(4),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
		{
			EventNonce:     big.NewInt(5),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
		{
			EventNonce:     big.NewInt(7),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
	}

	erc20Deployed := []*wrappers.PeggyERC20DeployedEvent{
		{
			EventNonce: big.NewInt(8),
		},
	}

	s.SendEthereumClaims(context.Background(),
		0,
		deposits,
		withdraws,
		valsetUpdates,
		erc20Deployed,
		time.Microsecond,
	)
}

func TestSendEthereumClaimsIgnoreNonSequentialNonces(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
	mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{}).AnyTimes()

	mockCosmos.EXPECT().SyncBroadcastMsg(HasBiggerNonce(0)).Return(&sdk.TxResponse{}, nil).Times(7)

	s := peggyBroadcastClient{
		daemonQueryClient: nil,
		broadcastClient:   mockCosmos,
	}

	// We have events with nonces 1, 2, 3, 4, 5, 6, 7, 9.
	// So we are missing the 8, meaning events above that won't be relayed
	deposits := []*wrappers.PeggySendToCosmosEvent{
		{
			EventNonce: big.NewInt(2),
			Amount:     big.NewInt(123),
		},
		{
			EventNonce: big.NewInt(6),
			Amount:     big.NewInt(456),
		},
	}

	withdraws := []*wrappers.PeggyTransactionBatchExecutedEvent{
		{
			EventNonce: big.NewInt(1),
			BatchNonce: big.NewInt(0),
		},
		{
			EventNonce: big.NewInt(3),
			BatchNonce: big.NewInt(0),
		},
	}

	valsetUpdates := []*wrappers.PeggyValsetUpdatedEvent{
		{
			EventNonce:     big.NewInt(4),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
		{
			EventNonce:     big.NewInt(5),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
		{
			EventNonce:     big.NewInt(9),
			NewValsetNonce: big.NewInt(0),
			RewardAmount:   big.NewInt(0),
		},
	}

	erc20Deployed := []*wrappers.PeggyERC20DeployedEvent{
		{
			EventNonce: big.NewInt(7),
		},
	}

	s.SendEthereumClaims(context.Background(),
		0,
		deposits,
		withdraws,
		valsetUpdates,
		erc20Deployed,
		time.Microsecond,
	)
}

func TestSendRequestBatch(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(nil)
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			nil,
		)

		err := s.SendRequestBatch(context.Background(), "uumee")

		assert.Nil(t, err)
	})

	t.Run("error during broadcast", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockCosmos := mocks.NewMockCosmosClient(mockCtrl)
		mockCosmos.EXPECT().QueueBroadcastMsg(gomock.Any()).Return(errors.New("some error during broadcast"))
		mockCosmos.EXPECT().FromAddress().Return(sdk.AccAddress{})

		s := NewPeggyBroadcastClient(
			zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}),
			nil,
			mockCosmos,
			nil,
			nil,
		)

		err := s.SendRequestBatch(context.Background(), "uumee")

		assert.EqualError(t, err, "broadcasting MsgRequestBatch failed: some error during broadcast")
	})

}
