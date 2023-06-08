package orchestrator

import (
	"context"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/xlab/suplog"
	"testing"
)

type mockInjective struct {
}

func (i mockInjective) UnbatchedTokenFees(_ context.Context) ([]*peggytypes.BatchFees, error) {
	return nil, errors.New("fail")
}

func (i mockInjective) SendRequestBatch(_ context.Context, _ string) error {
	return nil
}

func TestRequestBatches(t *testing.T) {
	inj := mockInjective{}

	orch := &PeggyOrchestrator{
		injective: inj,
	}

	_ = orch

	assert.Nil(t, orch.requestBatches(context.Background(), suplog.DefaultLogger, false))
}
