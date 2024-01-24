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

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tendermint"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
)

type customNetwork struct {
	peggy.QueryClient
	peggy.BroadcastClient
	tendermint.Client
}

// NewCustomRPCNetwork creates a single endpoint connection to the Injective network
func newCustomNetwork(
	cfg NetworkConfig,
	keyring keyring.Keyring,
	personalSignerFn keystore.PersonalSignFn,
) (Network, error) {
	netCfg := loadCustomNetworkConfig(cfg.ChainID, "inj", cfg.CosmosGRPC, cfg.TendermintRPC)

	clientCtx, err := chain.NewClientContext(cfg.ChainID, cfg.ValidatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmRPC, err := comethttp.New(cfg.TendermintRPC, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", netCfg.TmEndpoint)
	}

	clientCtx = clientCtx.WithNodeURI(netCfg.TmEndpoint)
	clientCtx = clientCtx.WithClient(tmRPC)

	daemonClient, err := chain.NewChainClient(clientCtx, netCfg, clientcommon.OptionGasPrices(cfg.GasPrice))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", cfg.CosmosGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := peggytypes.NewQueryClient(grpcConn)

	n := &customNetwork{
		QueryClient:     peggy.NewQueryClient(peggyQuerier),
		BroadcastClient: peggy.NewBroadcastClient(daemonClient, personalSignerFn),
		Client:          tendermint.NewRPCClient(netCfg.TmEndpoint),
	}

	log.WithFields(log.Fields{
		"chain_id":   cfg.ChainID,
		"addr":       cfg.ValidatorAddress,
		"injective":  cfg.CosmosGRPC,
		"tendermint": netCfg.TmEndpoint,
	}).Infoln("connected to custom Injective endpoints")

	return n, nil
}

func (n *customNetwork) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.Client.GetBlock(ctx, height)
	if err != nil {
		return time.Time{}, err
	}

	return block.Block.Time, nil
}

func loadCustomNetworkConfig(chainID, feeDenom, cosmosGRPC, tendermintRPC string) clientcommon.Network {
	cfg := clientcommon.LoadNetwork("devnet", "")
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
