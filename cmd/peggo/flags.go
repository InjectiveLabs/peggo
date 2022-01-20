// nolint: lll
package peggo

import (
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/spf13/pflag"
)

const (
	logLevelJSON = "json"
	logLevelText = "text"

	flagLogLevel                = "log-level"
	flagLogFormat               = "log-format"
	flagSvcWaitTimeout          = "svc-wait-timeout"
	flagCosmosChainID           = "cosmos-chain-id"
	flagCosmosGRPC              = "cosmos-grpc"
	flagTendermintRPC           = "tendermint-rpc"
	flagCosmosGasPrices         = "cosmos-gas-prices"
	flagCosmosKeyring           = "cosmos-keyring"
	flagCosmosKeyringDir        = "cosmos-keyring-dir"
	flagCosmosKeyringApp        = "cosmos-keyring-app"
	flagCosmosFrom              = "cosmos-from"
	flagCosmosFromPassphrase    = "cosmos-from-passphrase"
	flagCosmosPK                = "cosmos-pk"
	flagCosmosUseLedger         = "cosmos-use-ledger"
	flagCosmosFeeGranter        = "cosmos-fee-granter"
	flagCosmosMsgsPerTx         = "cosmos-msgs-per-tx"
	flagEthKeystoreDir          = "eth-keystore-dir"
	flagEthFrom                 = "eth-from"
	flagEthPassphrase           = "eth-passphrase"
	flagEthPK                   = "eth-pk"
	flagEthUseLedger            = "eth-use-ledger"
	flagEthRPC                  = "eth-rpc"
	flagEthGasAdjustment        = "eth-gas-price-adjustment"
	flagEthGasLimitAdjustment   = "eth-gas-limit-adjustment"
	flagEthAlchemyWS            = "eth-alchemy-ws"
	flagRelayValsets            = "relay-valsets"
	flagRelayBatches            = "relay-batches"
	flagCoinGeckoAPI            = "coingecko-api"
	flagEthGasPrice             = "eth-gas-price"
	flagEthGasLimit             = "eth-gas-limit"
	flagAutoApprove             = "auto-approve"
	flagEthBlocksPerLoop        = "eth-blocks-per-loop"
	flagEthPendingTXWait        = "eth-pending-tx-wait"
	flagProfitMultiplier        = "profit-multiplier"
	flagRelayerLoopMultiplier   = "relayer-loop-multiplier"
	flagRequesterLoopMultiplier = "requester-loop-multiplier"
	flagBridgeStartHeight       = "bridge-start-height"
)

func cosmosFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)

	fs.String(flagCosmosChainID, "", "The chain ID of the cosmos network")
	fs.String(flagCosmosGRPC, "tcp://localhost:9090", "The gRPC endpoint of a cosmos node")
	fs.String(flagTendermintRPC, "http://localhost:26657", "The Tendermint RPC endpoint of a Cosmos node")
	fs.String(flagCosmosGasPrices, "", "The gas prices to use for Cosmos transaction fees")

	return fs
}

func cosmosKeyringFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)

	fs.String(flagCosmosKeyring, keyring.BackendFile, "Specify Cosmos keyring backend (os|file|kwallet|pass|test)")
	fs.String(flagCosmosKeyringDir, "", "Specify Cosmos keyring directory, if using file keyring")
	fs.String(flagCosmosKeyringApp, "peggo", "Specify Cosmos keyring app name")
	fs.String(flagCosmosFrom, "", "Specify the Cosmos orchestrator key name or address. If specified, must exist in keyring, ledger or match the privkey")
	fs.String(flagCosmosFromPassphrase, "", "Specify the keyring passphrase, otherwise STDIN will be used")
	fs.String(flagCosmosPK, "", "Specify a Cosmos account private key of the orchestrator in hex")
	fs.Bool(flagCosmosUseLedger, false, "Use the Cosmos app on a hardware ledger to sign transactions")

	return fs
}

func ethereumKeyOptsFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)

	fs.String(flagEthKeystoreDir, "", "Specify the Ethereum keystore directory (Geth-format) prefix")
	fs.String(flagEthFrom, "", "Specify the Ethereum from address; If specified, it must exist in the keystore, ledger or match the privkey")
	fs.String(flagEthPassphrase, "", "Specify the passphrase to unlock the private key from armor; If empty then STDIN is used")
	fs.String(flagEthPK, "", "Provide the Ethereum private key of the orchestrator in hex")
	fs.Bool(flagEthUseLedger, false, "Use the Ethereum app on hardware ledger to sign transactions")
	return fs
}

func ethereumOptsFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)

	fs.String(flagEthRPC, "http://localhost:8545", "Specify the RPC address of an Ethereum node")
	fs.Float64(flagEthGasAdjustment, float64(1.3), "Specify a gas price adjustment for Ethereum transactions")
	fs.Float64(flagEthGasLimitAdjustment, float64(1.2), "Specify a gas limit adjustment for Ethereum transactions")

	return fs
}

func bridgeFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("", pflag.ContinueOnError)

	fs.String(flagEthRPC, "http://localhost:8545", "Specify the RPC address of an Ethereum node")
	fs.String(flagEthPK, "", "Provide the Ethereum private key of the orchestrator in hex")
	fs.Int64(flagEthGasPrice, 0, "The Ethereum gas price to include in the transaction; If zero, gas price will be estimated")
	fs.Int64(flagEthGasLimit, 6000000, "The Ethereum gas limit to include in the transaction")

	return fs
}
