package peggo

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"time"

	gravitytypes "github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcmn "github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/knadh/koanf"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"google.golang.org/grpc"

	"github.com/umee-network/peggo/cmd/peggo/client"
	"github.com/umee-network/peggo/orchestrator/relayer"
	wrappers "github.com/umee-network/peggo/solwrappers/Gravity.sol"
)

var (
	//nolint: lll
	maxUint256     = new(big.Int).SetBytes(ethcmn.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"))
	halfMaxUint256 = new(big.Int).Div(maxUint256, big.NewInt(2))
)

func getBridgeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Commands to interface with Gravity Bridge Ethereum contract",
	}

	cmd.PersistentFlags().AddFlagSet(cosmosFlagSet())
	cmd.PersistentFlags().AddFlagSet(bridgeFlagSet())

	cmd.AddCommand(
		deployGravityCmd(),
		deployERC20Cmd(),
		deployERC20RawCmd(),
		sendToCosmosCmd(),
	)

	return cmd
}

// TODO: Support --admin capabilities.
func deployGravityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-gravity",
		Short: "Deploy the Gravity Bridge smart contract on Ethereum",
		RunE: func(cmd *cobra.Command, args []string) error {
			konfig, err := parseServerConfig(cmd)
			if err != nil {
				return err
			}

			// COSMOS RPC
			clientCtx, err := client.NewClientContext(konfig.String(flagCosmosChainID), "", nil)
			if err != nil {
				return err
			}

			logger, err := getLogger(cmd)
			if err != nil {
				return err
			}

			tmRPCEndpoint, err := parseURL(logger, konfig, flagTendermintRPC)
			if err != nil {
				return err
			}
			cosmosGRPC, err := parseURL(logger, konfig, flagCosmosGRPC)
			if err != nil {
				return err
			}

			tmRPC, err := rpchttp.New(tmRPCEndpoint, "/websocket")
			if err != nil {
				return fmt.Errorf("failed to create Tendermint RPC client: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Connected to Tendermint RPC: %s\n", tmRPCEndpoint)
			clientCtx = clientCtx.WithClient(tmRPC).WithNodeURI(tmRPCEndpoint)

			daemonClient, err := client.NewCosmosClient(clientCtx, logger, cosmosGRPC)
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

			gravityParams, err := getGravityParams(gRPCConn)
			if err != nil {
				return err
			}

			// ETH RPC
			ethRPCEndpoint := konfig.String(flagEthRPC)
			ethRPC, err := ethclient.Dial(ethRPCEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial Ethereum RPC node: %w", err)
			}

			auth, err := buildTransactOpts(konfig, ethRPC)
			if err != nil {
				return err
			}

			gravityQueryClient := gravitytypes.NewQueryClient(gRPCConn)
			currValset, err := gravityQueryClient.CurrentValset(cmd.Context(), &gravitytypes.QueryCurrentValsetRequest{})
			if err != nil {
				return err
			}

			if currValset == nil {
				return errors.New("no validator set found")
			}

			var (
				validators = make([]ethcmn.Address, len(currValset.Valset.Members))
				powers     = make([]*big.Int, len(currValset.Valset.Members))

				totalPower uint64
			)

			// Always sort the validator set
			relayer.BridgeValidators(currValset.Valset.Members).Sort()

			for i, member := range currValset.Valset.Members {
				validators[i] = ethcmn.HexToAddress(member.EthereumAddress)
				powers[i] = new(big.Int).SetUint64(member.Power)
				totalPower += member.Power
			}

			powerThreshold := big.NewInt(2834678415)

			if totalPower < powerThreshold.Uint64() {
				return fmt.Errorf(
					"refusing to deploy; total power (%d) < power threshold (%d)",
					totalPower, powerThreshold.Uint64(),
				)
			}

			gravityIDBytes := []uint8(gravityParams.GravityId)
			var gravityIDBytes32 [32]uint8
			copy(gravityIDBytes32[:], gravityIDBytes)

			address, tx, _, err := wrappers.DeployGravity(auth, ethRPC, gravityIDBytes32, validators, powers)
			if err != nil {
				return fmt.Errorf("failed deploy Gravity Bridge contract: %w", err)
			}

			powerStr := ""
			for _, power := range powers {
				powerStr += power.String() + " ,"
			}

			_, _ = fmt.Fprintf(os.Stderr, `Gravity Bridge contract successfully deployed!
Address: %s
Input: %+v, %+v, [%s]
Transaction: %s
`,
				address.Hex(),
				gravityIDBytes32, validators, powerStr,
				tx.Hash().Hex(),
			)

			return nil
		},
	}

	return cmd
}

func deployERC20Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy-erc20 [gravity-addr] [denom-base]",
		Args:  cobra.ExactArgs(2),
		Short: "Deploy a Cosmos native asset on Ethereum as an ERC20 token",
		RunE: func(cmd *cobra.Command, args []string) error {
			konfig, err := parseServerConfig(cmd)
			if err != nil {
				return err
			}

			ethRPCEndpoint := konfig.String(flagEthRPC)
			ethRPC, err := ethclient.Dial(ethRPCEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial Ethereum RPC node: %w", err)
			}

			auth, err := buildTransactOpts(konfig, ethRPC)
			if err != nil {
				return err
			}

			// query for the name and symbol on-chain via the token's metadata
			clientCtx, err := client.NewClientContext(konfig.String(flagCosmosChainID), "", nil)
			if err != nil {
				return err
			}

			logger, err := getLogger(cmd)
			if err != nil {
				return err
			}

			tmRPCEndpoint, err := parseURL(logger, konfig, flagTendermintRPC)
			if err != nil {
				return err
			}
			cosmosGRPC, err := parseURL(logger, konfig, flagCosmosGRPC)
			if err != nil {
				return err
			}

			tmRPC, err := rpchttp.New(tmRPCEndpoint, "/websocket")
			if err != nil {
				return fmt.Errorf("failed to create Tendermint RPC client: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Connected to Tendermint RPC: %s\n", tmRPCEndpoint)
			clientCtx = clientCtx.WithClient(tmRPC).WithNodeURI(tmRPCEndpoint)

			daemonClient, err := client.NewCosmosClient(clientCtx, logger, cosmosGRPC)
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

			if !ethcmn.IsHexAddress(args[0]) {
				return fmt.Errorf("invalid gravity address: %s", args[0])
			}

			gravityAddr := ethcmn.HexToAddress(args[0])
			gravityContract, err := getGravityContract(ethRPC, gravityAddr)
			if err != nil {
				return err
			}

			baseDenom := args[1]
			bankQuerier := banktypes.NewQueryClient(gRPCConn)

			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			resp, err := bankQuerier.DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{Denom: baseDenom})
			if err != nil {
				return fmt.Errorf("failed to query for bank metadata: %w", err)
			}

			switch {
			case len(resp.Metadata.Name) == 0:
				return errors.New("token metadata name cannot be empty")

			case len(resp.Metadata.Symbol) == 0:
				return errors.New("token metadata symbol cannot be empty")

			case len(resp.Metadata.Display) == 0:
				return errors.New("token metadata display cannot be empty")
			}

			var decimals uint8
			for _, unit := range resp.Metadata.DenomUnits {
				if unit.Denom == resp.Metadata.Display {
					if unit.Exponent > math.MaxUint8 {
						return fmt.Errorf("token exponent too large; %d > %d", unit.Exponent, math.MaxInt8)
					}

					decimals = uint8(unit.Exponent)
					break
				}
			}

			tx, err := gravityContract.DeployERC20(auth, baseDenom, resp.Metadata.Name, resp.Metadata.Symbol, decimals)
			if err != nil {
				return fmt.Errorf("failed deploy Cosmos native ERC20 token: %w", err)
			}

			_, _ = fmt.Fprintf(os.Stderr, `Cosmos native token deployed as an ERC20 on Ethereum!
Base Denom: %s
Name: %s
Symbol: %s
Decimals: %d
Transaction: %s
`,
				baseDenom,
				resp.Metadata.Name,
				resp.Metadata.Symbol,
				decimals,
				tx.Hash().Hex(),
			)

			return nil
		},
	}

	return cmd
}

func deployERC20RawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy-erc20-raw [gravity-addr] [denom-base] [denom-name] [denom-symbol] [denom-decimals]",
		Short: "Deploy a Cosmos native asset on Ethereum as an ERC20 token using raw input",
		Long: `Deploy a Cosmos native asset on Ethereum as an ERC20 token using raw input.
The Gravity Bridge contract address along with all Cosmos native token denomination data
must be provided. This can be useful for deploying ERC20 tokens prior to the Umee
network starting.`,
		Args: cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			konfig, err := parseServerConfig(cmd)
			if err != nil {
				return err
			}

			ethRPCEndpoint := konfig.String(flagEthRPC)
			ethRPC, err := ethclient.Dial(ethRPCEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial Ethereum RPC node: %w", err)
			}

			auth, err := buildTransactOpts(konfig, ethRPC)
			if err != nil {
				return err
			}

			if !ethcmn.IsHexAddress(args[0]) {
				return fmt.Errorf("invalid gravity address: %s", args[0])
			}

			gravityAddr := ethcmn.HexToAddress(args[0])
			gravityContract, err := getGravityContract(ethRPC, gravityAddr)
			if err != nil {
				return err
			}

			denomBase := args[1]
			denomName := args[2]
			denomSymbol := args[3]

			denomDecimals, err := strconv.ParseUint(args[4], 10, 8)
			if err != nil {
				return fmt.Errorf("invalid denom decimals: %w", err)
			}

			tx, err := gravityContract.DeployERC20(auth, denomBase, denomName, denomSymbol, uint8(denomDecimals))
			if err != nil {
				return fmt.Errorf("failed deploy Cosmos native ERC20 token: %w", err)
			}

			_, _ = fmt.Fprintf(os.Stderr, `Cosmos native token deployed as an ERC20 on Ethereum!
Base Denom: %s
Name: %s
Symbol: %s
Decimals: %d
Transaction: %s
`,
				denomBase,
				denomName,
				denomSymbol,
				denomDecimals,
				tx.Hash().Hex(),
			)

			return nil
		},
	}
}

func sendToCosmosCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send-to-cosmos [gravity-addr] [token-address] [recipient] [amount]",
		Args:  cobra.ExactArgs(4),
		Short: "Send tokens from an Ethereum account to a recipient on Cosmos via Gravity Bridge",
		RunE: func(cmd *cobra.Command, args []string) error {
			konfig, err := parseServerConfig(cmd)
			if err != nil {
				return err
			}

			ethRPCEndpoint := konfig.String(flagEthRPC)
			ethRPC, err := ethclient.Dial(ethRPCEndpoint)
			if err != nil {
				return fmt.Errorf("failed to dial Ethereum RPC node: %w", err)
			}

			if !ethcmn.IsHexAddress(args[0]) {
				return fmt.Errorf("invalid gravity address: %s", args[0])
			}

			gravityAddr := ethcmn.HexToAddress(args[0])
			gravityContract, err := getGravityContract(ethRPC, gravityAddr)
			if err != nil {
				return err
			}

			if !ethcmn.IsHexAddress(args[1]) {
				return fmt.Errorf("invalid token address: %s", args[1])
			}

			tokenAddr := ethcmn.HexToAddress(args[1])

			if konfig.Bool(flagAutoApprove) {
				if err := approveERC20(konfig, ethRPC, tokenAddr, gravityAddr); err != nil {
					return err
				}
			}

			auth, err := buildTransactOpts(konfig, ethRPC)
			if err != nil {
				return err
			}

			recipientAddr, err := sdk.AccAddressFromBech32(args[2])
			if err != nil {
				return fmt.Errorf("failed to Bech32 decode recipient address: %w", err)
			}

			amount, ok := new(big.Int).SetString(args[3], 10)
			if !ok || amount == nil {
				return fmt.Errorf("invalid token amount: %s", args[3])
			}

			tx, err := gravityContract.SendToCosmos(auth, tokenAddr, recipientAddr.String(), amount)
			if err != nil {
				return fmt.Errorf("failed to send tokens to Cosmos: %w", err)
			}

			_, _ = fmt.Fprintf(os.Stderr, `Ethereum tokens successfully sent to Cosmos!
Token Address: %s
Sender: %s
Recipient: %s
Amount: %s
Transaction: %s
`,
				tokenAddr.String(),
				auth.From.String(),
				recipientAddr.String(),
				amount.String(),
				tx.Hash().Hex(),
			)

			return nil
		},
	}

	cmd.Flags().Bool(flagAutoApprove, true, "Auto approve the ERC20 for Gravity to spend from (using max uint256)")

	return cmd
}

func buildTransactOpts(konfig *koanf.Koanf, ethClient *ethclient.Client) (*bind.TransactOpts, error) {
	ethPrivKeyHexStr := konfig.String(flagEthPK)

	privKey, err := ethcrypto.ToECDSA(ethcmn.FromHex(ethPrivKeyHexStr))
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	publicKey := privKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("invalid public key; expected: %T, got: %T", &ecdsa.PublicKey{}, publicKey)
	}

	goCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fromAddress := ethcrypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := ethClient.PendingNonceAt(goCtx, fromAddress)
	if err != nil {
		return nil, err
	}

	goCtx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ethChainID, err := ethClient.ChainID(goCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ethereum chain ID: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privKey, ethChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ethereum transactor: %w", err)
	}

	var gasPrice *big.Int

	gasPriceInt := konfig.Int64(flagEthGasPrice)
	switch {
	case gasPriceInt < 0:
		return nil, fmt.Errorf("invalid Ethereum gas price: %d", gasPriceInt)

	case gasPriceInt > 0:
		gasPrice = big.NewInt(gasPriceInt)

	default:
		gasPrice, err = ethClient.SuggestGasPrice(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get Ethereum gas estimate: %w", err)
		}
	}

	gasLimit := konfig.Int64(flagEthGasLimit)
	if gasLimit < 0 {
		return nil, fmt.Errorf("invalid Ethereum gas limit: %d", gasLimit)
	}

	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.Value = big.NewInt(0)       // in wei
	auth.GasLimit = uint64(gasLimit) // in units
	auth.GasPrice = gasPrice

	return auth, nil
}

func getGravityParams(gRPCConn *grpc.ClientConn) (*gravitytypes.Params, error) {
	gravityQueryClient := gravitytypes.NewQueryClient(gRPCConn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	gravityParamsResp, err := gravityQueryClient.Params(ctx, &gravitytypes.QueryParamsRequest{})
	if err != nil || gravityParamsResp == nil {
		return nil, fmt.Errorf("failed to query for Gravity params: %w", err)
	}

	return &gravityParamsResp.Params, nil
}

func getGravityContract(ethRPC *ethclient.Client, gravityAddr ethcmn.Address) (*wrappers.Gravity, error) {
	contract, err := wrappers.NewGravity(gravityAddr, ethRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gravity contract instance: %w", err)
	}

	return contract, nil
}

func approveERC20(konfig *koanf.Koanf, ethRPC *ethclient.Client, erc20Addr, gravityAddr ethcmn.Address) error {
	contract, err := wrappers.NewERC20(erc20Addr, ethRPC)
	if err != nil {
		return fmt.Errorf("failed to create ERC20 contract instance: %w", err)
	}

	auth, err := buildTransactOpts(konfig, ethRPC)
	if err != nil {
		return err
	}

	// Check if the allowance remaining is greater than half of a Uint256 - it's
	// as good a test as any. If so, we skip approving Gravity as the spender and
	// assume it's already approved.
	allowance, err := contract.Allowance(nil, auth.From, gravityAddr)
	if err != nil {
		return fmt.Errorf("failed to get ERC20 allowance: %w", err)
	}

	if allowance.Cmp(halfMaxUint256) > 0 {
		_, _ = fmt.Fprintln(os.Stderr, "Skipping ERC20 contract approval")
		return nil
	}

	tx, err := contract.Approve(auth, gravityAddr, maxUint256)
	if err != nil {
		return fmt.Errorf("failed to approve ERC20 contract: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Approved ERC20 contract: %s\n", tx.Hash().Hex())
	return nil
}
