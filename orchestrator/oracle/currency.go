package oracle

import (
	"strings"

	umeepftypes "github.com/umee-network/umee/price-feeder/oracle/types"
)

const (
	symbolUSD  = "USD"
	symbolUSDT = "USDT"
	symbolDAI  = "DAI"
)

var (
	quoteStablecoins = []string{symbolUSD, symbolUSDT, symbolDAI}
)

// GetStablecoinsCurrencyPair return the currency pair of that symbol quoted by some
// stablecoins.
func GetStablecoinsCurrencyPair(baseSymbol string) []umeepftypes.CurrencyPair {
	currencyPairs := make([]umeepftypes.CurrencyPair, len(quoteStablecoins))

	for i, quote := range quoteStablecoins {
		currencyPairs[i] = umeepftypes.CurrencyPair{
			Base:  strings.ToUpper(baseSymbol),
			Quote: quote,
		}
	}

	return currencyPairs
}
