package relayer

import "github.com/umee-network/peggo/orchestrator/coingecko"

// SetPriceFeeder sets the (optional) price feeder used when performing profitable
// batch calculations. Note, this should be supplied only when the min batch
// fee is non-zero.
func SetPriceFeeder(pf *coingecko.PriceFeed) func(PeggyRelayer) {
	return func(s PeggyRelayer) { s.SetPriceFeeder(pf) }
}

func (s *peggyRelayer) SetPriceFeeder(pf *coingecko.PriceFeed) {
	s.priceFeeder = pf
}
