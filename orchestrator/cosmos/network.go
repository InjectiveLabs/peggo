package cosmos

import (
	"context"
	"time"
)

type Network interface {
	GetBlockTime(ctx context.Context, height int64) (time.Time, error)

	PeggyQueryClient
	PeggyBroadcastClient
}
