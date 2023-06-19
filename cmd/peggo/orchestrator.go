package main

import (
	"context"
	"os"
	"time"

	ctypes "github.com/InjectiveLabs/sdk-go/chain/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
)

// startOrchestrator action runs an infinite loop,
// listening for events and performing hooks.
//
// $ peggo orchestrator
func orchestratorCmd(cmd *cli.Cmd) {
	// orchestrator-specific CLI options
	cfg := initConfig(cmd)

	cmd.Before = func() {
		initMetrics(cmd)
	}

	cmd.Action = func() {
		// ensure a clean exit
		defer closer.Close()

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

		log.Infoln("using Injective validator address", valAddress.String())
		log.Infoln("using Ethereum address", ethKeyFromAddress.String())

		// Connect to Injective network
		injNetwork, err := cosmos.NewNetwork(
			*cfg.cosmosChainID,
			valAddress.String(),
			*cfg.cosmosGRPC,
			*cfg.cosmosGasPrices,
			*cfg.tendermintRPC,
			cosmosKeyring,
			signerFn,
			personalSignFn,
		)
		orShutdown(err)

		// See if the provided ETH address belongs to a validator and determine in which mode peggo should run
		isValidator, err := isValidatorAddress(injNetwork.PeggyQueryClient, ethKeyFromAddress)
		if err != nil {
			log.WithError(err).Fatalln("failed to query current validator set on Injective")
		}

		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		// Construct erc20 token mapping
		peggyParams, err := injNetwork.PeggyParams(ctx)
		if err != nil {
			log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
		}

		peggyContractAddr := ethcmn.HexToAddress(peggyParams.BridgeEthereumAddress)
		injTokenAddr := ethcmn.HexToAddress(peggyParams.CosmosCoinErc20Contract)

		erc20ContractMapping := make(map[ethcmn.Address]string)
		erc20ContractMapping[injTokenAddr] = ctypes.InjectiveCoin

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

		coingeckoFeed := coingecko.NewCoingeckoPriceFeed(100, &coingecko.Config{BaseURL: *cfg.coingeckoApi})

		// make the flag obsolete and hardcode
		*cfg.minBatchFeeUSD = 49.0

		// Create peggo and run it
		peggo, err := orchestrator.NewPeggyOrchestrator(
			injNetwork,
			ethNetwork,
			coingeckoFeed,
			erc20ContractMapping,
			*cfg.minBatchFeeUSD,
			*cfg.periodicBatchRequesting,
			*cfg.relayValsets,
			*cfg.relayBatches,
			*cfg.relayValsetOffsetDur,
			*cfg.relayBatchOffsetDur,
		)
		orShutdown(err)

		go func() {
			if err := peggo.Run(ctx, isValidator); err != nil {
				log.Errorln(err)
				os.Exit(1)
			}
		}()

		closer.Hold()
	}
}

func isValidatorAddress(peggyQuery cosmos.PeggyQueryClient, addr ethcmn.Address) (bool, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelFn()

	currentValset, err := peggyQuery.CurrentValset(ctx)
	if err != nil {
		return false, err
	}

	var isValidator bool
	for _, validator := range currentValset.Members {
		if ethcmn.HexToAddress(validator.EthereumAddress) == addr {
			isValidator = true
		}
	}

	return isValidator, nil
}
