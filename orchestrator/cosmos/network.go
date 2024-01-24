package cosmos

import (
	"context"
	"time"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/InjectiveLabs/sdk-go/client/chain"
	clientcommon "github.com/InjectiveLabs/sdk-go/client/common"
	comethttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tendermint"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
)

type Network interface {
	GetBlockTime(ctx context.Context, height int64) (time.Time, error)

	peggy.QueryClient
	peggy.BroadcastClient
}

type network struct {
	peggy.QueryClient
	peggy.BroadcastClient
	tendermint.Client
}

func NewCosmosNetwork(k keyring.Keyring, ethSignFn keystore.PersonalSignFn, cfg NetworkConfig) (Network, error) {
	clientCfg := cfg.loadClientConfig()

	clientCtx, err := chain.NewClientContext(clientCfg.ChainId, cfg.ValidatorAddress, k)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmRPC, err := comethttp.New(clientCfg.TmEndpoint, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", clientCfg.TmEndpoint)
	}

	clientCtx = clientCtx.WithNodeURI(clientCfg.TmEndpoint).WithClient(tmRPC)

	chainClient, err := chain.NewChainClient(clientCtx, clientCfg, clientcommon.OptionGasPrices(cfg.GasPrice))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", cfg.CosmosGRPC)
	}

	time.Sleep(1 * time.Second)

	conn := awaitConnection(1*time.Minute, chainClient)

	n := network{
		QueryClient:     peggy.NewQueryClient(peggytypes.NewQueryClient(conn)),
		BroadcastClient: peggy.NewBroadcastClient(chainClient, ethSignFn),
		Client:          tendermint.NewRPCClient(clientCfg.TmEndpoint),
	}

	log.WithFields(log.Fields{
		"chain_id":   cfg.ChainID,
		"addr":       cfg.ValidatorAddress,
		"injective":  clientCfg.ChainGrpcEndpoint,
		"tendermint": clientCfg.TmEndpoint,
	}).Infoln("connected to Injective network")

	return n, nil
}

func (n network) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
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

func awaitConnection(timeout time.Duration, client chain.ChainClient) *grpc.ClientConn {
	ctx, cancelWait := context.WithTimeout(context.Background(), timeout)
	defer cancelWait()

	grpcConn := client.QueryClient()

	for {
		select {
		case <-ctx.Done():
			log.Fatalln("GRPC service wait timed out")
		default:
			state := grpcConn.GetState()
			if state != connectivity.Ready {
				log.WithField("state", state.String()).Warningln("state of GRPC connection not ready")
				time.Sleep(5 * time.Second)
				continue
			}

			return grpcConn
		}
	}
}
