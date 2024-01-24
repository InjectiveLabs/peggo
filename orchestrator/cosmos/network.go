package cosmos

import (
	"context"
	"fmt"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tendermint"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/InjectiveLabs/sdk-go/client/chain"
	comethttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
	clientcommon "github.com/InjectiveLabs/sdk-go/client/common"
)

type NetworkConfig struct {
	ChainID,
	ValidatorAddress,
	CosmosGRPC,
	TendermintRPC,
	GasPrice string
}

func (cfg NetworkConfig) loadClientConfig() clientcommon.Network {
	if custom := cfg.CosmosGRPC != "" && cfg.TendermintRPC != ""; custom {
		c := clientcommon.LoadNetwork("devnet", "")
		c.Name = "custom"
		c.ChainId = cfg.ChainID
		c.Fee_denom = "inj"
		c.TmEndpoint = cfg.TendermintRPC
		c.ChainGrpcEndpoint = cfg.CosmosGRPC
		c.ExplorerGrpcEndpoint = ""
		c.LcdEndpoint = ""
		c.ExplorerGrpcEndpoint = ""

		return c
	}

	var networkName string
	switch cfg.ChainID {
	case "injective-1":
		networkName = "mainnet"
	case "injective-777":
		networkName = "devnet"
	case "injective-888":
		networkName = "testnet"
	default:
		panic(fmt.Errorf("no provider for chain id %s", cfg.ChainID))
	}

	return clientcommon.LoadNetwork(networkName, "lb")
}

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

	clientCtx = clientCtx.WithNodeURI(clientCfg.TmEndpoint)
	clientCtx = clientCtx.WithClient(tmRPC)

	chainClient, err := chain.NewChainClient(clientCtx, clientCfg, clientcommon.OptionGasPrices(cfg.GasPrice))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to Injective GRPC %s", cfg.CosmosGRPC)
	}

	time.Sleep(1 * time.Second)

	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
	defer cancelWait()

	grpcConn := chainClient.QueryClient()
	waitForService(daemonWaitCtx, grpcConn)
	peggyQuerier := peggytypes.NewQueryClient(grpcConn)

	n := network{
		QueryClient:     peggy.NewQueryClient(peggyQuerier),
		BroadcastClient: peggy.NewBroadcastClient(chainClient, ethSignFn),
		Client:          tendermint.NewRPCClient(clientCfg.TmEndpoint),
	}

	log.WithFields(log.Fields{
		"chain_id":   cfg.ChainID,
		"addr":       cfg.ValidatorAddress,
		"injective":  cfg.CosmosGRPC,
		"tendermint": clientCfg.TmEndpoint,
	}).Infoln("connected to custom Injective endpoints")

	return n, nil
}

func (n network) GetBlockTime(ctx context.Context, height int64) (time.Time, error) {
	block, err := n.Client.GetBlock(ctx, height)
	if err != nil {
		return time.Time{}, err
	}

	return block.Block.Time, nil
}
