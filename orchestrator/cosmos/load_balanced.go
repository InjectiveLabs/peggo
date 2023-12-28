package cosmos

import (
	"context"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	"github.com/InjectiveLabs/sdk-go/client/common"
	explorerclient "github.com/InjectiveLabs/sdk-go/client/explorer"
)

type LoadBalancedNetwork struct {
	PeggyQueryClient
	PeggyBroadcastClient
	explorerclient.ExplorerClient
}

// NewLoadBalancedNetwork creates a load balanced connection to the Injective network.
// The chainID argument decides which network Peggo will be connecting to:
//   - injective-1 (mainnet)
//   - injective-777 (devnet)
//   - injective-888 (testnet)
func NewLoadBalancedNetwork(
	chainID,
	validatorAddress,
	injectiveGasPrices string,
	keyring keyring.Keyring,
	personalSignerFn keystore.PersonalSignFn,
) (*LoadBalancedNetwork, error) {
	clientCtx, err := chainclient.NewClientContext(chainID, validatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	var networkName string
	switch chainID {
	case "injective-1":
		networkName = "mainnet"
	case "injective-777":
		networkName = "devnet"
	case "injective-888":
		networkName = "testnet"
	default:
		return nil, errors.Errorf("provided chain id %v does not belong to any known Injective network", chainID)
	}

	netCfg := common.LoadNetwork(networkName, "lb")
	explorer, err := explorerclient.NewExplorerClient(netCfg)
	if err != nil {
		return nil, err
	}

	daemonClient, err := chainclient.NewChainClient(clientCtx, netCfg, common.OptionGasPrices(injectiveGasPrices))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective network: %s", networkName)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := types.NewQueryClient(grpcConn)

	n := &LoadBalancedNetwork{
		PeggyQueryClient:     NewPeggyQueryClient(peggyQuerier),
		PeggyBroadcastClient: NewPeggyBroadcastClient(peggyQuerier, daemonClient, personalSignerFn),
		ExplorerClient:       explorer,
	}

	log.WithFields(log.Fields{"chain_id": chainID, "connection": "load_balanced"}).Infoln("connected to Injective network")

	return n, nil
}

func (n *LoadBalancedNetwork) GetBlockCreationTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.ExplorerClient.GetBlock(ctx, strconv.FormatInt(height, 10))
	if err != nil {
		return time.Time{}, err
	}

	blockTime, err := time.Parse("2006-01-02 15:04:05.999 -0700 MST", block.Data.Timestamp)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to parse timestamp from block")
	}

	return blockTime, nil
}

func (n *LoadBalancedNetwork) PeggyParams(ctx context.Context) (*peggytypes.Params, error) {
	return n.PeggyQueryClient.PeggyParams(ctx)
}

func (n *LoadBalancedNetwork) LastClaimEvent(ctx context.Context) (*peggytypes.LastClaimEvent, error) {
	return n.LastClaimEventByAddr(ctx, n.AccFromAddress())
}

func (n *LoadBalancedNetwork) SendEthereumClaims(
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

func (n *LoadBalancedNetwork) UnbatchedTokenFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	return n.PeggyQueryClient.UnbatchedTokensWithFees(ctx)
}

func (n *LoadBalancedNetwork) SendRequestBatch(ctx context.Context, denom string) error {
	return n.PeggyBroadcastClient.SendRequestBatch(ctx, denom)
}

func (n *LoadBalancedNetwork) OldestUnsignedValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return n.PeggyQueryClient.OldestUnsignedValsets(ctx, n.AccFromAddress())
}

func (n *LoadBalancedNetwork) LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	return n.PeggyQueryClient.LatestValsets(ctx)
}

func (n *LoadBalancedNetwork) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error) {
	return n.PeggyQueryClient.AllValsetConfirms(ctx, nonce)
}

func (n *LoadBalancedNetwork) ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error) {
	return n.PeggyQueryClient.ValsetAt(ctx, nonce)
}

func (n *LoadBalancedNetwork) SendValsetConfirm(
	ctx context.Context,
	peggyID gethcommon.Hash,
	valset *peggytypes.Valset,
	ethFrom gethcommon.Address,
) error {
	return n.PeggyBroadcastClient.SendValsetConfirm(ctx, ethFrom, peggyID, valset)
}

func (n *LoadBalancedNetwork) OldestUnsignedTransactionBatch(ctx context.Context) (*peggytypes.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.OldestUnsignedTransactionBatch(ctx, n.AccFromAddress())
}

func (n *LoadBalancedNetwork) LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
	return n.PeggyQueryClient.LatestTransactionBatches(ctx)
}

func (n *LoadBalancedNetwork) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
	return n.PeggyQueryClient.TransactionBatchSignatures(ctx, nonce, tokenContract)
}

func (n *LoadBalancedNetwork) SendBatchConfirm(
	ctx context.Context,
	peggyID gethcommon.Hash,
	batch *peggytypes.OutgoingTxBatch,
	ethFrom gethcommon.Address,
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
