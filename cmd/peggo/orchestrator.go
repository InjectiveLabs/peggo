package main

import (
	"context"
	"os"
	"time"

	ethcmn "github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	"github.com/InjectiveLabs/sdk-go/client/common"

	"github.com/InjectiveLabs/peggo/orchestrator"
	"github.com/InjectiveLabs/peggo/orchestrator/coingecko"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
	"github.com/InjectiveLabs/peggo/orchestrator/cosmos/tmclient"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/committer"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/peggy"
	"github.com/InjectiveLabs/peggo/orchestrator/ethereum/provider"
	"github.com/InjectiveLabs/peggo/orchestrator/relayer"

	ctypes "github.com/InjectiveLabs/sdk-go/chain/types"
	"github.com/ethereum/go-ethereum/rpc"
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
			log.Fatalln("cannot really use Ledger for orchestrator, since signatures msut be realtime")
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
			log.WithError(err).Fatalln("failed to init Cosmos keyring")
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
			log.WithError(err).Fatalln("failed to init Ethereum account")
		}

		log.Infoln("Using Cosmos ValAddress", valAddress.String())
		log.Infoln("Using Ethereum address", ethKeyFromAddress.String())

		clientCtx, err := chainclient.NewClientContext(*cfg.cosmosChainID, valAddress.String(), cosmosKeyring)
		if err != nil {
			log.WithError(err).Fatalln("failed to initialize cosmos client context")
		}
		clientCtx = clientCtx.WithNodeURI(*cfg.tendermintRPC)
		tmRPC, err := rpchttp.New(*cfg.tendermintRPC, "/websocket")
		if err != nil {
			log.WithError(err)
		}
		clientCtx = clientCtx.WithClient(tmRPC)

		daemonClient, err := chainclient.NewChainClient(clientCtx, *cfg.cosmosGRPC, common.OptionGasPrices(*cfg.cosmosGasPrices))
		if err != nil {
			log.WithError(err).WithFields(
				log.Fields{"endpoint": *cfg.cosmosGRPC}).
				Fatalln("failed to connect to daemon, is injectived running?")
		}

		log.Infoln("Waiting for injectived GRPC")
		time.Sleep(1 * time.Second)

		daemonWaitCtx, cancelWait := context.WithTimeout(context.Background(), time.Minute)
		grpcConn := daemonClient.QueryClient()
		waitForService(daemonWaitCtx, grpcConn)
		peggyQuerier := types.NewQueryClient(grpcConn)
		peggyBroadcaster := cosmos.NewPeggyBroadcastClient(
			peggyQuerier,
			daemonClient,
			signerFn,
			personalSignFn,
		)
		cancelWait()

		// Query peggy params
		cosmosQueryClient := cosmos.NewPeggyQueryClient(peggyQuerier)
		ctx, cancelFn := context.WithCancel(context.Background())
		closer.Bind(cancelFn)

		peggyParams, err := cosmosQueryClient.PeggyParams(ctx)
		if err != nil {
			log.WithError(err).Fatalln("failed to query peggy params, is injectived running?")
		}

		peggyAddress := ethcmn.HexToAddress(peggyParams.BridgeEthereumAddress)
		injAddress := ethcmn.HexToAddress(peggyParams.CosmosCoinErc20Contract)

		// Check if the provided ETH address belongs to a validator
		isValidator, err := isValidatorAddress(cosmosQueryClient, ethKeyFromAddress)
		if err != nil {
			log.WithError(err).Fatalln("failed to query the current validator set from injective")

			return
		}

		erc20ContractMapping := make(map[ethcmn.Address]string)
		erc20ContractMapping[injAddress] = ctypes.InjectiveCoin

		evmRPC, err := rpc.Dial(*cfg.ethNodeRPC)
		if err != nil {
			log.WithField("endpoint", *cfg.ethNodeRPC).WithError(err).Fatalln("Failed to connect to Ethereum RPC")
			return
		}
		ethProvider := provider.NewEVMProvider(evmRPC)
		log.Infoln("Connected to Ethereum RPC at", *cfg.ethNodeRPC)

		ethCommitter, err := committer.NewEthCommitter(ethKeyFromAddress, *cfg.ethGasPriceAdjustment, *cfg.ethMaxGasPrice, signerFn, ethProvider)
		orShutdown(err)

		pendingTxInputList := peggy.PendingTxInputList{}

		pendingTxWaitDuration, err := time.ParseDuration(*cfg.pendingTxWaitDuration)
		orShutdown(err)

		peggyContract, err := peggy.NewPeggyContract(ethCommitter, peggyAddress, pendingTxInputList, pendingTxWaitDuration)
		orShutdown(err)

		// If Alchemy Websocket URL is set, then Subscribe to Pending Transaction of Peggy Contract.
		if *cfg.ethNodeAlchemyWS != "" {
			go peggyContract.SubscribeToPendingTxs(*cfg.ethNodeAlchemyWS)
		}

		relayer := relayer.NewPeggyRelayer(
			cosmosQueryClient,
			tmclient.NewRPCClient(*cfg.tendermintRPC),
			peggyContract,
			*cfg.relayValsets,
			*cfg.relayValsetOffsetDur,
			*cfg.relayBatches,
			*cfg.relayBatchOffsetDur,
		)

		coingeckoFeed := coingecko.NewCoingeckoPriceFeed(100, &coingecko.Config{BaseURL: *cfg.coingeckoApi})

		// make the flag obsolete and hardcode
		*cfg.minBatchFeeUSD = 49.0

		svc := orchestrator.NewPeggyOrchestrator(
			cosmosQueryClient,
			peggyBroadcaster,
			tmclient.NewRPCClient(*cfg.tendermintRPC),
			peggyContract,
			ethKeyFromAddress,
			signerFn,
			personalSignFn,
			erc20ContractMapping,
			relayer,
			*cfg.minBatchFeeUSD,
			coingeckoFeed,
			*cfg.periodicBatchRequesting,
		)

		go func() {
			if err := svc.Start(ctx, isValidator); err != nil {
				log.Errorln(err)

				// signal there that the app failed
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
