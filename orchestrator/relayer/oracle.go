package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Oracle defines the Oracle interface that the relayer depends on.
type Oracle interface {
	// GetPrices returns the price for the provided base symbols.
	GetPrices(baseSymbols ...string) (map[string]sdk.Dec, error)

	// GetPrice returns the price based on the base symbol ex.: UMEE, ETH.
	GetPrice(baseSymbol string) (sdk.Dec, error)

	// SubscribeSymbols attempts to subscribe the symbols in all the providers.
	// baseSymbols is the base to be subscribed ex.: ["UMEE", "ATOM"].
	SubscribeSymbols(baseSymbols ...string) error
}
