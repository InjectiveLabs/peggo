package main

import (
	"context"
	"os"

	gethcommon "github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	chaintypes "github.com/InjectiveLabs/sdk-go/chain/types"

	"github.com/InjectiveLabs/peggo/orchestrator"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"github.com/InjectiveLabs/peggo/orchestrator/version"
)

// startOrchestrator action runs an infinite loop,
// listening for events and performing hooks.
//
// $ peggo orchestrator
func orchestratorCmd(cmd *cli.Cmd) {
	cmd.Before = func() {
		initMetrics(cmd)
	}

	cmd.Action = func() {
		// ensure a clean exit
		defer closer.Close()

		cfg := initConfig(cmd)

		log.WithFields(log.Fields{
			"version":    version.AppVersion,
			"git":        version.GitCommit,
			"build_date": version.BuildDate,
			"go_version": version.GoVersion,
			"go_arch":    version.GoArch,
		}).Infoln("peggo - peggy binary for Ethereum bridge")

		if *cfg.cosmosUseLedger || *cfg.ethUseLedger {
			log.Fatalln("cannot use Ledger for orchestrator, since signatures must be realtime")
		}

		cosmosKeyring, err := cosmos.NewKeyring(cosmos.KeyringConfig{
			KeyringDir:     *cfg.cosmosKeyringDir,
			KeyringAppName: *cfg.cosmosKeyringAppName,
			KeyringBackend: *cfg.cosmosKeyringBackend,
			KeyFrom:        *cfg.cosmosKeyFrom,
			KeyPassphrase:  *cfg.cosmosKeyPassphrase,
			PrivateKey:     *cfg.cosmosPrivKey,
			UseLedger:      *cfg.cosmosUseLedger,
		})
		orShutdown(err)

		ethKeyFromAddress, signerFn, personalSignFn, err := initEthereumAccountsManager(
			uint64(*cfg.ethChainID),
			cfg.ethKeystoreDir,
			cfg.ethKeyFrom,
			cfg.ethPassphrase,
			cfg.ethPrivKey,
			cfg.ethUseLedger,
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to initialize Ethereum account")
		}

		cosmosNetwork, err := cosmos.NewNetwork(cosmosKeyring, personalSignFn, cosmos.NetworkConfig{
			ChainID:          *cfg.cosmosChainID,
			ValidatorAddress: cosmosKeyring.Addr.String(),
			CosmosGRPC:       *cfg.cosmosGRPC,
			TendermintRPC:    *cfg.tendermintRPC,
			GasPrice:         *cfg.cosmosGasPrices,
		})
		orShutdown(err)

		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		// Construct erc20 token mapping
		peggyParams, err := cosmosNetwork.PeggyParams(ctx)
		if err != nil {
			log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
		}

		var (
			peggyContractAddr    = gethcommon.HexToAddress(peggyParams.BridgeEthereumAddress)
			injTokenAddr         = gethcommon.HexToAddress(peggyParams.CosmosCoinErc20Contract)
			erc20ContractMapping = map[gethcommon.Address]string{
				injTokenAddr: chaintypes.InjectiveCoin,
			}
		)

		// Connect to ethereum network
		ethereumNetwork, err := ethereum.NewNetwork(
			peggyContractAddr,
			ethKeyFromAddress,
			signerFn,
			ethereum.NetworkConfig{
				EthNodeRPC:            *cfg.ethNodeRPC,
				GasPriceAdjustment:    *cfg.ethGasPriceAdjustment,
				MaxGasPrice:           *cfg.ethMaxGasPrice,
				PendingTxWaitDuration: *cfg.pendingTxWaitDuration,
				EthNodeAlchemyWS:      *cfg.ethNodeAlchemyWS,
			},
		)
		orShutdown(err)

		// Create peggo and run it
		peggo, err := orchestrator.NewPeggyOrchestrator(
			cosmosKeyring.Addr,
			ethKeyFromAddress,
			coingecko.NewPriceFeed(100, &coingecko.Config{BaseURL: *cfg.coingeckoApi}),
			orchestrator.Config{
				MinBatchFeeUSD:       *cfg.minBatchFeeUSD,
				ERC20ContractMapping: erc20ContractMapping,
				RelayValsetOffsetDur: *cfg.relayValsetOffsetDur,
				RelayBatchOffsetDur:  *cfg.relayBatchOffsetDur,
				RelayValsets:         *cfg.relayValsets,
				RelayBatches:         *cfg.relayBatches,
			},
		)
		orShutdown(err)

		go func() {
			if err := peggo.Run(ctx, cosmosNetwork, ethereumNetwork); err != nil {
				log.Errorln(err)
				os.Exit(1)
			}
		}()

		closer.Hold()
	}
}
