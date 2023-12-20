package main

import (
	"context"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	cli "github.com/jawher/mow.cli"
	"github.com/xlab/closer"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/sdk-go/chain/peggy/types"
	chainclient "github.com/InjectiveLabs/sdk-go/client/chain"
	"github.com/InjectiveLabs/sdk-go/client/common"

	"github.com/InjectiveLabs/peggo/orchestrator/cosmos"
)

// txCmdSubset contains actions that can sign and send messages to Cosmos module
// as well as Ethereum transactions to Peggy contract.
//
// $ peggo tx
func txCmdSubset(cmd *cli.Cmd) {
	cmd.Command(
		"register-eth-key",
		"Submits an Ethereum key that will be used to sign messages on behalf of your Validator",
		registerEthKeyCmd,
	)
}

func registerEthKeyCmd(cmd *cli.Cmd) {
	var (
		// Cosmos params
		cosmosChainID   *string
		cosmosGRPC      *string
		tendermintRPC   *string
		cosmosGasPrices *string

		// Cosmos Key Management
		cosmosKeyringDir     *string
		cosmosKeyringAppName *string
		cosmosKeyringBackend *string

		cosmosKeyFrom       *string
		cosmosKeyPassphrase *string
		cosmosPrivKey       *string
		cosmosUseLedger     *bool

		// Ethereum Key Management
		ethKeystoreDir *string
		ethKeyFrom     *string
		ethPassphrase  *string
		ethPrivKey     *string
		ethUseLedger   *bool

		// Misc
		alwaysAutoConfirm *bool
	)

	initCosmosOptions(
		cmd,
		&cosmosChainID,
		&cosmosGRPC,
		&tendermintRPC,
		&cosmosGasPrices,
	)

	initCosmosKeyOptions(
		cmd,
		&cosmosKeyringDir,
		&cosmosKeyringAppName,
		&cosmosKeyringBackend,
		&cosmosKeyFrom,
		&cosmosKeyPassphrase,
		&cosmosPrivKey,
		&cosmosUseLedger,
	)

	initEthereumKeyOptions(
		cmd,
		&ethKeystoreDir,
		&ethKeyFrom,
		&ethPassphrase,
		&ethPrivKey,
		&ethUseLedger,
	)

	initInteractiveOptions(
		cmd,
		&alwaysAutoConfirm,
	)

	cmd.Action = func() {
		// ensure a clean exit
		defer closer.Close()

		if *ethUseLedger {
			log.Warningln("beware: you cannot really use Ledger for orchestrator, so make sure the Ethereum key is accessible outside of it")
		}

		valAddress, cosmosKeyring, err := initCosmosKeyring(
			cosmosKeyringDir,
			cosmosKeyringAppName,
			cosmosKeyringBackend,
			cosmosKeyFrom,
			cosmosKeyPassphrase,
			cosmosPrivKey,
			cosmosUseLedger,
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init Cosmos keyring")
		}

		ethKeyFromAddress, _, personalSignFn, err := initEthereumAccountsManager(
			0,
			ethKeystoreDir,
			ethKeyFrom,
			ethPassphrase,
			ethPrivKey,
			ethUseLedger,
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init Ethereum account")
		}

		log.Infoln("Using Cosmos ValAddress", valAddress.String())
		log.Infoln("Using Ethereum address", ethKeyFromAddress.String())

		actionConfirmed := *alwaysAutoConfirm || stdinConfirm("Confirm UpdatePeggyOrchestratorAddresses transaction? [y/N]: ")
		if !actionConfirmed {
			return
		}

		clientCtx, err := chainclient.NewClientContext(*cosmosChainID, valAddress.String(), cosmosKeyring)
		if err != nil {
			log.WithError(err).Fatalln("failed to initialize cosmos client context")
		}
		clientCtx = clientCtx.WithNodeURI(*tendermintRPC)

		tmRPC, err := rpchttp.New(*tendermintRPC, "/websocket")
		if err != nil {
			log.WithError(err)
		}

		clientCtx = clientCtx.WithClient(tmRPC)

		var networkName string

		switch *cosmosChainID {
		case "injective-1":
			networkName = "mainnet"
		case "injective-777":
			networkName = "devnet"
		case "injective-888":
			networkName = "testnet"
		}

		netCfg := common.LoadNetwork(networkName, "lb")

		daemonClient, err := chainclient.NewChainClient(clientCtx, netCfg, common.OptionGasPrices(*cosmosGasPrices))
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"endpoint": *cosmosGRPC,
			}).Fatalln("failed to connect to Cosmos daemon")
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
			nil,
			personalSignFn,
		)
		cancelWait()

		broadcastCtx, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelFn()

		if err = peggyBroadcaster.UpdatePeggyOrchestratorAddresses(broadcastCtx, ethKeyFromAddress, valAddress); err != nil {
			log.WithError(err).Errorln("failed to broadcast Tx")
			time.Sleep(time.Second)
			return
		}

		log.Infof("Registered Ethereum address %s for validator address %s",
			ethKeyFromAddress, valAddress.String())
	}
}

// waitForService awaits an active ClientConn to a GRPC service.
func waitForService(ctx context.Context, clientconn *grpc.ClientConn) {
	for {
		select {
		case <-ctx.Done():
			log.Fatalln("GRPC service wait timed out")
		default:
			state := clientconn.GetState()

			if state != connectivity.Ready {
				log.WithField("state", state.String()).Warningln("state of GRPC connection not ready")
				time.Sleep(5 * time.Second)
				continue
			}

			return
		}
	}
}
