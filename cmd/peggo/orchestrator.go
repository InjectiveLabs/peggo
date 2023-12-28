package main

import (
	"context"
	"os"
	"time"

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

		// todo: remove
		if *cfg.cosmosUseLedger || *cfg.ethUseLedger {
			log.Fatalln("cannot use Ledger for peggo, since signatures must be realtime")
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

		log.WithFields(log.Fields{
			"inj_addr": valAddress.String(),
			"eth_addr": ethKeyFromAddress.String(),
		}).Infoln("starting peggo service")

		var injective orchestrator.InjectiveNetwork
		if customEndpointRPCs := *cfg.cosmosGRPC != "" && *cfg.tendermintRPC != ""; customEndpointRPCs {
			injective, err = cosmos.NewCustomRPCNetwork(
				*cfg.cosmosChainID,
				valAddress.String(),
				*cfg.cosmosGRPC,
				*cfg.cosmosGasPrices,
				*cfg.tendermintRPC,
				cosmosKeyring,
				signerFn,
				personalSignFn,
			)
		} else {
			// load balanced connection
			injective, err = cosmos.NewLoadBalancedNetwork(
				*cfg.cosmosChainID,
				valAddress.String(),
				*cfg.cosmosGRPC,
				*cfg.cosmosGasPrices,
				*cfg.tendermintRPC,
				cosmosKeyring,
				signerFn,
				personalSignFn,
			)
		}

		orShutdown(err)

		// Connect to Injective network
		//injNetwork, err := cosmos.NewLoadBalancedNetwork(
		//	*cfg.cosmosChainID,
		//	valAddress.String(),
		//	*cfg.cosmosGRPC,
		//	*cfg.cosmosGasPrices,
		//	*cfg.tendermintRPC,
		//	cosmosKeyring,
		//	signerFn,
		//	personalSignFn,
		//)
		//orShutdown(err)

		// todo
		// See if the provided ETH address belongs to a validator and determine in which mode peggo should run
		//isValidator, err := isValidatorAddress(injNetwork.PeggyQueryClient, ethKeyFromAddress)
		//if err != nil {
		//	log.WithError(err).Fatalln("failed to query current validator set on Injective")
		//}

		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		// Construct erc20 token mapping
		peggyParams, err := injective.PeggyParams(ctx)
		if err != nil {
			log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
		}

		peggyContractAddr := gethcommon.HexToAddress(peggyParams.BridgeEthereumAddress)
		injTokenAddr := gethcommon.HexToAddress(peggyParams.CosmosCoinErc20Contract)

		erc20ContractMapping := make(map[gethcommon.Address]string)
		erc20ContractMapping[injTokenAddr] = chaintypes.InjectiveCoin

		// Connect to ethereum network
		ethNetwork, err := ethereum.NewNetwork(
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

		//coingeckoFeed := coingecko.NewCoingeckoPriceFeed(100, &coingecko.Config{BaseURL: *cfg.coingeckoApi})
		// LOCAL ENV TESTING //
		coingeckoFeed := coingecko.NewDummyCoingeckoFeed()

		// Create peggo and run it
		peggo, err := orchestrator.NewPeggyOrchestrator(
			injective,
			ethNetwork,
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
			if err := peggo.Run(ctx, true); err != nil { //todo
				log.Errorln(err)
				os.Exit(1)
			}
		}()

		closer.Hold()
	}
}

// todo: change check to GetDelegateKeyByEth
func isValidatorAddress(peggyQuery cosmos.PeggyQueryClient, addr gethcommon.Address) (bool, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelFn()

	currentValset, err := peggyQuery.CurrentValset(ctx)
	if err != nil {
		return false, err
	}

	var isValidator bool
	for _, validator := range currentValset.Members {
		if gethcommon.HexToAddress(validator.EthereumAddress) == addr {
			isValidator = true
		}
	}

	return isValidator, nil
}
