package gravity

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/umee-network/Gravity-Bridge/module/x/gravity/types"

	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

// gravityPowerToPass is a mirror of constant_powerThreshold in Gravity.sol
const gravityPowerToPass int64 = 2863311530

var (
	gravityABI, _ = abi.JSON(strings.NewReader(wrappers.GravityABI))

	ErrInsufficientVotingPowerToPass = errors.New("insufficient voting power")
)

type Contract interface {
	committer.EVMCommitter

	// Address returns the Gravity contract address
	Address() ethcmn.Address

	// EncodeTransactionBatch encodes a batch into a tx byte data. This is specially helpful for estimating gas and
	// detecting identical transactions in the mempool.
	EncodeTransactionBatch(
		ctx context.Context,
		currentValset types.Valset,
		batch types.OutgoingTxBatch,
		confirms []types.MsgConfirmBatch,
	) ([]byte, error)

	// EncodeValsetUpdate encodes a valset update into a tx byte data. This is specially helpful for estimating gas and
	// detecting identical transactions in the mempool.
	EncodeValsetUpdate(
		ctx context.Context,
		oldValset types.Valset,
		newValset types.Valset,
		confirms []types.MsgValsetConfirm,
	) ([]byte, error)

	GetTxBatchNonce(
		ctx context.Context,
		erc20ContractAddress ethcmn.Address,
		callerAddress ethcmn.Address,
	) (*big.Int, error)

	GetValsetNonce(
		ctx context.Context,
		callerAddress ethcmn.Address,
	) (*big.Int, error)

	GetGravityID(
		ctx context.Context,
		callerAddress ethcmn.Address,
	) (string, error)

	GetERC20Symbol(
		ctx context.Context,
		erc20ContractAddress ethcmn.Address,
		callerAddress ethcmn.Address,
	) (symbol string, err error)

	GetERC20Decimals(
		ctx context.Context,
		erc20ContractAddress ethcmn.Address,
		callerAddress ethcmn.Address,
	) (decimals uint8, err error)

	// SubscribeToPendingTxs starts a websocket connection to Alchemy's service that listens for new pending txs made
	// to the Gravity contract.
	SubscribeToPendingTxs(ctx context.Context, alchemyWebsocketURL string) error

	// IsPendingTxInput returns true if the input data is found in the pending tx list. If the tx is found but the tx is
	// older than pendingTxWaitDuration, we consider it stale and return false, so the validator re-sends it.
	IsPendingTxInput(txData []byte, pendingTxWaitDuration time.Duration) bool

	GetPendingTxInputList() *PendingTxInputList
}

type gravityContract struct {
	committer.EVMCommitter

	logger             zerolog.Logger
	gravityAddress     ethcmn.Address
	ethGravity         *wrappers.Gravity
	pendingTxInputList PendingTxInputList

	mtx               sync.Mutex
	erc20DecimalCache map[string]uint8
}

func NewGravityContract(
	logger zerolog.Logger,
	ethCommitter committer.EVMCommitter,
	gravityAddress ethcmn.Address,
	ethGravity *wrappers.Gravity,
) (Contract, error) {
	return &gravityContract{
		logger:         logger.With().Str("module", "gravity_contract").Logger(),
		EVMCommitter:   ethCommitter,
		gravityAddress: gravityAddress,
		ethGravity:     ethGravity,
	}, nil
}

func (s *gravityContract) Address() ethcmn.Address {
	return s.gravityAddress
}

// Gets the latest transaction batch nonce
func (s *gravityContract) GetTxBatchNonce(
	ctx context.Context,
	erc20ContractAddress ethcmn.Address,
	callerAddress ethcmn.Address,
) (*big.Int, error) {

	nonce, err := s.ethGravity.LastBatchNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	}, erc20ContractAddress)

	if err != nil {
		return nil, errors.Wrap(err, "LastBatchNonce call failed")
	}

	return nonce, nil
}

// Gets the latest validator set nonce
func (s *gravityContract) GetValsetNonce(
	ctx context.Context,
	callerAddress ethcmn.Address,
) (*big.Int, error) {

	nonce, err := s.ethGravity.StateLastValsetNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		return nil, errors.Wrap(err, "StateLastValsetNonce call failed")
	}

	return nonce, nil
}

// Gets the gravityID
func (s *gravityContract) GetGravityID(
	ctx context.Context,
	callerAddress ethcmn.Address,
) (string, error) {

	gravityID, err := s.ethGravity.StateGravityId(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		err = errors.Wrap(err, "StateGravityId call failed")
		return "", err
	}

	return string(gravityID[:]), nil
}

func (s *gravityContract) GetERC20Symbol(
	ctx context.Context,
	erc20ContractAddress ethcmn.Address,
	callerAddress ethcmn.Address,
) (symbol string, err error) {

	erc20Wrapper, err := wrappers.NewERC20(erc20ContractAddress, s.EVMCommitter.Provider())
	if err != nil {
		err = errors.Wrap(err, "failed to get ERC20 wrapper")
		return "", err
	}

	callOpts := &bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	}

	symbol, err = erc20Wrapper.Symbol(callOpts)
	if err != nil {
		err = errors.Wrap(err, "ERC20 'symbol' call failed")
		return "", err
	}

	return symbol, nil
}

func (s *gravityContract) GetERC20Decimals(
	ctx context.Context,
	tokenAddr ethcmn.Address,
	callerAddr ethcmn.Address,
) (uint8, error) {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	tokenAddrStr := tokenAddr.String()

	if d, ok := s.erc20DecimalCache[tokenAddrStr]; ok {
		return d, nil
	}

	erc20Wrapper, err := wrappers.NewERC20(tokenAddr, s.EVMCommitter.Provider())
	if err != nil {
		return 0, errors.Wrap(err, "failed to get ERC20 wrapper")
	}

	callOpts := &bind.CallOpts{
		From:    callerAddr,
		Context: ctx,
	}

	decimals, err := erc20Wrapper.Decimals(callOpts)
	if err != nil {
		return 0, errors.Wrap(err, "ERC20 'decimals' call failed")
	}

	if s.erc20DecimalCache == nil {
		s.erc20DecimalCache = map[string]uint8{}
	}

	s.erc20DecimalCache[tokenAddrStr] = decimals
	return decimals, nil
}

func sigToVRS(sigHex string) (v uint8, r, s ethcmn.Hash) {
	signatureBytes := ethcmn.FromHex(sigHex)
	vParam := signatureBytes[64]
	if vParam == byte(0) {
		vParam = byte(27)
	} else if vParam == byte(1) {
		vParam = byte(28)
	}

	v = vParam
	r = ethcmn.BytesToHash(signatureBytes[0:32])
	s = ethcmn.BytesToHash(signatureBytes[32:64])

	return
}

// isEnoughPower compares a power value to the power required to pass (a constant)
func isEnoughPower(total *big.Int) bool {
	return total.Cmp(big.NewInt(gravityPowerToPass)) == 1
}
