package ethereum

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

type Network struct {
	peggy.PeggyContract
}

func NewNetwork(
	ethNodeRPC string,
	peggyContractAddr,
	fromAddr ethcmn.Address,
	signerFn bind.SignerFn,
	gasPriceAdjustment float64,
	maxGasPrice string,
	pendingTxWaitDuration string,
	ethNodeAlchemyWS string,
) (*Network, error) {
	evmRPC, err := rpc.Dial(ethNodeRPC)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to ethereum RPC: %s", ethNodeRPC)
	}

	ethCommitter, err := committer.NewEthCommitter(
		fromAddr,
		gasPriceAdjustment,
		maxGasPrice,
		signerFn,
		provider.NewEVMProvider(evmRPC),
	)
	if err != nil {
		return nil, err
	}

	pendingTxDuration, err := time.ParseDuration(pendingTxWaitDuration)
	if err != nil {
		return nil, err
	}

	peggyContract, err := peggy.NewPeggyContract(ethCommitter, peggyContractAddr, peggy.PendingTxInputList{}, pendingTxDuration)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"rpc":            ethNodeRPC,
		"addr":           fromAddr.String(),
		"peggy_contract": peggyContractAddr,
	}).Infoln("connected to Ethereum network")

	// If Alchemy Websocket URL is set, then Subscribe to Pending Transaction of Peggy Contract.
	if ethNodeAlchemyWS != "" {
		log.WithFields(log.Fields{
			"url": ethNodeAlchemyWS,
		}).Infoln("subscribing to Alchemy websocket")
		go peggyContract.SubscribeToPendingTxs(ethNodeAlchemyWS)
	}

	return &Network{PeggyContract: peggyContract}, nil
}

func (n *Network) FromAddress() ethcmn.Address {
	return n.PeggyContract.FromAddress()
}

func (n *Network) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return n.Provider().HeaderByNumber(ctx, number)
}

func (n *Network) GetPeggyID(ctx context.Context) (ethcmn.Hash, error) {
	return n.PeggyContract.GetPeggyID(ctx, n.FromAddress())
}

func (n *Network) GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	iter, err := peggyFilterer.FilterSendToCosmosEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endBlock,
	}, nil, nil, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	defer iter.Close()

	var sendToCosmosEvents []*wrappers.PeggySendToCosmosEvent
	for iter.Next() {
		sendToCosmosEvents = append(sendToCosmosEvents, iter.Event)
	}

	return sendToCosmosEvents, nil
}

func (n *Network) GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*wrappers.PeggySendToInjectiveEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	iter, err := peggyFilterer.FilterSendToInjectiveEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endBlock,
	}, nil, nil, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	defer iter.Close()

	var sendToInjectiveEvents []*wrappers.PeggySendToInjectiveEvent
	for iter.Next() {
		sendToInjectiveEvents = append(sendToInjectiveEvents, iter.Event)
	}

	return sendToInjectiveEvents, nil
}

func (n *Network) GetPeggyERC20DeployedEvents(startBlock, endBlock uint64) ([]*wrappers.PeggyERC20DeployedEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	iter, err := peggyFilterer.FilterERC20DeployedEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endBlock,
	}, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past TransactionBatchExecuted events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	defer iter.Close()

	var transactionBatchExecutedEvents []*wrappers.PeggyERC20DeployedEvent
	for iter.Next() {
		transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
	}

	return transactionBatchExecutedEvents, nil
}

func (n *Network) GetValsetUpdatedEvents(startBlock, endBlock uint64) ([]*wrappers.PeggyValsetUpdatedEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	iter, err := peggyFilterer.FilterValsetUpdatedEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endBlock,
	}, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past ValsetUpdatedEvent events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	defer iter.Close()

	var valsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent
	for iter.Next() {
		valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
	}

	return valsetUpdatedEvents, nil
}

func (n *Network) GetTransactionBatchExecutedEvents(startBlock, endBlock uint64) ([]*wrappers.PeggyTransactionBatchExecutedEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	iter, err := peggyFilterer.FilterTransactionBatchExecutedEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endBlock,
	}, nil, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past TransactionBatchExecuted events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	defer iter.Close()

	var transactionBatchExecutedEvents []*wrappers.PeggyTransactionBatchExecutedEvent
	for iter.Next() {
		transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
	}

	return transactionBatchExecutedEvents, nil
}

func (n *Network) GetValsetNonce(ctx context.Context) (*big.Int, error) {
	return n.PeggyContract.GetValsetNonce(ctx, n.FromAddress())
}

func (n *Network) SendEthValsetUpdate(
	ctx context.Context,
	oldValset *peggytypes.Valset,
	newValset *peggytypes.Valset,
	confirms []*peggytypes.MsgValsetConfirm,
) (*ethcmn.Hash, error) {
	return n.PeggyContract.SendEthValsetUpdate(ctx, oldValset, newValset, confirms)
}

func (n *Network) GetTxBatchNonce(ctx context.Context, erc20ContractAddress ethcmn.Address) (*big.Int, error) {
	return n.PeggyContract.GetTxBatchNonce(ctx, erc20ContractAddress, n.FromAddress())
}

func (n *Network) SendTransactionBatch(
	ctx context.Context,
	currentValset *peggytypes.Valset,
	batch *peggytypes.OutgoingTxBatch,
	confirms []*peggytypes.MsgConfirmBatch,
) (*ethcmn.Hash, error) {
	return n.PeggyContract.SendTransactionBatch(ctx, currentValset, batch, confirms)
}

func isUnknownBlockErr(err error) bool {
	// Geth error
	if strings.Contains(err.Error(), "unknown block") {
		return true
	}

	// Parity error
	if strings.Contains(err.Error(), "One of the blocks specified in filter") {
		return true
	}

	return false
}
