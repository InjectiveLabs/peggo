package relayer

import (
	"context"
	"crypto/ecdsa"

	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/InjectiveLabs/peggo/orchestrator/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/sidechain"
	"github.com/ethereum/go-ethereum/common"
)

type PeggyRelayer interface {
	RelayBatches(ctx context.Context) error
}

type peggyRelayer struct {
	svcTags metrics.Tags

	ethPrivateKey     *ecdsa.PrivateKey
	ethProvider       *provider.EVMProvider
	ethCommitter      *committer.EVMCommitter
	cosmosQueryClient sidechain.PeggyQueryClient
	peggyContract     peggy.PeggyContract
}

func NewPeggyRelayer(
	ethPrivateKey *ecdsa.PrivateKey,
	ethProvider *provider.EVMProvider,
	ethCommitter *committer.EVMCommitter,
	cosmosQueryClient sidechain.PeggyQueryClient,
	peggyContract     peggy.PeggyContract
) PeggyRelayer {
	return &peggyRelayer{
		ethPrivateKey:     ethPrivateKey,
		ethProvider:       ethProvider,
		ethCommitter:      ethCommitter,
		cosmosQueryClient: cosmosQueryClient,
		peggyContract:     peggyContract,

		svcTags: metrics.Tags{
			"svc": "peggy_relayer",
		},
	}
}
