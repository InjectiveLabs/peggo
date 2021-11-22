package coingecko

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).Level(zerolog.DebugLevel).With().Timestamp().Logger()

func TestQueryETHUSDPrice(t *testing.T) {
	expected := `{"ethereum": {"usd": 4271.57}}`
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expected)
	}))
	defer svr.Close()
	coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

	price, err := coingeckoFeed.QueryETHUSDPrice()
	assert.NoError(t, err)
	assert.Equal(t, 4271.57, price)
}

func TestQueryUSDPrice(t *testing.T) {
	expected := `{"0xdac17f958d2ee523a2206206994597c13d831ec7":{"usd":0.998233}}`
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, expected)
	}))
	defer svr.Close()
	coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

	price, err := coingeckoFeed.QueryUSDPrice(common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"))
	assert.NoError(t, err)
	assert.Equal(t, 0.998233, price)
}
