package sidechain

import (
	"github.com/ethereum/go-ethereum/common"
)

type ContractSet struct {
	PeggyContract common.Address
}

// type ContractDiscoverer interface {
// 	GetContractSet(ctx context.Context) ContractSet
// }

// func NewContractDiscoverer(ordersQuerier ordertypes.QueryClient) ContractDiscoverer {
// 	return &contractDiscoverer{
// 		queryClient: ordersQuerier,
// 		logger:      log.WithField("module", "contract_discoverer"),
// 	}
// }

// type contractDiscoverer struct {
// 	queryClient ordertypes.QueryClient
// 	logger      log.Logger
// }

// const defaultRetryDelay = 10 * time.Second

// func (c *contractDiscoverer) GetContractSet(ctx context.Context) (set ContractSet) {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		default:
// 			resp, err := c.queryClient.QueryDerivativeMarkets(ctx, &ordertypes.QueryDerivativeMarketsRequest{})
// 			if err != nil {
// 				c.logger.WithError(err).Warningln("failed to query derivative markets, retry in 10s")
// 				time.Sleep(defaultRetryDelay)
// 				continue
// 			} else if resp == nil || len(resp.Markets) == 0 {
// 				c.logger.Warningln("no derivative markets returned, retry in 10s")
// 				time.Sleep(defaultRetryDelay)
// 				continue
// 			}

// 			for _, market := range resp.Markets {
// 				set.PriceFeederContract = common.HexToAddress(market.Oracle)
// 				if set.PriceFeederContract != (common.Address{}) {
// 					return
// 				}
// 			}

// 			if set.PriceFeederContract == (common.Address{}) {
// 				c.logger.WithFields(log.Fields{
// 					"price_feeder_contract": set.PriceFeederContract.Hex(),
// 				}).Warningln("could not discover some market contracts, retry in 10s")
// 				continue
// 			}
// 		}
// 	}
// }
