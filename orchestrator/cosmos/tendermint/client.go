package tendermint

import (
	"context"
	"strings"

	"github.com/InjectiveLabs/metrics"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	comettypes "github.com/cometbft/cometbft/rpc/core/types"
	log "github.com/xlab/suplog"
)

type Client interface {
	GetBlock(ctx context.Context, height int64) (*comettypes.ResultBlock, error)
	GetLatestBlockHeight(ctx context.Context) (int64, error)
	GetTxs(ctx context.Context, block *comettypes.ResultBlock) ([]*comettypes.ResultTx, error)
	GetValidatorSet(ctx context.Context, height int64) (*comettypes.ResultValidators, error)
}

type tmClient struct {
	rpcClient rpcclient.Client
	svcTags   metrics.Tags
}

func NewRPCClient(rpcNodeAddr string) Client {
	rpcClient, err := rpchttp.NewWithTimeout(rpcNodeAddr, "/websocket", 10)
	if err != nil {
		log.WithError(err).Fatalln("failed to init rpcClient")
	}

	return &tmClient{
		rpcClient: rpcClient,
		svcTags: metrics.Tags{
			"svc": string("tendermint"),
		},
	}
}

// GetBlock queries for a block by height. An error is returned if the query fails.
func (c *tmClient) GetBlock(ctx context.Context, height int64) (*comettypes.ResultBlock, error) {
	return c.rpcClient.Block(ctx, &height)
}

// GetLatestBlockHeight returns the latest block height on the active chain.
func (c *tmClient) GetLatestBlockHeight(ctx context.Context) (int64, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	status, err := c.rpcClient.Status(ctx)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return -1, err
	}

	height := status.SyncInfo.LatestBlockHeight

	return height, nil
}

// GetTxs queries for all the transactions in a block height.
// It uses `Tx` RPC method to query for the transaction.
func (c *tmClient) GetTxs(ctx context.Context, block *comettypes.ResultBlock) ([]*comettypes.ResultTx, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	txs := make([]*comettypes.ResultTx, 0, len(block.Block.Txs))
	for _, tmTx := range block.Block.Txs {
		tx, err := c.rpcClient.Tx(ctx, tmTx.Hash(), true)
		if err != nil {
			if strings.HasSuffix(err.Error(), "not found") {
				metrics.ReportFuncError(c.svcTags)
				log.WithError(err).Errorln("failed to get Tx by hash")
				continue
			}

			return nil, err
		}

		txs = append(txs, tx)
	}

	return txs, nil
}

// GetValidatorSet returns all the known Tendermint validators for a given block
// height. An error is returned if the query fails.
func (c *tmClient) GetValidatorSet(ctx context.Context, height int64) (*comettypes.ResultValidators, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	return c.rpcClient.Validators(ctx, &height, nil, nil)
}
