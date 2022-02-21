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
	gravitytypes "github.com/umee-network/Gravity-Bridge/module/x/gravity/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/umee-network/peggo/cmd/peggo/client"
	"github.com/umee-network/peggo/orchestrator"
	"github.com/umee-network/peggo/orchestrator/coingecko"
	"github.com/umee-network/peggo/orchestrator/cosmos"
	"github.com/umee-network/peggo/orchestrator/ethereum/committer"
	gravity "github.com/umee-network/peggo/orchestrator/ethereum/gravity"
	"github.com/umee-network/peggo/orchestrator/ethereum/provider"
	"github.com/umee-network/peggo/orchestrator/relayer"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

func getOrchestratorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestrator [gravity-addr]",
		Args:  cobra.ExactArgs(1),
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

			orchAddress, cosmosKeyring, err := initCosmosKeyring(konfig)
			if err != nil {
				return fmt.Errorf("failed to initialize Cosmos keyring: %w", err)
			}

			cosmosChainID := konfig.String(flagCosmosChainID)
			clientCtx, err := client.NewClientContext(cosmosChainID, orchAddress.String(), cosmosKeyring)
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

			var feeGranter sdk.AccAddress
			if v := konfig.String(flagCosmosFeeGranter); len(v) > 0 {
				feeGranter, err = sdk.AccAddressFromBech32(v)
				if err != nil {
					return fmt.Errorf("failed to parse fee granter address: %w", err)
				}
			}

			clientCtx = clientCtx.WithClient(tmRPC).WithNodeURI(tmRPCEndpoint).WithFeeGranterAddress(feeGranter)

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

			gravityQuerier := gravitytypes.NewQueryClient(gRPCConn)

			gravityParams, err := getGravityParams(gRPCConn)
			if err != nil {
				return fmt.Errorf("failed to query for Gravity params: %w", err)
			}

			ethChainID := gravityParams.BridgeChainId
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
			ethGasLimitAdjustment := konfig.Float64(flagEthGasLimitAdjustment)
			ethCommitter, err := committer.NewEthCommitter(
				logger,
				ethKeyFromAddress,
				ethGasPriceAdjustment,
				ethGasLimitAdjustment,
				signerFn,
				ethProvider,
			)
			if err != nil && err != grpc.ErrServerStopped {
				return fmt.Errorf("failed to create Ethereum committer: %w", err)
			}

			gravityBroadcaster := cosmos.NewGravityBroadcastClient(
				logger,
				gravityQuerier,
				daemonClient,
				signerFn,
				personalSignFn,
				konfig.Int(flagCosmosMsgsPerTx),
			)

			gravityAddr := ethcmn.HexToAddress(args[0])

			ethGravity, err := wrappers.NewGravity(gravityAddr, ethCommitter.Provider())
			if err != nil {
				return fmt.Errorf("failed to create a new instance of Gravity: %w", err)
			}

			gravityContract, err := gravity.NewGravityContract(logger, ethCommitter, gravityAddr, ethGravity)
			if err != nil {
				return fmt.Errorf("failed to create Ethereum committer: %w", err)
			}

			coingeckoAPI := konfig.String(flagCoinGeckoAPI)
			coingeckoFeed := coingecko.NewCoingeckoPriceFeed(logger, 100, &coingecko.Config{
				BaseURL: coingeckoAPI,
			})

			// gravityParams.AverageBlockTime and gravityParams.AverageEthereumBlockTime are in milliseconds.
			averageCosmosBlockTime := time.Duration(gravityParams.AverageBlockTime) * time.Millisecond
			averageEthBlockTime := time.Duration(gravityParams.AverageEthereumBlockTime) * time.Millisecond

			// We multiply the relayer loop multiplier by the ETH block time.
			// gravityParams.AverageEthereumBlockTime is in milliseconds.
			ethBlockTimeF64 := float64(averageEthBlockTime.Milliseconds())
			relayerLoopMultiplier := konfig.Float64(flagRelayerLoopMultiplier)

			// Here we cast the float64 to a Duration (int64); as we are dealing with ms, we'll lose as much as 1ms.
			relayerLoopDuration := time.Duration(ethBlockTimeF64*relayerLoopMultiplier) * time.Millisecond

			relayValsets := konfig.Bool(flagRelayValsets)
			valsetRelayMode, err := validateRelayValsetsMode(konfig.String(flagValsetRelayMode))
			if err != nil {
				return err
			}

			// If relayValsets is true then the user didn't specify a value for 'valset-relay-mode',
			// so we'll default to "minimum".
			if relayValsets && valsetRelayMode == relayer.ValsetRelayModeNone {
				valsetRelayMode = relayer.ValsetRelayModeMinimum
			}

			relayer := relayer.NewGravityRelayer(
				logger,
				gravityQuerier,
				gravityContract,
				valsetRelayMode,
				konfig.Bool(flagRelayBatches),
				relayerLoopDuration,
				konfig.Duration(flagEthPendingTXWait),
				konfig.Float64(flagProfitMultiplier),
				relayer.SetPriceFeeder(coingeckoFeed),
			)

			logger = logger.With().
				Str("relayer_orchestrator_addr", orchAddress.String()).
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

			orch := orchestrator.NewGravityOrchestrator(
				logger,
				gravityQuerier,
				gravityBroadcaster,
				gravityContract,
				ethKeyFromAddress,
				signerFn,
				personalSignFn,
				relayer,
				averageCosmosBlockTime,
				averageEthBlockTime,
				batchRequesterLoopDuration,
				konfig.Int64(flagEthBlocksPerLoop),
				konfig.Int64(flagBridgeStartHeight),
			)

			ctx, cancel = context.WithCancel(context.Background())
			g, errCtx := errgroup.WithContext(ctx)

			g.Go(func() error {
				return startOrchestrator(errCtx, logger, orch)
			})

			// If we have the alchemy WS endpoint, start listening for txs against the Gravity Bridge contract.
			alchemyWS := konfig.String(flagEthAlchemyWS)
			if alchemyWS != "" {
				g.Go(func() error {
					return gravityContract.SubscribeToPendingTxs(errCtx, alchemyWS)
				})
			}

			// listen for and trap any OS signal to gracefully shutdown and exit
			trapSignal(cancel)

			return g.Wait()
		},
	}

	cmd.Flags().Bool(flagRelayValsets, false, "Relay validator set updates to Ethereum")
	cmd.Flags().String(flagValsetRelayMode, relayer.ValsetRelayModeNone.String(), "Set an (optional) relaying mode for valset updates to Ethereum. Possible values: none, minimum, all")
	cmd.Flags().Bool(flagRelayBatches, false, "Relay transaction batches to Ethereum")
	cmd.Flags().Int64(flagEthBlocksPerLoop, 2000, "Number of Ethereum blocks to process per orchestrator loop")
	cmd.Flags().String(flagCoinGeckoAPI, "https://api.coingecko.com/api/v3", "Specify the coingecko API endpoint")
	cmd.Flags().Duration(flagEthPendingTXWait, 20*time.Minute, "Time for a pending tx to be considered stale")
	cmd.Flags().String(flagEthAlchemyWS, "", "Specify the Alchemy websocket endpoint")
	cmd.Flags().Float64(flagProfitMultiplier, 1.0, "Multiplier to apply to relayer profit")
	cmd.Flags().Float64(flagRelayerLoopMultiplier, 3.0, "Multiplier for the relayer loop duration (in ETH blocks)")
	cmd.Flags().Float64(flagRequesterLoopMultiplier, 60.0, "Multiplier for the batch requester loop duration (in Cosmos blocks)")
	cmd.Flags().String(flagCosmosFeeGranter, "", "Set an (optional) fee granter address that will pay for Cosmos fees (feegrant must exist)")
	cmd.Flags().Int64(flagBridgeStartHeight, 0, "Set an (optional) height to wait for the bridge to be available")
	cmd.Flags().Int(flagCosmosMsgsPerTx, 10, "Set a maximum number of messages to send per transaction (used for claims)")
	cmd.Flags().AddFlagSet(cosmosFlagSet())
	cmd.Flags().AddFlagSet(cosmosKeyringFlagSet())
	cmd.Flags().AddFlagSet(ethereumKeyOptsFlagSet())
	cmd.Flags().AddFlagSet(ethereumOptsFlagSet())
	_ = cmd.Flags().MarkDeprecated(flagRelayValsets, "use --valset-relay-mode instead")

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

func startOrchestrator(ctx context.Context, logger zerolog.Logger, orch orchestrator.GravityOrchestrator) error {
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

func validateRelayValsetsMode(mode string) (relayer.ValsetRelayMode, error) {
	switch mode {
	case relayer.ValsetRelayModeNone.String():
		return relayer.ValsetRelayModeNone, nil
	case relayer.ValsetRelayModeMinimum.String():
		return relayer.ValsetRelayModeMinimum, nil
	case relayer.ValsetRelayModeAll.String():
		return relayer.ValsetRelayModeAll, nil
	default:
		return relayer.ValsetRelayModeNone, fmt.Errorf("invalid relay valsets mode: %s", mode)
	}
}
