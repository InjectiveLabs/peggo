package cosmos

import (
	"context"
	"time"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/peggy"
)

type Network interface {
	GetBlockTime(ctx context.Context, height int64) (time.Time, error)

	peggy.QueryClient
	peggy.BroadcastClient
}
