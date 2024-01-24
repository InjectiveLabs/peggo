package cosmos

import (
	"context"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tendermint"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/InjectiveLabs/sdk-go/client/chain"
	clientcommon "github.com/InjectiveLabs/sdk-go/client/common"
)

type providerNetwork struct {
	peggy.QueryClient
	peggy.BroadcastClient
	tendermint.Client
}

// NewLoadBalancedNetwork creates a load balanced connection to the Injective network.
// The chainID argument decides which network Peggo will be connecting to:
//   - injective-1 (mainnet)
//   - injective-777 (devnet)
//   - injective-888 (testnet)
func newProviderNetwork(
	cfg NetworkConfig,
	keyring keyring.Keyring,
	personalSignerFn keystore.PersonalSignFn,
) (Network, error) {
	var networkName string
	switch cfg.ChainID {
	case "injective-1":
		networkName = "mainnet"
	case "injective-777":
		networkName = "devnet"
	case "injective-888":
		networkName = "testnet"
	default:
		return nil, errors.Errorf("provided chain id %v does not belong to any known Injective network", cfg.ChainID)
	}
	netCfg := clientcommon.LoadNetwork(networkName, "lb")

	clientCtx, err := chain.NewClientContext(cfg.ChainID, cfg.ValidatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmClient, err := rpchttp.New(netCfg.TmEndpoint, "/websocket")
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize tendermint client")
	}

	clientCtx = clientCtx.WithNodeURI(netCfg.TmEndpoint).WithClient(tmClient)

	daemonClient, err := chain.NewChainClient(clientCtx, netCfg, clientcommon.OptionGasPrices(cfg.GasPrice))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to intialize chain client (%s)", networkName)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := peggytypes.NewQueryClient(grpcConn)

	//explorerCLient, err := explorer.NewExplorerClient(netCfg)
	//if err != nil {
	//	return nil, errors.Wrap(err, "failed to initialize explorer client")
	//}

	n := &providerNetwork{
		Client:          tendermint.NewRPCClient(netCfg.TmEndpoint),
		QueryClient:     peggy.NewQueryClient(peggyQuerier),
		BroadcastClient: peggy.NewBroadcastClient(daemonClient, personalSignerFn),
	}

	log.WithFields(log.Fields{
		"addr":       cfg.ValidatorAddress,
		"chain_id":   cfg.ChainID,
		"injective":  netCfg.ChainGrpcEndpoint,
		"tendermint": netCfg.TmEndpoint,
	}).Infoln("connected to Injective's load balanced endpoints")

	return n, nil
}

//
//func (n *providerNetwork) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
//	block, err := n.ExplorerClient.GetBlock(ctx, strconv.FormatInt(height, 10))
//	if err != nil {
//		return time.Time{}, err
//	}
//
//	blockTime, err := time.Parse("2006-01-02 15:04:05.999 -0700 MST", block.Data.Timestamp)
//	if err != nil {
//		return time.Time{}, errors.Wrap(err, "failed to parse timestamp from block")
//	}
//
//	return blockTime, nil
//}

func (n *providerNetwork) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.Client.GetBlock(ctx, height)
	if err != nil {
		return time.Time{}, err
	}

	return block.Block.Time, nil
}

// waitForService awaits an active ClientConn to a GRPC service.
func waitForService(ctx context.Context, clientConn *grpc.ClientConn) {
	for {
		select {
		case <-ctx.Done():
			log.Fatalln("GRPC service wait timed out")
		default:
			state := clientConn.GetState()

			if state != connectivity.Ready {
				log.WithField("state", state.String()).Warningln("state of GRPC connection not ready")
				time.Sleep(5 * time.Second)
				continue
			}

			return
		}
	}
}
