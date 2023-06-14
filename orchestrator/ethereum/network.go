package ethereum

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	wrappers "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
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
		log.WithField("endpoint", ethNodeRPC).WithError(err).Fatalln("Failed to connect to Ethereum RPC")
		return nil, err
	}

	log.Infoln("Connected to Ethereum RPC at", ethNodeRPC)

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

	// If Alchemy Websocket URL is set, then Subscribe to Pending Transaction of Peggy Contract.
	if ethNodeAlchemyWS != "" {
		go peggyContract.SubscribeToPendingTxs(ethNodeAlchemyWS)
	}

	return &Network{
		PeggyContract: peggyContract,
	}, nil
}

func (n *Network) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return n.Provider().HeaderByNumber(ctx, number)
}

func (n *Network) GetSendToCosmosEvents(startBlock, endBlock uint64) ([]*wrappers.PeggySendToCosmosEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	endblock := endBlock
	iter, err := peggyFilterer.FilterSendToCosmosEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endblock,
	}, nil, nil, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	var sendToCosmosEvents []*wrappers.PeggySendToCosmosEvent
	for iter.Next() {
		sendToCosmosEvents = append(sendToCosmosEvents, iter.Event)
	}

	iter.Close()

	return sendToCosmosEvents, nil
}

func (n *Network) GetSendToInjectiveEvents(startBlock, endBlock uint64) ([]*wrappers.PeggySendToInjectiveEvent, error) {
	peggyFilterer, err := wrappers.NewPeggyFilterer(n.Address(), n.Provider())
	if err != nil {
		return nil, errors.Wrap(err, "failed to init Peggy events filterer")
	}

	endblock := endBlock
	iter, err := peggyFilterer.FilterSendToInjectiveEvent(&bind.FilterOpts{
		Start: startBlock,
		End:   &endblock,
	}, nil, nil, nil)
	if err != nil {
		if !isUnknownBlockErr(err) {
			return nil, errors.Wrap(err, "failed to scan past SendToCosmos events from Ethereum")
		} else if iter == nil {
			return nil, errors.New("no iterator returned")
		}
	}

	var sendToInjectiveEvents []*wrappers.PeggySendToInjectiveEvent
	for iter.Next() {
		sendToInjectiveEvents = append(sendToInjectiveEvents, iter.Event)
	}

	iter.Close()

	return sendToInjectiveEvents, nil
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

	var transactionBatchExecutedEvents []*wrappers.PeggyTransactionBatchExecutedEvent
	for iter.Next() {
		transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
	}

	iter.Close()

	return transactionBatchExecutedEvents, nil
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

	var transactionBatchExecutedEvents []*wrappers.PeggyERC20DeployedEvent
	for iter.Next() {
		transactionBatchExecutedEvents = append(transactionBatchExecutedEvents, iter.Event)
	}

	iter.Close()

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

	var valsetUpdatedEvents []*wrappers.PeggyValsetUpdatedEvent
	for iter.Next() {
		valsetUpdatedEvents = append(valsetUpdatedEvents, iter.Event)
	}

	iter.Close()

	return valsetUpdatedEvents, nil
}

func (n *Network) GetPeggyID(ctx context.Context) (ethcmn.Hash, error) {
	return n.PeggyContract.GetPeggyID(ctx, n.FromAddress())
}

func (n *Network) FromAddress() ethcmn.Address {
	return n.PeggyContract.FromAddress()
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
