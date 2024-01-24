package query

import (
	"context"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/InjectiveLabs/metrics"
	peggytypes "github.com/InjectiveLabs/sdk-go/chain/peggy/types"
)

var ErrNotFound = errors.New("not found")

type PeggyQueryClient struct {
	peggytypes.QueryClient

	svcTags metrics.Tags
}

func (c PeggyQueryClient) ValsetAt(ctx context.Context, nonce uint64) (*peggytypes.Valset, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryValsetRequestRequest{Nonce: nonce}

	resp, err := c.QueryClient.ValsetRequest(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query ValsetRequest from client")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Valset, nil
}

func (c PeggyQueryClient) CurrentValset(ctx context.Context) (*peggytypes.Valset, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.CurrentValset(ctx, &peggytypes.QueryCurrentValsetRequest{})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query CurrentValset from client")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Valset, nil
}

func (c PeggyQueryClient) OldestUnsignedValsets(ctx context.Context, valAccountAddress sdktypes.AccAddress) ([]*peggytypes.Valset, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryLastPendingValsetRequestByAddrRequest{
		Address: valAccountAddress.String(),
	}

	resp, err := c.QueryClient.LastPendingValsetRequestByAddr(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query LastPendingValsetRequestByAddr from client")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Valsets, nil
}

func (c PeggyQueryClient) LatestValsets(ctx context.Context) ([]*peggytypes.Valset, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.LastValsetRequests(ctx, &peggytypes.QueryLastValsetRequestsRequest{})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query LastValsetRequests from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Valsets, nil
}

func (c PeggyQueryClient) AllValsetConfirms(ctx context.Context, nonce uint64) ([]*peggytypes.MsgValsetConfirm, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.ValsetConfirmsByNonce(ctx, &peggytypes.QueryValsetConfirmsByNonceRequest{Nonce: nonce})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query ValsetConfirmsByNonce from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Confirms, nil
}

func (c PeggyQueryClient) OldestUnsignedTransactionBatch(ctx context.Context, valAccountAddress sdktypes.AccAddress) (*peggytypes.OutgoingTxBatch, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryLastPendingBatchRequestByAddrRequest{
		Address: valAccountAddress.String(),
	}

	resp, err := c.QueryClient.LastPendingBatchRequestByAddr(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query LastPendingBatchRequestByAddr from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Batch, nil
}

func (c PeggyQueryClient) LatestTransactionBatches(ctx context.Context) ([]*peggytypes.OutgoingTxBatch, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.OutgoingTxBatches(ctx, &peggytypes.QueryOutgoingTxBatchesRequest{})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query OutgoingTxBatches from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Batches, nil
}

func (c PeggyQueryClient) UnbatchedTokensWithFees(ctx context.Context) ([]*peggytypes.BatchFees, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.BatchFees(ctx, &peggytypes.QueryBatchFeeRequest{})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query BatchFees from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.BatchFees, nil
}

func (c PeggyQueryClient) TransactionBatchSignatures(ctx context.Context, nonce uint64, tokenContract common.Address) ([]*peggytypes.MsgConfirmBatch, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryBatchConfirmsRequest{
		Nonce:           nonce,
		ContractAddress: tokenContract.String(),
	}

	resp, err := c.QueryClient.BatchConfirms(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query BatchConfirms from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.Confirms, nil
}

func (c PeggyQueryClient) LastClaimEventByAddr(ctx context.Context, validatorAccountAddress sdktypes.AccAddress) (*peggytypes.LastClaimEvent, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryLastEventByAddrRequest{
		Address: validatorAccountAddress.String(),
	}

	resp, err := c.QueryClient.LastEventByAddr(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query LastEventByAddr from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return resp.LastClaimEvent, nil
}

func (c PeggyQueryClient) PeggyParams(ctx context.Context) (*peggytypes.Params, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	resp, err := c.QueryClient.Params(ctx, &peggytypes.QueryParamsRequest{})
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query PeggyParams from daemon")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	return &resp.Params, nil
}

func (c PeggyQueryClient) GetValidatorAddress(ctx context.Context, addr common.Address) (sdktypes.AccAddress, error) {
	metrics.ReportFuncCall(c.svcTags)
	doneFn := metrics.ReportFuncTiming(c.svcTags)
	defer doneFn()

	req := &peggytypes.QueryDelegateKeysByEthAddress{
		EthAddress: addr.Hex(),
	}

	resp, err := c.QueryClient.GetDelegateKeyByEth(ctx, req)
	if err != nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, errors.Wrap(err, "failed to query GetDelegateKeyByEth from client")
	}

	if resp == nil {
		metrics.ReportFuncError(c.svcTags)
		return nil, ErrNotFound
	}

	valAddr, err := sdktypes.AccAddressFromBech32(resp.ValidatorAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode validator address: %v", resp.ValidatorAddress)
	}

	return valAddr, nil
}
