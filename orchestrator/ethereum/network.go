package ethereum

import (
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/xlab/suplog"
	"time"
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

	return &Network{PeggyContract: peggyContract}, nil
}
