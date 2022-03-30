package relayer

import (
	ethcmn "github.com/ethereum/go-ethereum/common"
)

// SymbolRetriever wraps the interface to convert an ethereum contract address
// into an token symbol like 0xdAC17F958D2ee523a2206206994597C13D831ec7 -> USDT
type SymbolRetriever interface {
	// GetTokenSymbol executes an API client to retrieve the
	// correct Symbol from the contract address
	GetTokenSymbol(erc20Contract ethcmn.Address) (string, error)
}
