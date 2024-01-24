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

type CustomRPCNetwork struct {
	peggy.QueryClient
	peggy.BroadcastClient
	tendermint.Client
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

// NewCustomRPCNetwork creates a single endpoint connection to the Injective network
func NewCustomRPCNetwork(
	chainID,
	validatorAddress,
	injectiveGRPC,
	injectiveGasPrices,
	tendermintRPC string,
	keyring keyring.Keyring,
	personalSignerFn keystore.PersonalSignFn,
) (Network, error) {
	clientCtx, err := chain.NewClientContext(chainID, validatorAddress, keyring)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client context for Injective chain")
	}

	tmRPC, err := comethttp.New(tendermintRPC, "/websocket")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Tendermint RPC %s", tendermintRPC)
	}

	clientCtx = clientCtx.WithNodeURI(tendermintRPC)
	clientCtx = clientCtx.WithClient(tmRPC)

	netCfg := loadCustomNetworkConfig(chainID, "inj", injectiveGRPC, tendermintRPC)
	daemonClient, err := chain.NewChainClient(clientCtx, netCfg, clientcommon.OptionGasPrices(injectiveGasPrices))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", injectiveGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := daemonClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := peggytypes.NewQueryClient(grpcConn)

	n := &CustomRPCNetwork{
		Client:          tendermint.NewRPCClient(tendermintRPC),
		QueryClient:     peggy.NewQueryClient(peggyQuerier),
		BroadcastClient: peggy.NewBroadcastClient(daemonClient, personalSignerFn),
	}

	log.WithFields(log.Fields{
		"chain_id":   chainID,
		"addr":       validatorAddress,
		"injective":  injectiveGRPC,
		"tendermint": tendermintRPC,
	}).Infoln("connected to custom Injective endpoints")

	return n, nil
}

func (n *CustomRPCNetwork) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.Client.GetBlock(ctx, height)
	if err != nil {
		return time.Time{}, err
	}

	return block.Block.Time, nil
}
