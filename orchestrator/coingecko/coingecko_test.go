package coingecko

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).Level(zerolog.DebugLevel).With().Timestamp().Logger()

func TestQueryETHUSDPrice(t *testing.T) {
	t.Run("ok", func(t *testing.T) {

		expected := `{"ethereum": {"usd": 4271.57}}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		price, err := coingeckoFeed.QueryETHUSDPrice()
		assert.NoError(t, err)
		assert.Equal(t, 4271.57, price)
	})

	t.Run("failed to parse response body", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		_, err := coingeckoFeed.QueryETHUSDPrice()
		assert.EqualError(t, err, "failed to parse response body from "+svr.URL+"/simple/price?ids=ethereum&vs_currencies=usd: EOF")
	})

	t.Run("price is zero", func(t *testing.T) {

		expected := `{"ethereum": {"usd": 0.0}}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		price, err := coingeckoFeed.QueryETHUSDPrice()
		assert.EqualError(t, err, "failed to get price for Ethereum")
		assert.Equal(t, 0.0, price)
	})
}

func TestQueryUSDPrice(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		expected := `{"0xdac17f958d2ee523a2206206994597c13d831ec7":{"usd":0.998233}}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		price, err := coingeckoFeed.QueryUSDPrice(ethcmn.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"))
		assert.NoError(t, err)
		assert.Equal(t, 0.998233, price)
	})

	t.Run("failed to parse response body", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		_, err := coingeckoFeed.QueryUSDPrice(ethcmn.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"))
		assert.EqualError(t, err, "failed to parse response body from "+svr.URL+"/simple/token_price/ethereum?contract_addresses=0xdac17f958d2ee523a2206206994597c13d831ec7&vs_currencies=usd: EOF")
	})

	t.Run("price is zero", func(t *testing.T) {
		expected := `{"0xdac17f958d2ee523a2206206994597c13d831ec7":{"usd":0.0}}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()
		coingeckoFeed := NewCoingeckoPriceFeed(logger, 100, &Config{BaseURL: svr.URL})

		price, err := coingeckoFeed.QueryUSDPrice(ethcmn.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7"))
		assert.EqualError(t, err, "failed to get price for token 0xdAC17F958D2ee523a2206206994597C13D831ec7")
		assert.Equal(t, 0.0, price)
	})
}

func TestCheckCoingeckoConfig(t *testing.T) {
	assert.NotNil(t, checkCoingeckoConfig(nil))
	assert.NotNil(t, checkCoingeckoConfig(&Config{BaseURL: ""}))
}
