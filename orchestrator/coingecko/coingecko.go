package coingecko

import (
	"encoding/json"

	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	maxRespTime        = 15 * time.Second
	maxRespHeadersTime = 15 * time.Second
)

var zeroPrice = float64(0)

type PriceFeed struct {
	client *http.Client
	config *Config

	interval time.Duration

	logger zerolog.Logger
}

type Config struct {
	BaseURL string
}

func urlJoin(baseURL string, segments ...string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(append([]string{u.Path}, segments...)...)
	return u.String()

}

type priceResponse map[string]struct {
	USD float64 `json:"usd"`
}

func (cp *PriceFeed) QueryETHUSDPrice() (float64, error) {
	u, err := url.ParseRequestURI(urlJoin(cp.config.BaseURL, "simple", "price"))
	if err != nil {
		cp.logger.Fatal().Err(err).Msg("failed to parse URL")
	}

	q := make(url.Values)

	q.Set("ids", "ethereum")
	q.Set("vs_currencies", "usd")
	u.RawQuery = q.Encode()

	reqURL := u.String()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		cp.logger.Fatal().Err(err).Msg("failed to create HTTP request")
	}

	resp, err := cp.client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "failed to fetch price from %s", reqURL)
		return zeroPrice, err
	}

	defer resp.Body.Close()

	var respBody priceResponse

	err = json.NewDecoder(resp.Body).Decode(&respBody)

	if err != nil {
		return zeroPrice, errors.Wrapf(err, "failed to parse response body from %s", reqURL)
	}

	price := respBody["ethereum"].USD

	if price == zeroPrice {
		return zeroPrice, errors.Errorf("failed to get price for Ethereum")
	}

	return price, nil
}

func (cp *PriceFeed) QueryUSDPrice(erc20Contract common.Address) (float64, error) {

	u, err := url.ParseRequestURI(urlJoin(cp.config.BaseURL, "simple", "token_price", "ethereum"))
	if err != nil {
		cp.logger.Fatal().Err(err).Msg("failed to parse URL")
	}

	q := make(url.Values)

	q.Set("contract_addresses", strings.ToLower(erc20Contract.String()))
	q.Set("vs_currencies", "usd")
	u.RawQuery = q.Encode()

	reqURL := u.String()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		cp.logger.Fatal().Err(err).Msg("failed to create HTTP request")
	}

	resp, err := cp.client.Do(req)
	if err != nil {
		err = errors.Wrapf(err, "failed to fetch price from %s", reqURL)
		return zeroPrice, err
	}

	defer resp.Body.Close()

	var respBody priceResponse

	err = json.NewDecoder(resp.Body).Decode(&respBody)

	if err != nil {
		return zeroPrice, errors.Wrapf(err, "failed to parse response body from %s", reqURL)
	}

	price := respBody[strings.ToLower(erc20Contract.String())].USD

	if price == zeroPrice {
		return zeroPrice, errors.Errorf("failed to get price for token %s", erc20Contract.Hex())
	}

	return price, nil
}

// NewCoingeckoPriceFeed returns price puller for given symbol. The price will be pulled
// from endpoint and divided by scaleFactor. Symbol name (if reported by endpoint) must match.
func NewCoingeckoPriceFeed(logger zerolog.Logger, interval time.Duration, endpointConfig *Config) *PriceFeed {
	return &PriceFeed{
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: maxRespHeadersTime,
			},
			Timeout: maxRespTime,
		},
		config:   checkCoingeckoConfig(endpointConfig),
		interval: interval,
		logger:   logger.With().Str("module", "coingecko_pricefeed").Logger(),
	}
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
