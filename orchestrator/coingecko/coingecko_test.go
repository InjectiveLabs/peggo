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

func TestGetTokenSymbol(t *testing.T) {
	coinGecko := NewCoingecko(logger, nil)
	symbol := "UMEE"
	umeeContractAddr := ethcmn.HexToAddress("0xc0a4Df35568F116C370E6a6A6022Ceb908eedDaC")
	coinGecko.setCoinSymbol(umeeContractAddr, symbol)

	requestedSymbol, err := coinGecko.GetTokenSymbol(umeeContractAddr)
	assert.Nil(t, err)
	assert.Equal(t, symbol, requestedSymbol)
}

func TestRequestCoinSymbol(t *testing.T) {
	t.Run("get umee symbol from contract", func(t *testing.T) {
		expected := `{"symbol": "umee"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()

		coinGecko := NewCoingecko(logger, &Config{BaseURL: svr.URL})
		symbol, err := coinGecko.requestCoinSymbol(ethcmn.HexToAddress("0xc0a4Df35568F116C370E6a6A6022Ceb908eedDaC"))
		assert.Nil(t, err)
		assert.Equal(t, symbol, "UMEE")
	})

	t.Run("symbol not found", func(t *testing.T) {
		expected := `{"error": "Could not find coin with the given id"}`
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expected)
		}))
		defer svr.Close()

		coinGecko := NewCoingecko(logger, &Config{BaseURL: svr.URL})
		symbol, err := coinGecko.requestCoinSymbol(ethcmn.HexToAddress("----"))
		assert.EqualError(t, err, "coin info request failed: Could not find coin with the given id")
		assert.Equal(t, symbol, "")
	})
}

func TestGetRequestCoinSymbolURL(t *testing.T) {
	coinGecko := NewCoingecko(logger, nil)
	url, err := coinGecko.getRequestCoinSymbolURL(ethcmn.HexToAddress("0xc0a4Df35568F116C370E6a6A6022Ceb908eedDaC"))
	assert.Nil(t, err)
	assert.Equal(t, url.String(), "https://api.coingecko.com/api/v3/coins/ethereum/contract/0xc0a4Df35568F116C370E6a6A6022Ceb908eedDaC")
}

func TestCheckCoingeckoConfig(t *testing.T) {
	assert.NotNil(t, checkCoingeckoConfig(nil))
	assert.NotNil(t, checkCoingeckoConfig(&Config{BaseURL: ""}))
}
