package cosmos

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tmclient"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	"github.com/InjectiveLabs/sdk-go/client/common"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"time"
)

type CustomRPCNetwork struct {
	tmclient.TendermintClient
	PeggyQueryClient
	PeggyBroadcastClient
}

func loadCustomNetworkConfig(chainID, feeDenom, cosmosGRPC, tendermintRPC string) common.Network {
	cfg := common.LoadNetwork("devnet", "")
	cfg.Name = "custom"
	cfg.ChainId = chainID
	cfg.Fee_denom = feeDenom
	cfg.TmEndpoint = tendermintRPC
	cfg.ChainGrpcEndpoint = cosmosGRPC
	cfg.ExplorerGrpcEndpoint = ""
	cfg.LcdEndpoint = ""
	cfg.ExplorerGrpcEndpoint = ""

	return cfg
}

// NewCustomRPCNetwork creates a single endpoint connection to the Injective network
func NewCustomRPCNetwork(
	chainID,
	validatorAddress,
	injectiveGRPC,
	injectiveGasPrices,
	tendermintRPC string,
	keyring keyring.Keyring,
	signerFn bind.SignerFn,
	personalSignerFn keystore.PersonalSignFn,
) (*CustomRPCNetwork, error) {
	clientCtx, err := chainclient.NewClientContext(chainID, validatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmRPC, err := rpchttp.New(tendermintRPC, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", tendermintRPC)
	}

	clientCtx = clientCtx.WithNodeURI(tendermintRPC)
	clientCtx = clientCtx.WithClient(tmRPC)

	netCfg := loadCustomNetworkConfig(chainID, "inj", injectiveGRPC, tendermintRPC)
	daemonClient, err := chainclient.NewChainClient(clientCtx, netCfg, common.OptionGasPrices(injectiveGasPrices))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", injectiveGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := types.NewQueryClient(grpcConn)

	n := &CustomRPCNetwork{
		TendermintClient:     tmclient.NewRPCClient(tendermintRPC),
		PeggyQueryClient:     NewPeggyQueryClient(peggyQuerier),
		PeggyBroadcastClient: NewPeggyBroadcastClient(peggyQuerier, daemonClient, signerFn, personalSignerFn),
	}

	log.WithFields(log.Fields{
		"chain_id":   chainID,
		"connection": "custom_rpc",
		"injective":  injectiveGRPC,
		"tendermint": tendermintRPC,
	}).Infoln("connected to Injective network")

	return n, nil
}

func (n *CustomRPCNetwork) GetBlockCreationTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.TendermintClient.GetBlock(ctx, height)
	if err != nil {
		return time.Time{}, err
	}

	return block.Block.Time, nil
}

//func (n *LoadBalancedNetwork) PeggyParams(ctx context.Context) (*peggytypes.Params, error) {
//	return n.PeggyQueryClient.PeggyParams(ctx)
//}

func (n *CustomRPCNetwork) LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error) {
	return n.LastClaimEventByAddr(ctx, n.AccFromAddress())
}

func (n *CustomRPCNetwork) SendEthereumClaims(
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

func (n *CustomRPCNetwork) UnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	return n.PeggyQueryClient.UnbatchedTokensWithFees(ctx)
}

func (n *CustomRPCNetwork) SendRequestBatch(ctx context.Context, denom string) error {
	return n.PeggyBroadcastClient.SendRequestBatch(ctx, denom)
}

func (n *CustomRPCNetwork) OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return n.PeggyQueryClient.OldestUnsignedValsets(ctx, n.AccFromAddress())
}

func (n *CustomRPCNetwork) LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return n.PeggyQueryClient.LatestValsets(ctx)
}

func (n *CustomRPCNetwork) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error) {
	return n.PeggyQueryClient.AllValsetConfirms(ctx, nonce)
}

func (n *CustomRPCNetwork) ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error) {
	return n.PeggyQueryClient.ValsetAt(ctx, nonce)
}

func (n *CustomRPCNetwork) SendValsetConfirm(
	ctx context.Context,
	peggyID gethcommon.Hash,
	valset *peggytypes.Valset,
	ethFrom gethcommon.Address,
) error {
	return n.PeggyBroadcastClient.SendValsetConfirm(ctx, ethFrom, peggyID, valset)
}

func (n *CustomRPCNetwork) OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.OldestUnsignedTransactionBatch(ctx, n.AccFromAddress())
}

func (n *CustomRPCNetwork) LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.LatestTransactionBatches(ctx)
}

func (n *CustomRPCNetwork) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
	return n.PeggyQueryClient.TransactionBatchSignatures(ctx, nonce, tokenContract)
}

func (n *CustomRPCNetwork) SendBatchConfirm(
	ctx context.Context,
	peggyID gethcommon.Hash,
	batch *peggytypes.OutgoingTxBatch,
	ethFrom gethcommon.Address,
) error {
	return n.PeggyBroadcastClient.SendBatchConfirm(ctx, ethFrom, peggyID, batch)
}

// waitForService awaits an active ClientConn to a GRPC service.
//func waitForService(ctx context.Context, clientconn *grpc.ClientConn) {
//	for {
//		select {
//		case <-ctx.Done():
//			log.Fatalln("GRPC service wait timed out")
//		default:
//			state := clientconn.GetState()
//
//			if state != connectivity.Ready {
//				log.WithField("state", state.String()).Warningln("state of GRPC connection not ready")
//				time.Sleep(5 * time.Second)
//				continue
//			}
//
//			return
//		}
//	}
//}
