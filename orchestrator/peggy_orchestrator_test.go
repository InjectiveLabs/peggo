package orchestrator

import (
	"context"
	"errors"
	"github.com/InjectiveLabs/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	peggyevents "github.com/InjectiveLabs/peggo/solidity/wrappers/Peggy.sol"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	comettypes "github.com/cometbft/cometbft/rpc/core/types"
	comet "github.com/cometbft/cometbft/types"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	gethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"time"
)

func Test_Orchestrator_Loops(t *testing.T) {
	t.Parallel()

	// faster test runs
	maxRetryAttempts = 1

	t.Run("batch requester", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name     string
			expected error
			orch     *Orchestrator
			inj      cosmos.Network
			eth      ethereum.Network
		}{
			{
				name:     "failed to get token fees",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					UnbatchedTokensWithFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "no unbatched tokens",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					UnbatchedTokensWithFeesFn: func(context.Context) ([]*peggytypes.BatchFees, error) {
						return nil, nil
					},
				},
			},

			{
				name:     "batch does not meet fee threshold",
				expected: nil,
				orch: &Orchestrator{
					logger:         DummyLog,
					priceFeed:      MockPriceFeed{QueryUSDPriceFn: func(_ gethcommon.Address) (float64, error) { return 1, nil }},
					minBatchFeeUSD: 51.0,
					erc20ContractMapping: map[gethcommon.Address]string{
						gethcommon.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"): "inj",
					},
				},
				inj: MockCosmosNetwork{
					SendRequestBatchFn: func(context.Context, string) error { return nil },
					UnbatchedTokensWithFeesFn: func(context.Context) ([]*peggytypes.BatchFees, error) {
						fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
						return []*peggytypes.BatchFees{
							{
								Token:     gethcommon.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30").String(),
								TotalFees: fees,
							},
						}, nil
					},
				},
			},

			{
				name:     "batch meets threshold and a request is sent",
				expected: nil,
				orch: &Orchestrator{
					logger:         DummyLog,
					priceFeed:      MockPriceFeed{QueryUSDPriceFn: func(_ gethcommon.Address) (float64, error) { return 1, nil }},
					minBatchFeeUSD: 49.0,
					erc20ContractMapping: map[gethcommon.Address]string{
						gethcommon.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30"): "inj",
					},
				},
				inj: MockCosmosNetwork{
					SendRequestBatchFn: func(context.Context, string) error { return nil },
					UnbatchedTokensWithFeesFn: func(_ context.Context) ([]*peggytypes.BatchFees, error) {
						fees, _ := cosmtypes.NewIntFromString("50000000000000000000")
						return []*peggytypes.BatchFees{{
							Token:     gethcommon.HexToAddress("0xe28b3B32B6c345A34Ff64674606124Dd5Aceca30").String(),
							TotalFees: fees,
						}}, nil
					},
				},
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := batchRequester{
					Orchestrator: tt.orch,
					Injective:    tt.inj,
				}

				assert.ErrorIs(t, r.RequestBatches(context.Background()), tt.expected)
			})
		}
	})

	t.Run("oracle", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name                    string
			expected                error
			orch                    *Orchestrator
			inj                     cosmos.Network
			eth                     ethereum.Network
			lastResyncWithInjective time.Time
			lastObservedEthHeight   uint64
		}{
			{
				name:     "failed to get current valset",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger: DummyLog,
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "orchestrator not bonded",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x76D2dDbb89C36FA39FAa5c5e7C61ee95AC4D76C4"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},
				},
			},

			{
				name:     "failed to get latest eth height",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return nil, errors.New("fail")
					},
				},
			},

			{
				name:     "not enough block on ethereum",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(10)}, nil // minimum is 12
					},
				},
			},

			{
				name:     "failed to get ethereum events",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return nil, errors.New("oops")
					},
				},
				lastObservedEthHeight: 100,
			},

			{
				name:     "failed to get last claim event",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},

					LastClaimEventByAddrFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
						return nil, errors.New("oops")
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return []*peggyevents.PeggySendToCosmosEvent{
							{
								EventNonce: big.NewInt(100),
							},
						}, nil
					},

					GetValsetUpdatedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
						return nil, nil
					},
					GetSendToInjectiveEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
						return nil, nil
					},
					GetTransactionBatchExecutedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
						return nil, nil
					},
					GetPeggyERC20DeployedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
						return nil, nil
					},
				},
				lastObservedEthHeight: 100,
			},

			{
				name:     "no new events",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},

					LastClaimEventByAddrFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
						return &peggytypes.LastClaimEvent{
							EthereumEventNonce: 101,
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return []*peggyevents.PeggySendToCosmosEvent{
							{
								EventNonce: big.NewInt(100),
							},
						}, nil
					},

					GetValsetUpdatedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
						return nil, nil
					},
					GetSendToInjectiveEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
						return nil, nil
					},
					GetTransactionBatchExecutedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
						return nil, nil
					},
					GetPeggyERC20DeployedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
						return nil, nil
					},
				},
				lastObservedEthHeight: 100,
			},

			{
				name:     "missed events triggers resync",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},

					LastClaimEventByAddrFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
						return &peggytypes.LastClaimEvent{
							EthereumEventNonce:  102,
							EthereumEventHeight: 1000,
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return []*peggyevents.PeggySendToCosmosEvent{
							{
								EventNonce: big.NewInt(104),
							},
						}, nil
					},

					GetValsetUpdatedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
						return nil, nil
					},
					GetSendToInjectiveEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
						return nil, nil
					},
					GetTransactionBatchExecutedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
						return nil, nil
					},
					GetPeggyERC20DeployedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
						return nil, nil
					},
				},
				lastObservedEthHeight: 100,
			},

			{
				name:     "sent new event claim",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},

					LastClaimEventByAddrFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
						return &peggytypes.LastClaimEvent{
							EthereumEventNonce:  102,
							EthereumEventHeight: 1000,
						}, nil
					},

					SendOldDepositClaimFn: func(_ context.Context, _ *peggyevents.PeggySendToCosmosEvent) error {
						return nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return []*peggyevents.PeggySendToCosmosEvent{
							{
								EventNonce: big.NewInt(103),
							},
						}, nil
					},

					GetValsetUpdatedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
						return nil, nil
					},
					GetSendToInjectiveEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
						return nil, nil
					},
					GetTransactionBatchExecutedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
						return nil, nil
					},
					GetPeggyERC20DeployedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
						return nil, nil
					},
				},
				lastObservedEthHeight:   100,
				lastResyncWithInjective: time.Now(), // skip auto resync
			},

			{
				name:     "auto resync",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					ethAddr: gethcommon.HexToAddress("0x3959f5246c452463279F690301D923D5a75bbD88"),
				},
				inj: MockCosmosNetwork{
					CurrentValsetFn: func(_ context.Context) (*peggytypes.Valset, error) {
						return &peggytypes.Valset{
							Members: []*peggytypes.BridgeValidator{
								{
									EthereumAddress: "0x3959f5246c452463279F690301D923D5a75bbD88",
								},
							},
						}, nil
					},

					LastClaimEventByAddrFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
						return &peggytypes.LastClaimEvent{
							EthereumEventNonce:  102,
							EthereumEventHeight: 1000,
						}, nil
					},

					SendOldDepositClaimFn: func(_ context.Context, _ *peggyevents.PeggySendToCosmosEvent) error {
						return nil
					},
				},
				eth: MockEthereumNetwork{
					GetHeaderByNumberFn: func(context.Context, *big.Int) (*types.Header, error) {
						return &types.Header{Number: big.NewInt(2100)}, nil
					},
					GetSendToCosmosEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToCosmosEvent, error) {
						return []*peggyevents.PeggySendToCosmosEvent{
							{
								EventNonce: big.NewInt(103),
							},
						}, nil
					},

					GetValsetUpdatedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyValsetUpdatedEvent, error) {
						return nil, nil
					},
					GetSendToInjectiveEventsFn: func(_, _ uint64) ([]*peggyevents.PeggySendToInjectiveEvent, error) {
						return nil, nil
					},
					GetTransactionBatchExecutedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyTransactionBatchExecutedEvent, error) {
						return nil, nil
					},
					GetPeggyERC20DeployedEventsFn: func(_, _ uint64) ([]*peggyevents.PeggyERC20DeployedEvent, error) {
						return nil, nil
					},
				},
				lastObservedEthHeight: 100,
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				o := ethOracle{
					Orchestrator:            tt.orch,
					Injective:               tt.inj,
					Ethereum:                tt.eth,
					LastResyncWithInjective: tt.lastResyncWithInjective,
					LastObservedEthHeight:   tt.lastObservedEthHeight,
				}

				err := o.ObserveEthEvents(context.Background())
				if tt.expected == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("relayer valset", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name     string
			expected error
			orch     *Orchestrator
			inj      cosmos.Network
			eth      ethereum.Network
		}{
			{
				name:     "failed to get latest valset updates",
				expected: errors.New("oops"),
				orch:     &Orchestrator{svcTags: metrics.Tags{"svc": "relayer"}},
				inj: MockCosmosNetwork{
					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "failed to get valset confirmations",
				expected: errors.New("oops"),
				orch:     &Orchestrator{svcTags: metrics.Tags{"svc": "relayer"}},
				inj: MockCosmosNetwork{
					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "no new valset to relay",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return nil, nil
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return nil, nil
					},
				},
			},

			{
				name:     "no new valset to relay",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return nil, errors.New("oops")
					},
				},

				inj: MockCosmosNetwork{
					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},

			{
				name:     "valset already updated",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return big.NewInt(101), nil
					},
				},

				inj: MockCosmosNetwork{
					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},

			{
				name:     "failed to get injective block",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return big.NewInt(99), nil
					},
				},

				inj: MockCosmosNetwork{
					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return nil, errors.New("oops")
					},

					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},

			{
				name:     "relay valset offser duration not expired",
				expected: nil,
				orch: &Orchestrator{
					logger:               DummyLog,
					svcTags:              metrics.Tags{"svc": "relayer"},
					relayValsetOffsetDur: 10 * time.Second,
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return big.NewInt(99), nil
					},
				},

				inj: MockCosmosNetwork{
					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},

					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},

			{
				name:     "failed to send valset update",
				expected: nil,
				orch: &Orchestrator{
					logger:               DummyLog,
					svcTags:              metrics.Tags{"svc": "relayer"},
					relayValsetOffsetDur: 0,
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return big.NewInt(99), nil
					},

					SendEthValsetUpdateFn: func(_ context.Context, _ *peggytypes.Valset, _ *peggytypes.Valset, _ []*peggytypes.MsgValsetConfirm) (*gethcommon.Hash, error) {
						return nil, errors.New("oops")
					},
				},

				inj: MockCosmosNetwork{
					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},

					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},

			{
				name:     "sent valset update",
				expected: nil,
				orch: &Orchestrator{
					logger:               DummyLog,
					svcTags:              metrics.Tags{"svc": "relayer"},
					relayValsetOffsetDur: 0,
				},
				eth: MockEthereumNetwork{
					GetValsetNonceFn: func(_ context.Context) (*big.Int, error) {
						return big.NewInt(99), nil
					},

					SendEthValsetUpdateFn: func(_ context.Context, _ *peggytypes.Valset, _ *peggytypes.Valset, _ []*peggytypes.MsgValsetConfirm) (*gethcommon.Hash, error) {
						return &gethcommon.Hash{}, nil
					},
				},

				inj: MockCosmosNetwork{
					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},

					LatestValsetsFn: func(_ context.Context) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil // non-empty will do
					},

					AllValsetConfirmsFn: func(_ context.Context, _ uint64) ([]*peggytypes.MsgValsetConfirm, error) {
						return []*peggytypes.MsgValsetConfirm{{}}, nil // non-empty will do
					},
				},
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := relayer{
					Orchestrator: tt.orch,
					Injective:    tt.inj,
					Ethereum:     tt.eth,
				}

				latestEthValset := &peggytypes.Valset{
					Nonce: 101,
				}

				err := r.relayValset(context.Background(), latestEthValset)
				if tt.expected == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("relayer batches", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name     string
			expected error
			orch     *Orchestrator
			inj      cosmos.Network
			eth      ethereum.Network
		}{
			{
				name:     "failed to get latest batches",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "failed to get batch confirmations",
				expected: errors.New("oops"),
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "no batch to relay",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return nil, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},
				},
			},

			{
				name:     "failed to get latest batch nonce",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "batch already updated",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{
							BatchNonce: 100,
						}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return big.NewInt(100), nil
					},
				},
			},

			{
				name:     "failed to get injective block",
				expected: nil,
				orch: &Orchestrator{
					logger:  DummyLog,
					svcTags: metrics.Tags{"svc": "relayer"},
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{
							BatchNonce: 101,
						}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},

					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return nil, errors.New("oops")
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return big.NewInt(100), nil
					},
				},
			},

			{
				name:     "batch relay offset not expired",
				expected: nil,
				orch: &Orchestrator{
					logger:              DummyLog,
					svcTags:             metrics.Tags{"svc": "relayer"},
					relayBatchOffsetDur: 10 * time.Second,
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{
							BatchNonce: 101,
						}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},

					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return big.NewInt(100), nil
					},
				},
			},

			{
				name:     "failed to send batch update",
				expected: nil,
				orch: &Orchestrator{
					logger:              DummyLog,
					svcTags:             metrics.Tags{"svc": "relayer"},
					relayBatchOffsetDur: 0,
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{
							BatchNonce: 101,
						}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},

					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return big.NewInt(100), nil
					},

					SendTransactionBatchFn: func(_ context.Context, _ *peggytypes.Valset, _ *peggytypes.OutgoingTxBatch, _ []*peggytypes.MsgConfirmBatch) (*gethcommon.Hash, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "sent batch update",
				expected: nil,
				orch: &Orchestrator{
					logger:              DummyLog,
					svcTags:             metrics.Tags{"svc": "relayer"},
					relayBatchOffsetDur: 0,
				},
				inj: MockCosmosNetwork{
					LatestTransactionBatchesFn: func(_ context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
						return []*peggytypes.OutgoingTxBatch{{
							BatchNonce: 101,
						}}, nil
					},

					TransactionBatchSignaturesFn: func(_ context.Context, _ uint64, _ gethcommon.Address) ([]*peggytypes.MsgConfirmBatch, error) {
						return []*peggytypes.MsgConfirmBatch{{}}, nil
					},

					GetBlockFn: func(_ context.Context, _ int64) (*comettypes.ResultBlock, error) {
						return &comettypes.ResultBlock{
							Block: &comet.Block{
								Header: comet.Header{Time: time.Now()},
							},
						}, nil
					},
				},
				eth: MockEthereumNetwork{
					GetTxBatchNonceFn: func(_ context.Context, _ gethcommon.Address) (*big.Int, error) {
						return big.NewInt(100), nil
					},

					SendTransactionBatchFn: func(_ context.Context, _ *peggytypes.Valset, _ *peggytypes.OutgoingTxBatch, _ []*peggytypes.MsgConfirmBatch) (*gethcommon.Hash, error) {
						return &gethcommon.Hash{}, nil
					},
				},
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				r := relayer{
					Orchestrator: tt.orch,
					Injective:    tt.inj,
					Ethereum:     tt.eth,
				}

				latestEthValset := &peggytypes.Valset{
					Nonce: 101,
				}

				err := r.relayBatch(context.Background(), latestEthValset)
				if tt.expected == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("signer valsets", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name     string
			expected error
			orch     *Orchestrator
			inj      cosmos.Network
		}{
			{
				name:     "failed to get unsigned valsets",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedValsetsFn: func(_ context.Context, _ cosmtypes.AccAddress) ([]*peggytypes.Valset, error) {
						return nil, errors.New("oops")
					},
				},
			},

			{
				name:     "no valset updates to sign",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedValsetsFn: func(_ context.Context, _ cosmtypes.AccAddress) ([]*peggytypes.Valset, error) {
						return nil, nil
					},
				},
			},

			{
				name:     "failed to send valset confirm",
				expected: errors.New("oops"),
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedValsetsFn: func(_ context.Context, _ cosmtypes.AccAddress) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil
					},

					SendValsetConfirmFn: func(_ context.Context, _ gethcommon.Address, _ gethcommon.Hash, _ *peggytypes.Valset) error {
						return errors.New("oops")
					},
				},
			},

			{
				name:     "sent valset confirm",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedValsetsFn: func(_ context.Context, _ cosmtypes.AccAddress) ([]*peggytypes.Valset, error) {
						return []*peggytypes.Valset{{}}, nil
					},

					SendValsetConfirmFn: func(_ context.Context, _ gethcommon.Address, _ gethcommon.Hash, _ *peggytypes.Valset) error {
						return nil
					},
				},
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := ethSigner{
					Orchestrator: tt.orch,
					Injective:    tt.inj,
				}

				err := s.signNewValsetUpdates(context.Background())
				if tt.expected == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})

	t.Run("signer batches", func(t *testing.T) {
		t.Parallel()

		testTable := []struct {
			name     string
			expected error
			orch     *Orchestrator
			inj      cosmos.Network
		}{
			{
				name:     "failed to get unsigned batches/no batch to confirm",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedTransactionBatchFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.OutgoingTxBatch, error) {
						return nil, errors.New("ooops")
					},
				},
			},

			{
				name:     "failed to send batch confirm",
				expected: errors.New("oops"),
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedTransactionBatchFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.OutgoingTxBatch, error) {
						return &peggytypes.OutgoingTxBatch{}, nil
					},

					SendBatchConfirmFn: func(_ context.Context, _ gethcommon.Address, _ gethcommon.Hash, _ *peggytypes.OutgoingTxBatch) error {
						return errors.New("oops")
					},
				},
			},

			{
				name:     "sent batch confirm",
				expected: nil,
				orch:     &Orchestrator{logger: DummyLog},
				inj: MockCosmosNetwork{
					OldestUnsignedTransactionBatchFn: func(_ context.Context, _ cosmtypes.AccAddress) (*peggytypes.OutgoingTxBatch, error) {
						return &peggytypes.OutgoingTxBatch{}, nil
					},

					SendBatchConfirmFn: func(_ context.Context, _ gethcommon.Address, _ gethcommon.Hash, _ *peggytypes.OutgoingTxBatch) error {
						return nil
					},
				},
			},
		}

		for _, tt := range testTable {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s := ethSigner{
					Orchestrator: tt.orch,
					Injective:    tt.inj,
				}

				err := s.signNewBatch(context.Background())
				if tt.expected == nil {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})
}
