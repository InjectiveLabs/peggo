package main

import (
	"context"
	"os"
	"time"

	gethcommon "github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	"github.com/pkg/errors"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/peggo/orchestrator"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum"
	"github.com/InjectiveLabs/peggo/orchestrator/pricefeed"
	"github.com/InjectiveLabs/peggo/orchestrator/version"
	chaintypes "github.com/InjectiveLabs/sdk-go/chain/types"
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

		var (
			cfg              = initConfig(cmd)
			cosmosKeyringCfg = cosmos.KeyringConfig{
				KeyringDir:     *cfg.cosmosKeyringDir,
				KeyringAppName: *cfg.cosmosKeyringAppName,
				KeyringBackend: *cfg.cosmosKeyringBackend,
				KeyFrom:        *cfg.cosmosKeyFrom,
				KeyPassphrase:  *cfg.cosmosKeyPassphrase,
				PrivateKey:     *cfg.cosmosPrivKey,
				UseLedger:      *cfg.cosmosUseLedger,
			}
			cosmosNetworkCfg = cosmos.NetworkConfig{
				ChainID:       *cfg.cosmosChainID,
				CosmosGRPC:    *cfg.cosmosGRPC,
				TendermintRPC: *cfg.tendermintRPC,
				GasPrice:      *cfg.cosmosGasPrices,
			}
			ethNetworkCfg = ethereum.NetworkConfig{
				EthNodeRPC:            *cfg.ethNodeRPC,
				GasPriceAdjustment:    *cfg.ethGasPriceAdjustment,
				MaxGasPrice:           *cfg.ethMaxGasPrice,
				PendingTxWaitDuration: *cfg.pendingTxWaitDuration,
				EthNodeAlchemyWS:      *cfg.ethNodeAlchemyWS,
			}
		)

		if *cfg.cosmosUseLedger || *cfg.ethUseLedger {
			log.Fatalln("cannot use Ledger for orchestrator, since signatures must be realtime")
		}

		log.WithFields(log.Fields{
			"version":    version.AppVersion,
			"git":        version.GitCommit,
			"build_date": version.BuildDate,
			"go_version": version.GoVersion,
			"go_arch":    version.GoArch,
		}).Infoln("Peggo - Peggy module companion binary for bridging assets between Injective and Ethereum")

		// 1. Connect to Injective network

		cosmosKeyring, err := cosmos.NewKeyring(cosmosKeyringCfg)
		orShutdown(errors.Wrap(err, "failed to initialize Injective keyring"))
		log.Infoln("initialized Injective keyring", cosmosKeyring.Addr.String())

		ethKeyFromAddress, signerFn, personalSignFn, err := initEthereumAccountsManager(
			uint64(*cfg.ethChainID),
			cfg.ethKeystoreDir,
			cfg.ethKeyFrom,
			cfg.ethPassphrase,
			cfg.ethPrivKey,
			cfg.ethUseLedger,
		)
		orShutdown(errors.Wrap(err, "failed to initialize Ethereum keyring"))
		log.Infoln("initialized Ethereum keyring", ethKeyFromAddress.String())

		cosmosNetworkCfg.ValidatorAddress = cosmosKeyring.Addr.String()
		cosmosNetwork, err := cosmos.NewNetwork(cosmosKeyring, personalSignFn, cosmosNetworkCfg)
		orShutdown(err)
		log.WithFields(log.Fields{"chain_id": *cfg.cosmosChainID, "gas_price": *cfg.cosmosGasPrices}).Infoln("connected to Injective network")

		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		peggyParams, err := cosmosNetwork.PeggyParams(ctx)
		orShutdown(errors.Wrap(err, "failed to query peggy params, is injectived running?"))

		var (
			peggyContractAddr    = gethcommon.HexToAddress(peggyParams.BridgeEthereumAddress)
			injTokenAddr         = gethcommon.HexToAddress(peggyParams.CosmosCoinErc20Contract)
			erc20ContractMapping = map[gethcommon.Address]string{injTokenAddr: chaintypes.InjectiveCoin}
		)

		log.WithFields(log.Fields{"peggy_contract": peggyContractAddr.String(), "inj_token_contract": injTokenAddr.String()}).Debugln("loaded Peggy module params")

		// 2. Connect to ethereum network

		ethNetwork, err := ethereum.NewNetwork(peggyContractAddr, ethKeyFromAddress, signerFn, ethNetworkCfg)
		orShutdown(err)

		log.WithFields(log.Fields{
			"chain_id":             *cfg.ethChainID,
			"rpc":                  *cfg.ethNodeRPC,
			"max_gas_price":        *cfg.ethMaxGasPrice,
			"gas_price_adjustment": *cfg.ethGasPriceAdjustment,
		}).Infoln("connected to Ethereum network")

		addr, isValidator := cosmos.HasRegisteredOrchestrator(cosmosNetwork, ethKeyFromAddress)
		if isValidator {
			log.Debugln("provided ETH address is registered with an orchestrator", addr.String())
		}

		var (
			valsetDur time.Duration
			batchDur  time.Duration
			loopDur   time.Duration
		)

		if *cfg.relayValsets {
			valsetDur, err = time.ParseDuration(*cfg.relayValsetOffsetDur)
			orShutdown(err)
		}

		if *cfg.relayBatches {
			batchDur, err = time.ParseDuration(*cfg.relayBatchOffsetDur)
			orShutdown(err)
		}

		loopDur, err = time.ParseDuration(*cfg.loopDuration)
		if err != nil {
			loopDur = 60 * time.Second
		}

		orchestratorCfg := orchestrator.Config{
			CosmosAddr:           cosmosKeyring.Addr,
			EthereumAddr:         ethKeyFromAddress,
			MinBatchFeeUSD:       *cfg.minBatchFeeUSD,
			ERC20ContractMapping: erc20ContractMapping,
			RelayValsetOffsetDur: valsetDur,
			RelayBatchOffsetDur:  batchDur,
			RelayValsets:         *cfg.relayValsets,
			RelayBatches:         *cfg.relayBatches,
			RelayerMode:          !isValidator,
			LoopDuration:         loopDur,
		}

		// Create peggo and run it
		peggo, err := orchestrator.NewOrchestrator(
			cosmosNetwork,
			ethNetwork,
			pricefeed.NewCoingeckoPriceFeed(100, &pricefeed.Config{BaseURL: *cfg.coingeckoApi}),
			orchestratorCfg,
		)
		orShutdown(err)

		go func() {
			if err := peggo.Run(ctx, cosmosNetwork, ethNetwork); err != nil {
				log.Errorln(err)
				os.Exit(1)
			}
		}()

		closer.Hold()
	}
}
