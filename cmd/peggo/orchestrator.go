package main

import (
	"context"
	"os"

	gethcommon "github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"github.com/InjectiveLabs/peggo/orchestrator/version"
	chaintypes "github.com/InjectiveLabs/sdk-go/chain/types"
)

// startOrchestrator action runs an infinite loop,
// listening for events and performing hooks.
//
// $ peggo orchestrator
func orchestratorCmd(cmd *cli.Cmd) {
	// orchestrator-specific CLI options

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

		valAddress, cosmosKeyring, err := initCosmosKeyring(
			cfg.cosmosKeyringDir,
			cfg.cosmosKeyringAppName,
			cfg.cosmosKeyringBackend,
			cfg.cosmosKeyFrom,
			cfg.cosmosKeyPassphrase,
			cfg.cosmosPrivKey,
			cfg.cosmosUseLedger,
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to initialize Injective keyring")
		}

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

		var (
			injectiveNet       orchestrator.InjectiveNetwork
			customEndpointRPCs = *cfg.cosmosGRPC != "" && *cfg.tendermintRPC != ""
		)

		if customEndpointRPCs {
			injectiveNet, err = cosmos.NewCustomRPCNetwork(
				*cfg.cosmosChainID,
				valAddress.String(),
				*cfg.cosmosGRPC,
				*cfg.cosmosGasPrices,
				*cfg.tendermintRPC,
				cosmosKeyring,
				personalSignFn,
			)
		} else {
			// load balanced connection
			injectiveNet, err = cosmos.NewLoadBalancedNetwork(
				*cfg.cosmosChainID,
				valAddress.String(),
				*cfg.cosmosGasPrices,
				cosmosKeyring,
				personalSignFn,
			)
		}

		orShutdown(err)

		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		// Construct erc20 token mapping
		peggyParams, err := injectiveNet.PeggyParams(ctx)
		if err != nil {
			log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
		}

		peggyContractAddr := gethcommon.HexToAddress(peggyParams.BridgeEthereumAddress)
		injTokenAddr := gethcommon.HexToAddress(peggyParams.CosmosCoinErc20Contract)

		erc20ContractMapping := make(map[gethcommon.Address]string)
		erc20ContractMapping[injTokenAddr] = chaintypes.InjectiveCoin

		// Connect to ethereum network
		ethereumNet, err := ethereum.NewNetwork(
			*cfg.ethNodeRPC,
			peggyContractAddr,
			ethKeyFromAddress,
			signerFn,
			*cfg.ethGasPriceAdjustment,
			*cfg.ethMaxGasPrice,
			*cfg.pendingTxWaitDuration,
			*cfg.ethNodeAlchemyWS,
		)
		orShutdown(err)

		coingeckoFeed := coingecko.NewCoingeckoPriceFeed(100, &coingecko.Config{BaseURL: *cfg.coingeckoApi})

		// Create peggo and run it
		peggo, err := orchestrator.NewPeggyOrchestrator(
			injectiveNet,
			ethereumNet,
			coingeckoFeed,
			erc20ContractMapping,
			*cfg.minBatchFeeUSD,
			*cfg.relayValsets,
			*cfg.relayBatches,
			*cfg.relayValsetOffsetDur,
			*cfg.relayBatchOffsetDur,
		)
		orShutdown(err)

		go func() {
			if err := peggo.Run(ctx); err != nil {
				log.Errorln(err)
				os.Exit(1)
			}
		}()

		closer.Hold()
	}
}
