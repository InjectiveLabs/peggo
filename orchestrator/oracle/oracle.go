package oracle

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	ummedpforacle "github.com/umee-network/umee/price-feeder/oracle"
	umeedpfprovider "github.com/umee-network/umee/price-feeder/oracle/provider"
	umeedpftypes "github.com/umee-network/umee/price-feeder/oracle/types"
	ummedpfsync "github.com/umee-network/umee/price-feeder/pkg/sync"
)

// We define tickerTimeout as the minimum timeout between each oracle loop.
const (
	tickerTimeout        = 1000 * time.Millisecond
	availablePairsReload = 24 * time.Hour
	BaseSymbolETH        = "ETH"
)

// Oracle implements the core component responsible for fetching exchange rates
// for a given set of currency pairs and determining the correct exchange rates.
type Oracle struct {
	logger zerolog.Logger
	closer *ummedpfsync.Closer

	mtx                   sync.RWMutex
	providers             map[string]*Provider // providerName => Provider
	prices                map[string]sdk.Dec   // baseSymbol => price ex.: UMEE, ETH => sdk.Dec
	subscribedBaseSymbols map[string]struct{}  // baseSymbol => nothing
}

// Provider wraps the umee provider interface.
type Provider struct {
	umeedpfprovider.Provider
	availablePairs  map[string]struct{}                  // Symbol => nothing
	subscribedPairs map[string]umeedpftypes.CurrencyPair // Symbol => currencyPair
}

func New(ctx context.Context, logger zerolog.Logger, providersName []string) (*Oracle, error) {
	providers := map[string]*Provider{}

	for _, providerName := range providersName {
		provider, err := ummedpforacle.NewProvider(ctx, providerName, logger, umeedpftypes.CurrencyPair{})
		if err != nil {
			return nil, err
		}

		providers[providerName] = &Provider{
			Provider:        provider,
			availablePairs:  map[string]struct{}{},
			subscribedPairs: map[string]umeedpftypes.CurrencyPair{},
		}
	}

	oracle := &Oracle{
		logger:                logger.With().Str("module", "oracle").Logger(),
		closer:                ummedpfsync.NewCloser(),
		providers:             providers,
		subscribedBaseSymbols: map[string]struct{}{},
	}
	oracle.loadAvailablePairs()
	go oracle.start(ctx)

	return oracle, nil
}

// GetPrices returns the price for the provided base symbols.
func (o *Oracle) GetPrices(baseSymbols ...string) (map[string]sdk.Dec, error) {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	// Creates a new array for the prices in the oracle.
	prices := make(map[string]sdk.Dec, len(baseSymbols))

	for _, baseSymbol := range baseSymbols {
		price, ok := o.prices[baseSymbol]
		if !ok {
			return nil, fmt.Errorf("error getting price for %s", baseSymbol)
		}
		prices[baseSymbol] = price
	}

	return prices, nil
}

// GetPrice returns the price based on the symbol ex.: UMEE, ETH.
func (o *Oracle) GetPrice(baseSymbol string) (sdk.Dec, error) {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	price, ok := o.prices[baseSymbol]
	if !ok {
		return sdk.Dec{}, fmt.Errorf("error getting price for %s", baseSymbol)
	}

	return price, nil
}

// SubscribeSymbols attempts to subscribe the symbols in all the providers.
// baseSymbols is the base to be subscribed ex.: ["UMEE", "ATOM"].
func (o *Oracle) SubscribeSymbols(baseSymbols ...string) error {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	for _, baseSymbol := range baseSymbols {
		_, ok := o.subscribedBaseSymbols[baseSymbol]
		if ok {
			// pair already subscribed
			continue
		}

		currencyPairs := GetStablecoinsCurrencyPair(baseSymbol)
		if err := o.subscribeProviders(currencyPairs); err != nil {
			return err
		}

		o.logger.Debug().
			Str("token_symbol", baseSymbol).
			Msg("New symbol subscribed")

		o.subscribedBaseSymbols[baseSymbol] = struct{}{}
	}

	return nil
}

func (o *Oracle) subscribeProviders(currencyPairs []umeedpftypes.CurrencyPair) error {
	for providerName, provider := range o.providers {
		var pairsToSubscribe []umeedpftypes.CurrencyPair

		for _, currencyPair := range currencyPairs {
			symbol := currencyPair.String()

			_, ok := provider.subscribedPairs[symbol]
			if ok {
				// currency pair already subscribed
				continue
			}

			_, availablePair := provider.availablePairs[symbol]
			if !availablePair {
				o.logger.Debug().Str("provider_name", providerName).Str("symbol", symbol).Msg("symbol is not available")
				continue
			}

			pairsToSubscribe = append(pairsToSubscribe, currencyPair)
		}

		if len(pairsToSubscribe) == 0 {
			o.logger.Debug().Str("provider_name", providerName).
				Msgf("No pairs to subscribe, received pairs to try: %+v", currencyPairs)
			continue
		}

		if err := provider.SubscribeCurrencyPairs(pairsToSubscribe...); err != nil {
			o.logger.Err(err).Str("provider_name", providerName).Msg("subscribing to new currency pairs")
			return err
		}

		for _, pair := range pairsToSubscribe {
			provider.subscribedPairs[pair.String()] = pair

			o.logger.Debug().Str("provider_name", providerName).
				Str("pair_symbol", pair.String()).
				Msg("Subscribed new pair")
		}

		o.logger.Info().Str("provider_name", providerName).
			Int("currency_pairs_length", len(pairsToSubscribe)).
			Msgf("Subscribed pairs %+v", pairsToSubscribe)
	}

	return nil
}

// Stop stops the oracle process and waits for it to gracefully exit.
func (o *Oracle) Stop() {
	o.closer.Close()
	<-o.closer.Done()
}

// start starts the oracle process in a blocking fashion.
func (o *Oracle) start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			o.closer.Close()

		case <-time.After(tickerTimeout):
			if err := o.tick(); err != nil {
				o.logger.Err(err).Msg("oracle tick failed")
			}

		case <-time.After(availablePairsReload):
			o.loadAvailablePairs()
		}
	}
}

// loadAvailablePairs loads all the available pairs from providers.
func (o *Oracle) loadAvailablePairs() {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	for providerName, provider := range o.providers {
		availablePairs, err := provider.GetAvailablePairs()
		if err != nil {
			o.logger.Debug().Err(err).Str("provider_name", providerName).Msg("Error getting available pairs for provider")
			continue
		}
		if len(availablePairs) == 0 {
			continue
		}
		provider.availablePairs = availablePairs
	}
}

// setPrices retrieves all the prices and candles from our set of providers as
// determined in the config. If candles are available, uses TVWAP in order
// to determine prices. If candles are not available, uses the most recent prices
// with VWAP. Warns the the user of any missing prices, and filters out any faulty
// providers which do not report prices or candles within 2𝜎 of the others.
// code originally from https://github.com/umee-network/umee/blob/2a69b56ae1c6098cb2d23ef8384f5acf28f76d35/price-feeder/oracle/oracle.go#L166-L167
func (o *Oracle) setPrices() error {
	g := new(errgroup.Group)
	mtx := new(sync.Mutex)
	providerPrices := make(umeedpfprovider.AggregatedProviderPrices)
	providerCandles := make(umeedpfprovider.AggregatedProviderCandles)

	for providerName, provider := range o.providers {
		providerName := providerName
		provider := provider
		subscribedPrices := umeedpftypes.MapPairsToSlice(provider.subscribedPairs)

		g.Go(func() error {
			var (
				tickerErr error
				candleErr error
			)

			prices, tickerErr := provider.GetTickerPrices(subscribedPrices...)
			candles, candleErr := provider.GetCandlePrices(subscribedPrices...)

			if tickerErr != nil && candleErr != nil {
				// only generates error if ticker and candle generate errors
				return fmt.Errorf("ticker error: %+v\ncandle error: %+v", tickerErr, candleErr)
			}

			// flatten and collect prices based on the base currency per provider
			//
			// e.g.: {ProviderKraken: {"ATOM": <price, volume>, ...}}
			mtx.Lock()
			for _, pair := range subscribedPrices {
				ummedpforacle.SetProviderTickerPricesAndCandles(
					providerName,
					providerPrices,
					providerCandles,
					prices,
					candles,
					pair,
				)
			}

			mtx.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		o.logger.Debug().Err(err).Msg("failed to get ticker prices from provider")
	}

	computedPrices, err := ummedpforacle.GetComputedPrices(o.logger, providerCandles, providerPrices)
	if err != nil {
		return err
	}

	o.prices = computedPrices
	return nil
}

func (o *Oracle) tick() error {
	if err := o.setPrices(); err != nil {
		return err
	}

	return nil
}

// GetStablecoinsCurrencyPair return the currency pair of that symbol quoted by some
// stablecoins.
func GetStablecoinsCurrencyPair(baseSymbol string) []umeedpftypes.CurrencyPair {
	quotes := []string{"USD", "USDT", "DAI"}
	currencyPairs := make([]umeedpftypes.CurrencyPair, len(quotes))

	for i, quote := range quotes {
		currencyPairs[i] = umeedpftypes.CurrencyPair{
			Base:  strings.ToUpper(baseSymbol),
			Quote: quote,
		}
	}

	return currencyPairs
}
