package orchestrator

import eth "github.com/ethereum/go-ethereum/common"

// PriceFeed provides token price for a given contract address
type PriceFeed interface {
	QueryUSDPrice(address eth.Address) (float64, error)
}
