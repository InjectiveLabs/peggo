package peggy

import (
	"context"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	wrappers "github.com/umee-network/peggo/solidity/wrappers/Peggy.sol"
	"github.com/umee-network/umee/x/peggy/types"
)

// The total power in the Peggy bridge is normalized to u32 max every
// time a validator set is created. This value of up to u32 max is then
// stored in a i64 to prevent overflow during computation.
const totalPeggyPower int64 = math.MaxUint32

var (
	peggyABI, _ = abi.JSON(strings.NewReader(wrappers.PeggyABI))

	ErrInsufficientVotingPowerToPass = errors.New("insufficient voting power")
)

type Contract interface {
	committer.EVMCommitter

	// Address returns the Peggy contract address
	Address() common.Address

	// EncodeTransactionBatch encodes a batch into a tx byte data. This is specially helpful for estimating gas and
	// detecting identical transactions in the mempool.
	EncodeTransactionBatch(
		ctx context.Context,
		currentValset *types.Valset,
		batch *types.OutgoingTxBatch,
		confirms []*types.MsgConfirmBatch,
	) ([]byte, error)

	// EncodeValsetUpdate encodes a valset update into a tx byte data. This is specially helpful for estimating gas and
	// detecting identical transactions in the mempool.
	EncodeValsetUpdate(
		ctx context.Context,
		oldValset *types.Valset,
		newValset *types.Valset,
		confirms []*types.MsgValsetConfirm,
	) ([]byte, error)

	GetTxBatchNonce(
		ctx context.Context,
		erc20ContractAddress common.Address,
		callerAddress common.Address,
	) (*big.Int, error)

	GetValsetNonce(
		ctx context.Context,
		callerAddress common.Address,
	) (*big.Int, error)

	GetPeggyID(
		ctx context.Context,
		callerAddress common.Address,
	) (common.Hash, error)

	GetERC20Symbol(
		ctx context.Context,
		erc20ContractAddress common.Address,
		callerAddress common.Address,
	) (symbol string, err error)

	GetERC20Decimals(
		ctx context.Context,
		erc20ContractAddress common.Address,
		callerAddress common.Address,
	) (decimals uint8, err error)

	// SubscribeToPendingTxs starts a websocket connection to Alchemy's service that listens for new pending txs made
	// to the Peggy contract.
	SubscribeToPendingTxs(ctx context.Context, alchemyWebsocketURL string) error

	// IsPendingTxInput returns true if the input data is found in the pending tx list. If the tx is found but the tx is
	// older than pendingTxWaitDuration, we consider it stale and return false, so the validator re-sends it.
	IsPendingTxInput(txData []byte, pendingTxWaitDuration time.Duration) bool

	GetPendingTxInputList() *PendingTxInputList
}

type peggyContract struct {
	committer.EVMCommitter

	logger             zerolog.Logger
	peggyAddress       common.Address
	ethPeggy           *wrappers.Peggy
	pendingTxInputList PendingTxInputList

	mtx               sync.Mutex
	erc20DecimalCache map[string]uint8
}

func NewPeggyContract(
	logger zerolog.Logger,
	ethCommitter committer.EVMCommitter,
	peggyAddress common.Address,
	ethPeggy *wrappers.Peggy,
) (Contract, error) {
	return &peggyContract{
		logger:       logger.With().Str("module", "peggy_contract").Logger(),
		EVMCommitter: ethCommitter,
		peggyAddress: peggyAddress,
		ethPeggy:     ethPeggy,
	}, nil
}

func (s *peggyContract) Address() common.Address {
	return s.peggyAddress
}

// Gets the latest transaction batch nonce
func (s *peggyContract) GetTxBatchNonce(
	ctx context.Context,
	erc20ContractAddress common.Address,
	callerAddress common.Address,
) (*big.Int, error) {

	nonce, err := s.ethPeggy.LastBatchNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	}, erc20ContractAddress)

	if err != nil {
		return nil, errors.Wrap(err, "LastBatchNonce call failed")
	}

	return nonce, nil
}

// Gets the latest validator set nonce
func (s *peggyContract) GetValsetNonce(
	ctx context.Context,
	callerAddress common.Address,
) (*big.Int, error) {

	nonce, err := s.ethPeggy.StateLastValsetNonce(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		return nil, errors.Wrap(err, "StateLastValsetNonce call failed")
	}

	return nonce, nil
}

// Gets the peggyID
func (s *peggyContract) GetPeggyID(
	ctx context.Context,
	callerAddress common.Address,
) (common.Hash, error) {

	peggyID, err := s.ethPeggy.StatePeggyId(&bind.CallOpts{
		From:    callerAddress,
		Context: ctx,
	})

	if err != nil {
		err = errors.Wrap(err, "StatePeggyId call failed")
		return common.Hash{}, err
	}

	return peggyID, nil
}

func (s *peggyContract) GetERC20Symbol(
	ctx context.Context,
	erc20ContractAddress common.Address,
	callerAddress common.Address,
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

func (s *peggyContract) GetERC20Decimals(
	ctx context.Context,
	tokenAddr common.Address,
	callerAddr common.Address,
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

func sigToVRS(sigHex string) (v uint8, r, s common.Hash) {
	signatureBytes := common.FromHex(sigHex)
	vParam := signatureBytes[64]
	if vParam == byte(0) {
		vParam = byte(27)
	} else if vParam == byte(1) {
		vParam = byte(28)
	}

	v = vParam
	r = common.BytesToHash(signatureBytes[0:32])
	s = common.BytesToHash(signatureBytes[32:64])

	return
}

// peggyPowerToPercent takes in an amount of power in the peggy bridge, returns a percentage of total
func peggyPowerToPercent(total *big.Int) float32 {
	d := decimal.NewFromBigInt(total, 0)
	f, _ := d.Div(decimal.NewFromInt(totalPeggyPower)).Shift(2).Float64()
	return float32(f)
}
