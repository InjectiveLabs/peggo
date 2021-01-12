package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	cli "github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/InjectiveLabs/peggo/orchestrator/sidechain"
	// 	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/InjectiveLabs/peggo/orchestrator/metrics"
	"github.com/InjectiveLabs/peggo/orchestrator/version"
)

var app = cli.App("peggo-orchestrator", "An Orchestrator service from Peggy (Peggo) bridge rewrite.")

func main() {
	readEnv()
	initFlags()

	app.Before = prepareApp
	app.Action = runApp

	app.Command("version", "Print the version information and exit.", versionCmd)

	_ = app.Run(os.Args)
}

func versionCmd(c *cli.Cmd) {
	c.Action = func() {
		fmt.Println(version.Version())
	}
}

func readEnv() {
	if envdata, _ := ioutil.ReadFile(".env"); len(envdata) > 0 {
		s := bufio.NewScanner(bytes.NewReader(envdata))
		for s.Scan() {
			parts := strings.Split(s.Text(), "=")
			if len(parts) != 2 {
				continue
			}

			// log.Infoln("Setting", parts[0], "to", parts[1])
			if err := os.Setenv(parts[0], parts[1]); err != nil {
				log.WithField("name", parts[0]).WithError(err).Warningln("failed to override ENV variable")
			}
		}
	}
}

func prepareApp() {
	log.DefaultLogger.SetLevel(Level(*appLogLevel))
}

func runApp() {
	defer closer.Close()

	if toBool(*statsdDisabled) {
		// initializes statsd client with a mock one with no-op enabled
		metrics.Disable()
	} else {
		go func() {
			for {
				hostname, _ := os.Hostname()
				err := metrics.Init(*statsdAddr, checkStatsdPrefix(*statsdPrefix), &metrics.StatterConfig{
					EnvName:              *envName,
					HostName:             hostname,
					StuckFunctionTimeout: duration(*statsdStuckDur, 30*time.Minute),
					MockingEnabled:       toBool(*statsdMocking) || *envName == "local",
				})
				if err != nil {
					log.WithError(err).Warningln("metrics init failed, will retry in 1 min")
					time.Sleep(time.Minute)
					continue
				}
				break
			}
			closer.Bind(func() {
				metrics.Close()
			})
		}()
	}

	var err error
	var ethSignerPk *ecdsa.PrivateKey
	if len(*priceFeederFromPK) > 0 {
		ethSignerPk, err = ethcrypto.HexToECDSA(*priceFeederFromPK)
		orShutdown(err)

		fromAddr := ethcrypto.PubkeyToAddress(ethSignerPk.PublicKey)
		log.WithField("from", fromAddr.Hex()).Info("Loaded sender private key")
	} else {
		log.Fatalln("No EVM account credentials provided")
	}

	evmRPC, err := rpc.Dial(*evmNodeHTTP)
	if err != nil {
		log.WithField("endpoint", *evmNodeHTTP).WithError(err).Fatalln("Failed to connect to EVM RPC")
		return
	}
	ethProvider := provider.NewEVMProvider(evmRPC)
	log.Infoln("Connected to EVM RPC at", *evmNodeHTTP)

	log.Infoln("Waiting for injectived RPC")
	time.Sleep(1 * time.Second)
	daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), duration(*svcWaitTimeout, time.Minute))

	conn, err := grpc.Dial(*injectiveProtoAddr, grpc.WithInsecure(), grpc.WithContextDialer(client.DialerFunc))
	orShutdown(err)

	time.Sleep(1 * time.Second)

	waitForService(daemonWaitCtx, conn)
	// //ordersQuerier := ordertypes.NewQueryClient(conn)
	// contractSetDiscoverer := client.NewContractDiscoverer(nil) //ordersQuerier)
	// contractSet := contractSetDiscoverer.GetContractSet(daemonWaitCtx)
	cancelWait()

	_ = ethProvider
	// ethCommitter, err := committer.NewEthCommitter(
	// 	ethSignerPk,
	// 	ethProvider,
	// 	contractSet,
	// )
	// orShutdown(err)

	// markets := listDerivativeMarkets(context.Background(), ordersQuerier)
	// if len(markets) == 0 {
	// 	log.Fatalln("No derivative markets info loaded")
	// }
	// log.WithField("markets_len", len(markets)).Infoln("Loaded derivative markets info")

	// // priceOracle := NewPriceOracle(markets, duration(*fetchIntervalSeconds, 60*time.Second), ethCommitter)

	// go priceOracle.Run()
	// closer.Bind(func() {
	// 	priceOracle.Shutdown()
	// })

	closer.Hold()
}

func waitForService(ctx context.Context, conn *grpc.ClientConn) {
	for {
		select {
		case <-ctx.Done():
			log.Fatalln("service wait timed out")
		default:
			state := conn.GetState()

			if state != connectivity.Ready {
				log.WithField("state", state.String()).Warningln("state of grpc connection not ready")
				time.Sleep(5 * time.Second)
				continue
			}

			return
		}
	}
}

// func listDerivativeMarkets(ctx context.Context, ordersQuerier ordertypes.QueryClient) (markets map[common.Hash]ordertypes.DerivativeMarket) {
// 	resp, err := ordersQuerier.QueryDerivativeMarkets(ctx, &ordertypes.QueryDerivativeMarketsRequest{})
// 	if err != nil {
// 		log.WithError(err).Errorln("failed to list derivative markets")
// 		return nil
// 	}

// 	markets = make(map[common.Hash]ordertypes.DerivativeMarket, len(resp.Markets))
// 	for _, market := range resp.Markets {
// 		markets[common.HexToHash(market.MarketId)] = *market
// 	}

// 	return markets
// }

func orShutdown(err error) {
	if err != nil && err != grpc.ErrServerStopped {
		log.WithError(err).Fatalln("unable to start index-price-oracle")
	}
}
