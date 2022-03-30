package coingecko

import (
	"encoding/json"
	"fmt"
	"strings"

	"net/http"
	"net/url"
	"path"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
)

const (
	maxRespTime        = 15 * time.Second
	maxRespHeadersTime = 15 * time.Second
	EthereumCoinID     = "ethereum"
)

type (
	// CoinGecko wraps the client to retrieve information from their API.
	CoinGecko struct {
		client *http.Client
		config *Config

		coinsSymbol map[ethcmn.Address]string // contract addr => token symbol

		logger zerolog.Logger
	}

	// Config wraps the config variable to get CoinGecko information.
	Config struct {
		BaseURL string
	}

	// CoinInfo wraps the coin information received from a contract address.
	//
	// Ref : https://api.coingecko.com/api/v3/coins/ethereum/contract/${CONTRACT_ADDR}
	CoinInfo struct {
		Symbol string `json:"symbol"`
		Error  string `json:"error"`
	}
)

// NewCoingecko grabs the symbol, given a contract address.
func NewCoingecko(logger zerolog.Logger, endpointConfig *Config) *CoinGecko {
	return &CoinGecko{
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: maxRespHeadersTime,
			},
			Timeout: maxRespTime,
		},
		config:      checkCoingeckoConfig(endpointConfig),
		coinsSymbol: bridgeTokensCoinSymbols,
		logger:      logger.With().Str("oracle", "coingecko").Logger(),
	}
}

func urlJoin(baseURL string, segments ...string) (*url.URL, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(append([]string{u.Path}, segments...)...)
	return u, nil
}

// GetTokenSymbol returns the token symbol checked by CoinGecko API.
func (cp *CoinGecko) GetTokenSymbol(erc20Contract ethcmn.Address) (string, error) {
	symbol, ok := cp.coinsSymbol[erc20Contract]
	if !ok {
		symbol, err := cp.requestCoinSymbol(erc20Contract)
		if err != nil {
			return "", err
		}
		cp.setCoinSymbol(erc20Contract, symbol)

		return symbol, nil
	}

	return symbol, nil
}

func (cp *CoinGecko) setCoinSymbol(erc20Contract ethcmn.Address, symbol string) {
	cp.coinsSymbol[erc20Contract] = symbol
}

func (cp *CoinGecko) getRequestCoinSymbolURL(erc20Contract ethcmn.Address) (*url.URL, error) {
	return urlJoin(cp.config.BaseURL, "coins", EthereumCoinID, "contract", erc20Contract.Hex())
}

func (cp *CoinGecko) requestCoinSymbol(erc20Contract ethcmn.Address) (string, error) {
	u, err := cp.getRequestCoinSymbolURL(erc20Contract)
	if err != nil {
		cp.logger.Err(err).Msg("failed to parse coin info URL")
	}

	reqURL := u.String()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		cp.logger.Err(err).Msg("failed to create HTTP request for coin info")
	}

	resp, err := cp.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch coin info from %s: %w", reqURL, err)
	}
	defer resp.Body.Close()

	var coinInfo CoinInfo
	if err := json.NewDecoder(resp.Body).Decode(&coinInfo); err != nil {
		return "", fmt.Errorf("failed to parse response body from %s: %w", reqURL, err)
	}

	if len(coinInfo.Error) > 0 {
		return "", fmt.Errorf("coin info request failed: %s", coinInfo.Error)
	}

	if len(coinInfo.Symbol) == 0 {
		return "", fmt.Errorf("fail to get coin info for contract: %s", erc20Contract.Hex())
	}

	return strings.ToUpper(coinInfo.Symbol), nil
}

func checkCoingeckoConfig(cfg *Config) *Config {
	if cfg == nil {
		cfg = &Config{}
	}

	if len(cfg.BaseURL) == 0 {
		cfg.BaseURL = "https://api.coingecko.com/api/v3"
	}

	return cfg
}
