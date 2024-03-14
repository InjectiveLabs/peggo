package cosmos

import (
	"context"
	"fmt"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"time"

	comethttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/InjectiveLabs/sdk-go/client/chain"
	clientcommon "github.com/InjectiveLabs/sdk-go/client/common"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tendermint"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/keystore"
)

type NetworkConfig struct {
	ChainID,
	ValidatorAddress,
	CosmosGRPC,
	TendermintRPC,
	GasPrice string
}

type Network interface {
	peggy.QueryClient
	peggy.BroadcastClient
	tendermint.Client
}

func NewNetwork(k keyring.Keyring, ethSignFn keystore.PersonalSignFn, cfg NetworkConfig) (Network, error) {
	clientCfg := cfg.loadClientConfig()

	clientCtx, err := chain.NewClientContext(clientCfg.ChainId, cfg.ValidatorAddress, k)
	if err != nil {
		return nil, err
	}

	clientCtx.WithNodeURI(clientCfg.TmEndpoint)

	tmRPC, err := comethttp.New(clientCfg.TmEndpoint, "/websocket")
	if err != nil {
		return nil, err
	}

	clientCtx = clientCtx.WithClient(tmRPC)

	chainClient, err := chain.NewChainClient(clientCtx, clientCfg, clientcommon.OptionGasPrices(cfg.GasPrice))
	if err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	conn := awaitConnection(chainClient, 1*time.Minute)

	net := struct {
		peggy.QueryClient
		peggy.BroadcastClient
		tendermint.Client
	}{
		peggy.NewQueryClient(peggytypes.NewQueryClient(conn)),
		peggy.NewBroadcastClient(chainClient, ethSignFn),
		tendermint.NewRPCClient(clientCfg.TmEndpoint),
	}

	log.WithFields(log.Fields{
		"chain_id":   cfg.ChainID,
		"addr":       cfg.ValidatorAddress,
		"chain_grpc": clientCfg.ChainGrpcEndpoint,
		"tendermint": clientCfg.TmEndpoint,
	}).Infoln("connected to Injective network")

	return net, nil
}

func awaitConnection(client chain.ChainClient, timeout time.Duration) *grpc.ClientConn {
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

func (cfg NetworkConfig) loadClientConfig() clientcommon.Network {
	if custom := cfg.CosmosGRPC != "" && cfg.TendermintRPC != ""; custom {
		return customEndpoints(cfg)
	}

	return loadBalancedEndpoints(cfg)
}

func customEndpoints(cfg NetworkConfig) clientcommon.Network {
	c := clientcommon.LoadNetwork("devnet", "")
	c.Name = "custom"
	c.ChainId = cfg.ChainID
	c.Fee_denom = "inj"
	c.TmEndpoint = cfg.TendermintRPC
	c.ChainGrpcEndpoint = cfg.CosmosGRPC
	c.ExplorerGrpcEndpoint = ""
	c.LcdEndpoint = ""
	c.ExplorerGrpcEndpoint = ""

	log.Infoln("using custom endpoints for Injective")

	return c
}

func loadBalancedEndpoints(cfg NetworkConfig) clientcommon.Network {
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

	log.Infoln("using load balanced endpoints for Injective")

	return clientcommon.LoadNetwork(networkName, "lb")
}

func IsBondedValidator(n Network, ethAddr gethcommon.Address) bool {
	ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFn()

	vs, err := n.CurrentValset(ctx)
	if err != nil {
		log.Fatalln("failed to query current validator set on Injective")
	}

	for _, validator := range vs.Members {
		if validator.EthereumAddress == ethAddr.Hex() {
			return true
		}
	}

	return false
}
