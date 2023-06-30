package cosmos

import (
	"context"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	tmctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tmclient"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	peggy "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	"github.com/InjectiveLabs/sdk-go/client/common"
)

type Network struct {
	tmclient.TendermintClient
	PeggyQueryClient
	PeggyBroadcastClient
}

func NewNetwork(
	chainID,
	validatorAddress,
	injectiveGRPC,
	injectiveGasPrices,
	tendermintRPC string,
	keyring keyring.Keyring,
	signerFn bind.SignerFn,
	personalSignerFn keystore.PersonalSignFn,
) (*Network, error) {
	clientCtx, err := chainclient.NewClientContext(chainID, validatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	clientCtx = clientCtx.WithNodeURI(tendermintRPC)

	tmRPC, err := rpchttp.New(tendermintRPC, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", tendermintRPC)
	}

	clientCtx = clientCtx.WithClient(tmRPC)

	daemonClient, err := chainclient.NewChainClient(clientCtx, injectiveGRPC, common.OptionGasPrices(injectiveGasPrices))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", injectiveGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := types.NewQueryClient(grpcConn)

	n := &Network{
		TendermintClient:     tmclient.NewRPCClient(tendermintRPC),
		PeggyQueryClient:     NewPeggyQueryClient(peggyQuerier),
		PeggyBroadcastClient: NewPeggyBroadcastClient(peggyQuerier, daemonClient, signerFn, personalSignerFn),
	}

	log.WithFields(log.Fields{
		"chain_id":   chainID,
		"injective":  injectiveGRPC,
		"tendermint": tendermintRPC,
	}).Infoln("connected to Injective network")

	return n, nil
}

func (n *Network) GetBlock(ctx context.Context, height int64) (*tmctypes.ResultBlock, error) {
	return n.TendermintClient.GetBlock(ctx, height)
}

func (n *Network) PeggyParams(ctx context.Context) (*peggy.Params, error) {
	return n.PeggyQueryClient.PeggyParams(ctx)
}

func (n *Network) LastClaimEvent(ctx context.Context) (*peggy.LastClaimEvent, error) {
	return n.LastClaimEventByAddr(ctx, n.AccFromAddress())
}

func (n *Network) SendEthereumClaims(
	ctx context.Context,
	lastClaimEvent uint64,
	oldDeposits []*peggyevents.PeggySendToCosmosEvent,
	deposits []*peggyevents.PeggySendToInjectiveEvent,
	withdraws []*peggyevents.PeggyTransactionBatchExecutedEvent,
	erc20Deployed []*peggyevents.PeggyERC20DeployedEvent,
	valsetUpdates []*peggyevents.PeggyValsetUpdatedEvent,
) error {
	return n.PeggyBroadcastClient.SendEthereumClaims(ctx,
		lastClaimEvent,
		oldDeposits,
		deposits,
		withdraws,
		erc20Deployed,
		valsetUpdates,
	)
}

func (n *Network) UnbatchedTokenFees(ctx context.Context) ([]*peggy.BatchFees, error) {
	return n.PeggyQueryClient.UnbatchedTokensWithFees(ctx)
}

func (n *Network) SendRequestBatch(ctx context.Context, denom string) error {
	return n.PeggyBroadcastClient.SendRequestBatch(ctx, denom)
}

func (n *Network) OldestUnsignedValsets(ctx context.Context) ([]*peggy.Valset, error) {
	return n.PeggyQueryClient.OldestUnsignedValsets(ctx, n.AccFromAddress())
}

func (n *Network) LatestValsets(ctx context.Context) ([]*peggy.Valset, error) {
	return n.PeggyQueryClient.LatestValsets(ctx)
}

func (n *Network) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggy.MsgValsetConfirm, error) {
	return n.PeggyQueryClient.AllValsetConfirms(ctx, nonce)
}

func (n *Network) ValsetAt(ctx context.Context, nonce uint64) (*peggy.Valset, error) {
	return n.PeggyQueryClient.ValsetAt(ctx, nonce)
}

func (n *Network) SendValsetConfirm(
	ctx context.Context,
	peggyID ethcmn.Hash,
	valset *peggy.Valset,
	ethFrom ethcmn.Address,
) error {
	return n.PeggyBroadcastClient.SendValsetConfirm(ctx, ethFrom, peggyID, valset)
}

func (n *Network) OldestUnsignedTransactionBatch(ctx context.Context) (*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.OldestUnsignedTransactionBatch(ctx, n.AccFromAddress())
}

func (n *Network) LatestTransactionBatches(ctx context.Context) ([]*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.LatestTransactionBatches(ctx)
}

func (n *Network) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract ethcmn.Address) ([]*peggy.MsgConfirmBatch, error) {
	return n.PeggyQueryClient.TransactionBatchSignatures(ctx, nonce, tokenContract)
}

func (n *Network) SendBatchConfirm(
	ctx context.Context,
	peggyID ethcmn.Hash,
	batch *peggy.OutgoingTxBatch,
	ethFrom ethcmn.Address,
) error {
	return n.PeggyBroadcastClient.SendBatchConfirm(ctx, ethFrom, peggyID, batch)
}

// waitForService awaits an active ClientConn to a GRPC service.
func waitForService(ctx context.Context, clientconn *grpc.ClientConn) {
	for {
		select {
		case <-ctx.Done():
			log.Fatalln("GRPC service wait timed out")
		default:
			state := clientconn.GetState()

			if state != connectivity.Ready {
				log.WithField("state", state.String()).Warningln("state of GRPC connection not ready")
				time.Sleep(5 * time.Second)
				continue
			}

			return
		}
	}
}
