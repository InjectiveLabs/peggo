package gravity

import (
	"testing"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
)

func TestEncodeValsetConfirm(t *testing.T) {
	gravityID := "defaultgravityid"

	valset := types.Valset{
		Nonce: 5,
		Members: []types.BridgeValidator{
			{Power: 1, EthereumAddress: "0x02fa1b44e2EF8436e6f35D5F56607769c658c225"},
			{Power: 123, EthereumAddress: "0x4f3a9f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f8f"},
		},
		Height:       111111,
		RewardAmount: sdk.NewInt(2),
		RewardToken:  "",
	}

	result := EncodeValsetConfirm(gravityID, valset)

	// Check the result with a previously calculated one.
	assert.Equal(t, "0xacfb0f575bd6e7ecbd77424461aa89340c2e876b1c4b177e9f9c6f92529d27ab", result.Hex())
}

func TestEncodeTxBatchConfirm(t *testing.T) {
	gravityID := "defaultgravityid"

	txBatch := types.OutgoingTxBatch{
		Transactions: []types.OutgoingTransferTx{
			{
				DestAddress: "0x02fa1b44e2EF8436e6f35D5F56607769c658c225",
				Erc20Token: types.ERC20Token{
					Contract: "0x4884e2a214dc5040f52a41c3f21c765283170b6e",
					Amount:   sdk.NewInt(100000),
				},
				Erc20Fee: types.ERC20Token{
					Contract: "0x4884e2a214dc5040f52a41c3f21c765283170b6e",
					Amount:   sdk.NewInt(2000),
				},
			},
		},
	}

	result := EncodeTxBatchConfirm(gravityID, txBatch)

	// Check the result with a previously calculated one.
	assert.Equal(t, "0xf78189166c4bf48863f7765ba1b29afe15c45c0e48b2fbdeaf43b15ed09c138c", result.Hex())
}
