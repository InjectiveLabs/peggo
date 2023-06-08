package orchestrator

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"

	ethcmn "github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	sidechain "github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/InjectiveLabs/peggo/orchestrator/relayer"
)

type Injective interface {
}

type EthereumNetwork interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
}

type PeggyOrchestrator struct {
	svcTags  metrics.Tags
	ethereum EthereumNetwork

	cosmosQueryClient       sidechain.PeggyQueryClient
	peggyBroadcastClient    sidechain.PeggyBroadcastClient
	peggyContract           peggy.PeggyContract
	ethProvider             provider.EVMProvider
	ethFrom                 ethcmn.Address
	erc20ContractMapping    map[ethcmn.Address]string
	relayer                 relayer.PeggyRelayer
	minBatchFeeUSD          float64
	priceFeeder             *coingecko.CoingeckoPriceFeed
	periodicBatchRequesting bool
}

func NewPeggyOrchestrator(
	cosmosQueryClient sidechain.PeggyQueryClient,
	peggyBroadcastClient sidechain.PeggyBroadcastClient,
	peggyContract peggy.PeggyContract,
	ethFrom ethcmn.Address,
	erc20ContractMapping map[ethcmn.Address]string,
	relayer relayer.PeggyRelayer,
	minBatchFeeUSD float64,
	priceFeeder *coingecko.CoingeckoPriceFeed,
	periodicBatchRequesting bool,
) *PeggyOrchestrator {
	return &PeggyOrchestrator{
		cosmosQueryClient:       cosmosQueryClient,
		peggyBroadcastClient:    peggyBroadcastClient,
		peggyContract:           peggyContract,
		ethProvider:             peggyContract.Provider(),
		ethFrom:                 ethFrom,
		erc20ContractMapping:    erc20ContractMapping,
		relayer:                 relayer,
		minBatchFeeUSD:          minBatchFeeUSD,
		priceFeeder:             priceFeeder,
		periodicBatchRequesting: periodicBatchRequesting,
		svcTags: metrics.Tags{
			"svc": "peggy_orchestrator",
		},
	}
}
