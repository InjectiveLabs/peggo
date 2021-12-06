package peggy

import (
	"bytes"
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// PendingTxInput contains the data of a pending transaction and the time we first saw it.
type PendingTxInput struct {
	InputData    hexutil.Bytes
	ReceivedTime time.Time
}

type PendingTxInputList []PendingTxInput

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	Input hexutil.Bytes `json:"input"`
}

// AddPendingTxInput adds pending submitBatch and updateBatch calls to the Peggy contract to the list of pending
// transactions, any other transaction is ignored.
func (p *PendingTxInputList) AddPendingTxInput(pendingTx *RPCTransaction) {

	submitBatchMethod := peggyABI.Methods["submitBatch"]
	valsetUpdateMethod := peggyABI.Methods["updateValset"]

	// If it's not a submitBatch or updateValset transaction, ignore it.
	// The first four bytes of the call data for a function call specifies the function to be called.
	// Ref: https://docs.soliditylang.org/en/develop/abi-spec.html#function-selector
	if !bytes.Equal(submitBatchMethod.ID, pendingTx.Input[:4]) &&
		!bytes.Equal(valsetUpdateMethod.ID, pendingTx.Input[:4]) {
		return
	}

	pendingTxInput := PendingTxInput{
		InputData:    pendingTx.Input,
		ReceivedTime: time.Now(),
	}

	*p = append(*p, pendingTxInput)
	// Persisting top 100 pending txs of peggy contract only.
	if len(*p) > 100 {
		(*p)[0] = PendingTxInput{} // to avoid memory leak
		// Dequeue pending tx input
		*p = (*p)[1:]
	}
}

func (s *peggyContract) IsPendingTxInput(txData []byte, pendingTxWaitDuration time.Duration) bool {
	t := time.Now()

	for _, pendingTxInput := range s.pendingTxInputList {
		if bytes.Equal(pendingTxInput.InputData, txData) {
			// If this tx was for too long in the pending list, consider it stale
			return t.Before(pendingTxInput.ReceivedTime.Add(pendingTxWaitDuration))
		}
	}
	return false
}

func (s *peggyContract) SubscribeToPendingTxs(ctx context.Context, alchemyWebsocketURL string) error {
	args := map[string]interface{}{
		"address": s.peggyAddress.Hex(),
	}

	wsClient, err := rpc.Dial(alchemyWebsocketURL)
	if err != nil {
		s.logger.Fatal().
			AnErr("err", err).
			Str("endpoint", alchemyWebsocketURL).
			Msg("failed to connect to Alchemy websocket")
		return err
	}

	ch := make(chan *RPCTransaction)
	_, err = wsClient.EthSubscribe(ctx, ch, "alchemy_filteredNewFullPendingTransactions", args)
	if err != nil {
		s.logger.Fatal().
			AnErr("err", err).
			Str("endpoint", alchemyWebsocketURL).
			Msg("Failed to subscribe to pending transactions")
		return err
	}

	for {
		select {
		case pendingTransaction := <-ch:
			s.pendingTxInputList.AddPendingTxInput(pendingTransaction)

		case <-ctx.Done():
			return nil
		}
	}
}

func (s *peggyContract) GetPendingTxInputList() *PendingTxInputList {
	return &s.pendingTxInputList
}
