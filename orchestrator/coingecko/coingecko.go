package coingecko

import (
	"encoding/json"
	"io"
	"io/ioutil"

	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/InjectiveLabs/metrics"
	cosmtypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	log "github.com/xlab/suplog"
)

const (
	maxRespTime        = 15 * time.Second
	maxRespHeadersTime = 15 * time.Second
	maxRespBytes       = 10 * 1024 * 1024
)

var zeroPrice = float64(0)

type PriceFeed struct {
	client *http.Client
	config *Config

	interval time.Duration

	logger  log.Logger
	svcTags metrics.Tags
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

func (cp *PriceFeed) QueryUSDPrice(erc20Contract common.Address) (float64, error) {
	metrics.ReportFuncCall(cp.svcTags)
	doneFn := metrics.ReportFuncTiming(cp.svcTags)
	defer doneFn()

	u, err := url.ParseRequestURI(urlJoin(cp.config.BaseURL, "simple", "token_price", "ethereum"))
	if err != nil {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Fatalln("failed to parse URL")
	}

	q := make(url.Values)

	q.Set("contract_addresses", strings.ToLower(erc20Contract.String()))
	q.Set("vs_currencies", "usd")
	u.RawQuery = q.Encode()

	reqURL := u.String()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Fatalln("failed to create HTTP request")
	}

	resp, err := cp.client.Do(req)
	if err != nil {
		metrics.ReportFuncError(cp.svcTags)
		err = errors.Wrapf(err, "failed to fetch price from %s", reqURL)
		return zeroPrice, err
	}

	respBody, err := ioutil.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		_ = resp.Body.Close()
		metrics.ReportFuncError(cp.svcTags)
		err = errors.Wrapf(err, "failed to read response body from %s", reqURL)
		return zeroPrice, err
	}
	_ = resp.Body.Close()

	var f interface{}
	if err := json.Unmarshal(respBody, &f); err != nil {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Errorln("failed to unmarshal response")
		return zeroPrice, err
	}

	m, ok := f.(map[string]interface{})
	if !ok {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Errorln("failed to cast response type: map[string]interface{}")
		return zeroPrice, err
	}

	v := m[strings.ToLower(erc20Contract.String())]
	if v == nil {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Errorln("failed to get contract address")
		return zeroPrice, err
	}

	n, ok := v.(map[string]interface{})
	if !ok {
		metrics.ReportFuncError(cp.svcTags)
		cp.logger.WithError(err).Errorln("failed to cast value type: map[string]interface{}")
		return zeroPrice, err
	}

	tokenPriceInUSD := n["usd"].(float64)
	return tokenPriceInUSD, nil
}

// NewPriceFeed returns price puller for given symbol. The price will be pulled
// from endpoint and divided by scaleFactor. Symbol name (if reported by endpoint) must match.
func NewPriceFeed(interval time.Duration, endpointConfig *Config) *PriceFeed {
	return &PriceFeed{
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: maxRespHeadersTime,
			},
			Timeout: maxRespTime,
		},
		config: checkCoingeckoConfig(endpointConfig),

		interval: interval,

		logger: log.WithFields(log.Fields{
			"svc":      "oracle",
			"provider": "coingeckgo",
		}),
		svcTags: metrics.Tags{
			"provider": string("coingeckgo"),
		},
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

func (cp *PriceFeed) CheckFeeThreshold(erc20Contract common.Address, totalFee cosmtypes.Int, minFeeInUSD float64) bool {
	metrics.ReportFuncCall(cp.svcTags)
	doneFn := metrics.ReportFuncTiming(cp.svcTags)
	defer doneFn()

	tokenPriceInUSD, err := cp.QueryUSDPrice(erc20Contract)
	if err != nil {
		metrics.ReportFuncError(cp.svcTags)
		return false
	}

	tokenPriceInUSDDec := decimal.NewFromFloat(tokenPriceInUSD)
	totalFeeInUSDDec := decimal.NewFromBigInt(totalFee.BigInt(), -18).Mul(tokenPriceInUSDDec)
	minFeeInUSDDec := decimal.NewFromFloat(minFeeInUSD)

	if totalFeeInUSDDec.GreaterThan(minFeeInUSDDec) {
		return true
	}
	return false
}
