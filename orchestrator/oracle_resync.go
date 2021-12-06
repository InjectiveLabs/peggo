package orchestrator

import (
	"context"

	"github.com/pkg/errors"
	"github.com/umee-network/umee/x/peggy/types"
)

// GetLastCheckedBlock retrieves the Ethereum block height from the last claim event this oracle has relayed to Cosmos.
func (p *peggyOrchestrator) GetLastCheckedBlock(ctx context.Context) (uint64, error) {

	lastEventResp, err := p.cosmosQueryClient.LastEventByAddr(ctx, &types.QueryLastEventByAddrRequest{
		Address: p.peggyBroadcastClient.AccFromAddress().String(),
	})

	if err != nil {
		return uint64(0), err
	}

	if lastEventResp == nil {
		return 0, errors.New("no last event response returned")
	}

	return lastEventResp.LastClaimEvent.EthereumEventHeight, nil
}
