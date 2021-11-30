// nolint: lll
package peggo

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/umee-network/peggo/cmd/peggo/client"
	"github.com/umee-network/peggo/orchestrator"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/cosmos"
	"github.com/umee-network/peggo/orchestrator/cosmos/tmclient"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	"github.com/umee-network/peggo/orchestrator/ethereum/peggy"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"
	"github.com/umee-network/peggo/orchestrator/relayer"
	peggytypes "github.com/umee-network/umee/x/peggy/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

func getOrchestratorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestrator",
		Short: "Starts the orchestrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			konfig, err := parseServerConfig(cmd)
			if err != nil {
				return err
			}

			logger, err := getLogger(cmd)
			if err != nil {
				return err
			}

			cosmosUseLedger := konfig.Bool(flagCosmosUseLedger)
			ethUseLedger := konfig.Bool(flagEthUseLedger)
			if cosmosUseLedger || ethUseLedger {
				return fmt.Errorf("cannot use Ledger for orchestrator")
			}

			valAddress, cosmosKeyring, err := initCosmosKeyring(konfig)
			if err != nil {
				return fmt.Errorf("failed to initialize Cosmos keyring: %w", err)
			}

			cosmosChainID := konfig.String(flagCosmosChainID)
			clientCtx, err := client.NewClientContext(cosmosChainID, valAddress.String(), cosmosKeyring)
			if err != nil {
				return err
			}

			tmRPCEndpoint := konfig.String(flagTendermintRPC)
			cosmosGRPC := konfig.String(flagCosmosGRPC)
			cosmosGasPrices := konfig.String(flagCosmosGasPrices)

			tmRPC, err := rpchttp.New(tmRPCEndpoint, "/websocket")
			if err != nil {
				return fmt.Errorf("failed to create Tendermint RPC client: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Connected to Tendermint RPC: %s\n", tmRPCEndpoint)
			clientCtx = clientCtx.WithClient(tmRPC).WithNodeURI(tmRPCEndpoint)

			daemonClient, err := client.NewCosmosClient(clientCtx, logger, cosmosGRPC, client.OptionGasPrices(cosmosGasPrices))
			if err != nil {
				return err
			}

			// TODO: Clean this up to be more ergonomic and clean. We can probably
			// encapsulate all of this into a single utility function that gracefully
			// checks for the gRPC status/health.
			//
			// Ref: https://github.com/umee-network/peggo/issues/2
			fmt.Fprintln(os.Stderr, "Waiting for cosmos gRPC service...")
			time.Sleep(time.Second)

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			gRPCConn := daemonClient.QueryClient()
			waitForService(ctx, gRPCConn)

			peggyQuerier := peggytypes.NewQueryClient(gRPCConn)

			// query peggy params
			peggyQueryClient := cosmos.NewPeggyQueryClient(peggyQuerier)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			peggyParams, err := peggyQueryClient.PeggyParams(ctx)
			if err != nil {
				return fmt.Errorf("failed to query for Peggy params: %w", err)
			}

			ethChainID := peggyParams.BridgeChainId
			ethKeyFromAddress, signerFn, personalSignFn, err := initEthereumAccountsManager(logger, ethChainID, konfig)
			if err != nil {
				return fmt.Errorf("failed to initialize Ethereum account: %w", err)
			}

			ethRPCEndpoint := konfig.String(flagEthRPC)
			ethRPC, err := ethrpc.Dial(ethRPCEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial Ethereum RPC node: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Connected to Ethereum RPC: %s\n", ethRPCEndpoint)
			ethProvider := provider.NewEVMProvider(ethRPC)

			ethGasPriceAdjustment := konfig.Float64(flagEthGasAdjustment)
			ethCommitter, err := committer.NewEthCommitter(
				logger,
				ethKeyFromAddress,
				ethGasPriceAdjustment,
				signerFn,
				ethProvider,
			)
			if err != nil && err != grpc.ErrServerStopped {
				return fmt.Errorf("failed to create Ethereum committer: %w", err)
			}

			peggyBroadcaster := cosmos.NewPeggyBroadcastClient(
				logger,
				peggyQuerier,
				daemonClient,
				signerFn,
				personalSignFn,
			)

			peggyAddress := ethcmn.HexToAddress(peggyParams.BridgeEthereumAddress)
			peggyContract, err := peggy.NewPeggyContract(logger, ethCommitter, peggyAddress)
			if err != nil {
				return fmt.Errorf("failed to create Ethereum committer: %w", err)
			}

			coingeckoAPI := konfig.String(flagCoinGeckoAPI)
			coingeckoFeed := coingecko.NewCoingeckoPriceFeed(logger, 100, &coingecko.Config{
				BaseURL: coingeckoAPI,
			})

			// peggyParams.AverageBlockTime and peggyParams.AverageEthereumBlockTime are in milliseconds.
			averageCosmosBlockTime := time.Duration(peggyParams.AverageBlockTime) * time.Millisecond
			averageEthBlockTime := time.Duration(peggyParams.AverageEthereumBlockTime) * time.Millisecond

			// We multiply the relayer loop multiplier by the ETH block time.
			// peggyParams.AverageEthereumBlockTime is in milliseconds.
			ethBlockTimeF64 := float64(averageEthBlockTime.Milliseconds())
			relayerLoopMultiplier := konfig.Float64(flagRelayerLoopMultiplier)

			// Here we cast the float64 to a Duration (int64); as we are dealing with ms, we'll lose as much as 1ms.
			relayerLoopDuration := time.Duration(ethBlockTimeF64*relayerLoopMultiplier) * time.Millisecond

			relayer := relayer.NewPeggyRelayer(
				logger,
				peggyQueryClient,
				peggyContract,
				tmclient.NewRPCClient(logger, tmRPCEndpoint),
				konfig.Bool(flagRelayValsets),
				konfig.Bool(flagRelayBatches),
				relayerLoopDuration,
				konfig.Duration(flagEthPendingTXWait),
				konfig.Float64(flagProfitMultiplier),
				relayer.SetPriceFeeder(coingeckoFeed),
			)

			logger = logger.With().
				Str("relayer_validator_addr", sdk.ValAddress(valAddress).String()).
				Str("relayer_ethereum_addr", ethKeyFromAddress.String()).
				Logger()

			// Run the requester loop every approximately 60 Cosmos blocks (around 5m by default) to allow time to
			// receive new transactions. Running this faster will cause a lot of small batches and lots of messages
			// going around the network. We need to keep in mind that this call is going to be made by all the
			// validators. This loop is configurable so it can be adjusted for E2E tests.

			cosmosBlockTimeF64 := float64(averageCosmosBlockTime.Milliseconds())
			requesterLoopMultiplier := konfig.Float64(flagRequesterLoopMultiplier)

			// Here we cast the float64 to a Duration (int64); as we are dealing with ms, we'll lose as much as 1ms.
			batchRequesterLoopDuration := time.Duration(cosmosBlockTimeF64*requesterLoopMultiplier) * time.Millisecond

			orch := orchestrator.NewPeggyOrchestrator(
				logger,
				peggyQueryClient,
				peggyBroadcaster,
				tmclient.NewRPCClient(logger, tmRPCEndpoint),
				peggyContract,
				ethKeyFromAddress,
				signerFn,
				personalSignFn,
				relayer,
				averageCosmosBlockTime,
				averageEthBlockTime,
				batchRequesterLoopDuration,
				konfig.Int64(flagEthBlocksPerLoop),
			)

			ctx, cancel = context.WithCancel(context.Background())
			g, errCtx := errgroup.WithContext(ctx)

			g.Go(func() error {
				return startOrchestrator(errCtx, logger, orch)
			})

			// If we have the alchemy WS endpoint, start listening for txs against the Peggy contract.
			alchemyWS := konfig.String(flagEthAlchemyWS)
			if alchemyWS != "" {
				g.Go(func() error {
					return peggyContract.SubscribeToPendingTxs(errCtx, alchemyWS)
				})
			}

			// listen for and trap any OS signal to gracefully shutdown and exit
			trapSignal(cancel)

			return g.Wait()
		},
	}

	cmd.Flags().Bool(flagRelayValsets, false, "Relay validator set updates to Ethereum")
	cmd.Flags().Bool(flagRelayBatches, false, "Relay transaction batches to Ethereum")
	cmd.Flags().Int64(flagEthBlocksPerLoop, 40, "Number of Ethereum blocks to process per orchestrator loop")
	cmd.Flags().String(flagCoinGeckoAPI, "https://api.coingecko.com/api/v3", "Specify the coingecko API endpoint")
	cmd.Flags().Duration(flagEthPendingTXWait, 20*time.Minute, "Time for a pending tx to be considered stale")
	cmd.Flags().String(flagEthAlchemyWS, "", "Specify the Alchemy websocket endpoint")
	cmd.Flags().Float64(flagProfitMultiplier, 1.0, "Multiplier to apply to relayer profit")
	cmd.Flags().Float64(flagRelayerLoopMultiplier, 3.0, "Multiplier for the relayer loop duration (in ETH blocks)")
	cmd.Flags().Float64(flagRequesterLoopMultiplier, 60.0, "Multiplier for the batch requester loop duration (in Cosmos blocks)")
	cmd.Flags().AddFlagSet(cosmosFlagSet())
	cmd.Flags().AddFlagSet(cosmosKeyringFlagSet())
	cmd.Flags().AddFlagSet(ethereumKeyOptsFlagSet())
	cmd.Flags().AddFlagSet(ethereumOptsFlagSet())

	return cmd
}

func trapSignal(cancel context.CancelFunc) {
	var sigCh = make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGTERM)
	signal.Notify(sigCh, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "Caught signal (%s); shutting down...\n", sig)
		cancel()
	}()
}

func startOrchestrator(ctx context.Context, logger zerolog.Logger, orch orchestrator.PeggyOrchestrator) error {
	srvErrCh := make(chan error, 1)
	go func() {
		logger.Info().Msg("starting orchestrator...")
		srvErrCh <- orch.Start(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-srvErrCh:
			logger.Error().Err(err).Msg("failed to start orchestrator")
			return err
		}
	}
}
