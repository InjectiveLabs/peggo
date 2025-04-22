package pricefeed

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
)

type DummyCoingeckoFeed struct {
	tokens map[string]string // token_addr -> denom
}

func NewDummyCoingeckoFeed() DummyCoingeckoFeed {
	return DummyCoingeckoFeed{
		tokens: map[string]string{
			"0x7E5C521F8515017487750c13C3bF3B15f3f5f654": "inj",
			"0x1ccec198630F2024c64C0aFC5aE2427bc8e2dce8": "wut",
		},
	}
}

func (f DummyCoingeckoFeed) QueryUSDPrice(address common.Address) (float64, error) {
	switch f.tokens[address.Hex()] {
	case "inj", "wut":
		return 10, nil
	default:
		return 0, errors.New("unknown token")
	}
}
