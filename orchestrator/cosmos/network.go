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

type NetworkConfig struct {
	Name,
	ChainID,
	FeeDenom,
	GasPrices,
	TendermintRPC,
	CosmosGRPC string
}

type Network struct {
	tmclient.TendermintClient
	PeggyQueryClient
	PeggyBroadcastClient
}

func LoadNetwork(cfg NetworkConfig) common.Network {
	switch cfg.Name {
	case "mainnet", "testnet":
		// todo: see if "lb" is an appropriate value
		return common.LoadNetwork(cfg.Name, "lb")
	case "devnet", "devnet-1":
		return common.LoadNetwork(cfg.Name, "")
	case "local":
		//	todo:
		net := common.LoadNetwork("devnet", "")
		net.Name = "local"
		net.ChainId = cfg.ChainID
		net.Fee_denom = cfg.FeeDenom
		net.TmEndpoint = cfg.TendermintRPC
		net.ChainGrpcEndpoint = cfg.CosmosGRPC
		net.ExplorerGrpcEndpoint = ""
		net.LcdEndpoint = ""
		net.ExplorerGrpcEndpoint = ""

		return net
	}

	return common.Network{}
}

func NewNetwork(
	validatorAddress string,
	cfg NetworkConfig,
	keyring keyring.Keyring,
	signerFn bind.SignerFn,
	personalSignerFn keystore.PersonalSignFn,
) (*Network, error) {
	clientCtx, err := chainclient.NewClientContext(cfg.ChainID, validatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmRPC, err := rpchttp.New(cfg.TendermintRPC, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", cfg.TendermintRPC)
	}

	clientCtx = clientCtx.WithNodeURI(cfg.TendermintRPC)
	clientCtx = clientCtx.WithClient(tmRPC)

	daemonClient, err := chainclient.NewChainClient(clientCtx, LoadNetwork(cfg), common.OptionGasPrices(cfg.GasPrices))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", cfg.CosmosGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := types.NewQueryClient(grpcConn)

	n := &Network{
		TendermintClient:     tmclient.NewRPCClient(cfg.TendermintRPC),
		PeggyQueryClient:     NewPeggyQueryClient(peggyQuerier),
		PeggyBroadcastClient: NewPeggyBroadcastClient(peggyQuerier, daemonClient, signerFn, personalSignerFn),
	}

	log.WithFields(log.Fields{
		"chain_id":   cfg.ChainID,
		"injective":  cfg.CosmosGRPC,
		"tendermint": cfg.TendermintRPC,
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

func (n *Network) UnconfirmedTransactionBatches(ctx context.Context) ([]*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.UnconfirmedTransactionBatches(ctx, n.AccFromAddress())
}

func (n *Network) LatestTransactionBatches(ctx context.Context) ([]*peggy.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.LatestTransactionBatches(ctx)
}

func (n *Network) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract ethcmn.Address) ([]*peggy.MsgConfirmBatch, error) {
	return n.PeggyQueryClient.TransactionBatchSignatures(ctx, nonce, tokenContract)
}

func (n *Network) FirstConfirmedOutgoingTxBatch(ctx context.Context) (*types.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.FirstConfirmedOutgoingTxBatch(ctx)
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
