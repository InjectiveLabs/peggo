#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

echo -e "\n"
echo "*** Peggy Contract Suite deployment ***"

# Initial Validator Set on Injective
PEGGY_ID="${PEGGY_ID:-0x696e6a6563746976652d70656767796964000000000000000000000000000000}" # bytes32 encoding of "injective-peggyid"
POWER_THRESHOLD="${POWER_THRESHOLD:-1431655765}" # how to get: 2/3 of total validator power on Injective

VALIDATOR_ADDRESSES="${VALIDATOR_ADDRESSES:-\
0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4,\
0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c,\
0x8B094eD440900CEB75B83A22eD8A2C7582B442C2}"

VALIDATOR_POWERS="${VALIDATOR_POWERS:-\
1431655765,\
1431655765,\
1431655765}"

PEGGY_INIT_ARGS="$PEGGY_ID $POWER_THRESHOLD $VALIDATOR_ADDRESSES $VALIDATOR_POWERS"

# Peggy contracts
PEGGY_CONTRACT_PATH="../../solidity/contracts/Peggy.sol"
PROXY_ADMIN_CONTRACT_PATH="../../solidity/contracts/@openzeppelin/contracts/ProxyAdmin.sol"
PROXY_CONTRACT_PATH="../../solidity/contracts/@openzeppelin/contracts/TransparentUpgradeableProxy.sol"
COSMOS_TOKEN_CONTRACT_PATH="../../solidity/contracts/CosmosToken.sol"
COSMOS_TOKEN_DEPLOY_ARGS="Injective inj 18"
COSMOS_TOKEN_MAX_AMOUNT=100000000000000000000000000 # 100 million tokens that will be minted straight to Peggy proxy

# Ethereum
DEPLOYER_PK=$(cat ../ethereum/geth/clique_signer.key)
ETH_ENDPOINT="http://localhost:8545"

TX_OPTS="-P $DEPLOYER_PK --endpoint $ETH_ENDPOINT"
COSMOS_TOKEN_OPTS="$TX_OPTS --name CosmosERC20 --source $COSMOS_TOKEN_CONTRACT_PATH"
PEGGY_OPTS="$TX_OPTS --name Peggy --source $PEGGY_CONTRACT_PATH"
PROXY_ADMIN_OPTS="$TX_OPTS --name ProxyAdmin --source $PROXY_ADMIN_CONTRACT_PATH"
PEGGY_PROXY_OPTS="$TX_OPTS --name TransparentUpgradeableProxy --source $PROXY_CONTRACT_PATH"

# get the bridge_contract_start_height early so orchestrators can catch the first Valset Updated event from Peggy.sol
peggy_block_number=$(curl http://localhost:8545 \
                -X POST \
                -H "Content-Type: application/json" \
                -d '{"id":1,"jsonrpc":"2.0", "method":"eth_getBlockByNumber","params":["latest", true]}' 2>/dev/null \
                | python3 -c "import sys, json; print(int(json.load(sys.stdin)['result']['number'], 0))")


echo "Deploying Peggy.sol ..."
peggy_impl_address=$(etherman $PEGGY_OPTS deploy)

sleep 1

echo "Initializing Peggy.sol ..."
peggy_init_data=$(etherman $PEGGY_OPTS tx --bytecode "$peggy_impl_address" initialize $PEGGY_INIT_ARGS)

sleep 1

echo "Deploying ProxyAdmin.sol ..."
proxy_admin_address=$(etherman $PROXY_ADMIN_OPTS deploy)

sleep 1

echo "Deploying TransparentUpgradeableProxy.sol ..."
peggy_proxy_address=$(etherman $PEGGY_PROXY_OPTS deploy "$peggy_impl_address" "$proxy_admin_address" "$peggy_init_data")

sleep 1

echo "Deploying Injective (CosmosERC20.sol) token ..."
coin_contract_address=$(etherman $COSMOS_TOKEN_OPTS deploy $COSMOS_TOKEN_DEPLOY_ARGS)

sleep 1

echo "Minting 100_000_000 Injective tokens to Peggy.sol proxy ..."
etherman $COSMOS_TOKEN_OPTS tx  "$coin_contract_address" mint "$peggy_proxy_address" $COSMOS_TOKEN_MAX_AMOUNT

sleep 1

echo "Done!"

echo "Contract addresses:"
echo "  * $peggy_impl_address Peggy.sol"
echo "  * $proxy_admin_address ProxyAdmin.sol:"
echo "  * $peggy_proxy_address TransparentUpgradeableProxy.sol"
echo "  * $coin_contract_address Injective token"
echo -e "\n"

sleep 2

PASSPHRASE="12345678"
TX_OPTS="--chain-id injective-333 --keyring-backend test --broadcast-mode sync --yes"

peggy_params_json="./peggy_params.json"
chain_dir="/Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333"
n0_home_dir=$chain_dir/n0
n1_home_dir=$chain_dir/n1
n2_home_dir=$chain_dir/n2

# Update peggy_params.json
jq --arg cosmos_coin_erc20 "$coin_contract_address" \
   --arg bridge_contract_height "$peggy_block_number" \
   --arg bridge_ethereum "$peggy_proxy_address" \
   '.messages[0].params.cosmos_coin_erc20_contract = $cosmos_coin_erc20 |
    .messages[0].params.bridge_contract_start_height = $bridge_contract_height |
    .messages[0].params.bridge_ethereum_address = $bridge_ethereum' \
   $peggy_params_json > tmpfile && mv tmpfile $peggy_params_json

# usage: resp_check [resp] [err_msg]
resp_check() {
  if [ "$(echo -e "$1" | awk -F"'" '/raw_log: /{print $2}')" != "[]" ]; then
    echo "$2"
    exit 1
  fi
}

echo "Submitting gov proposal for Peggy module params update..."
cat $peggy_params_json

#resp="$(yes $PASSPHRASE | injectived tx gov submit-proposal $peggy_params_json --home $n0_home_dir --from user --gas 2000000 --gas-prices 500000000inj $TX_OPTS)"
#resp_check "$resp" "Failed to submit gov proposal"

resp="$(yes $PASSPHRASE | injectived tx gov submit-proposal $peggy_params_json --home $n0_home_dir --from user --gas 2000000 --gas-prices 500000000inj $TX_OPTS)"
echo "$resp"

sleep 2

current_proposal_id=$(curl 'http://localhost:10337/cosmos/gov/v1beta1/proposals?proposal_status=0&pagination.limit=1&pagination.reverse=true' 2>/dev/null | jq -r '.proposals[].proposal_id')

yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n0_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null
yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n1_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null
yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n2_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null

echo -n "Waiting for proposal to pass... "
sleep 8
echo "DONE"

n0_inj_addr="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r"
n1_inj_addr="inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs"
n2_inj_addr="inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"

n0_eth_addr="0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4"
n1_eth_addr="0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c"
n2_eth_addr="0x8B094eD440900CEB75B83A22eD8A2C7582B442C2"

injectived tx peggy set-orchestrator-address $n0_inj_addr $n0_inj_addr $n0_eth_addr --home $n0_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null
injectived tx peggy set-orchestrator-address $n1_inj_addr $n1_inj_addr $n1_eth_addr --home $n1_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null
injectived tx peggy set-orchestrator-address $n2_inj_addr $n2_inj_addr $n2_eth_addr --home $n2_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null

echo -n "Registering orchestrator ETH addresses... "
sleep 2
echo "DONE"

# Start peggo service
echo "Starting 3x Peggo orchestrators..."

cwd=$(pwd)
example_env=$cwd/example.env
localhost_tcp="tcp://localhost"
localhost_http="http://localhost"

n0_peggo_dir=$cwd/data/n0
n1_peggo_dir=$cwd/data/n1
n2_peggo_dir=$cwd/data/n2

n0_peggo_env=$n0_peggo_dir/.env
n1_peggo_env=$n1_peggo_dir/.env
n2_peggo_env=$n2_peggo_dir/.env

mkdir -p "$n0_peggo_dir" && touch "$n0_peggo_env"
mkdir -p "$n1_peggo_dir" && touch "$n1_peggo_env"
mkdir -p "$n2_peggo_dir" && touch "$n2_peggo_env"

n0_cosmos_grpc="$localhost_tcp:9090"
n1_cosmos_grpc="$localhost_tcp:9091"
n2_cosmos_grpc="$localhost_tcp:9092"

n0_tendermint_rpc="$localhost_http:26657"
n1_tendermint_rpc="$localhost_http:26667"
n2_tendermint_rpc="$localhost_http:26677"

n0_eth_pk="e85344fa1e00f06bd286b716e410ee0ad73541956c4cf59520f6db13599eb3f3"
n1_eth_pk="60f6ee19454b8ff45693cd54c55860785e4af9eeb06d6c5617568458e4ca5c54"
n2_eth_pk="21eeff959d9752704e3f1ad6562fd0458c003bce0947e5aecf07b602f4e457aa"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n0_eth_addr\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n0_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n0_home_dir\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n0_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n0_tendermint_rpc\"|" \
    "$example_env" > "$n0_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n1_eth_addr\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n1_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n1_home_dir\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n1_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n1_tendermint_rpc\"|" \
    "$example_env" > "$n1_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n2_eth_addr\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n2_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n2_home_dir\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n2_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n2_tendermint_rpc\"|" \
    "$example_env" > "$n2_peggo_env"

# One relayer has lower min batch fee
CHEAP_RELAYER="${CHEAP_RELAYER:-false}"
if [[ "$CHEAP_RELAYER" == true ]]; then
  echo "Setting n2 orchestrator with PEGGO_MIN_BATCH_FEE_USD to 10"
  echo "$n2_peggo_env"
  sed -i '' 's/^PEGGO_MIN_BATCH_FEE_USD=.*/PEGGO_MIN_BATCH_FEE_USD=10/' "$n2_peggo_env"
fi

# Start a new tmux session
tmux new-session -d -s mysession

# Split the terminal vertically into three equally spaced panes
tmux split-window -v
tmux split-window -v
tmux select-layout even-vertical

# Select each pane and run a command from a different directory
peggo_cmd="peggo orchestrator"
tmux send-keys -t 0 "cd $n0_peggo_dir" C-m "$peggo_cmd" C-m
tmux send-keys -t 1 "cd $n1_peggo_dir" C-m "$peggo_cmd" C-m
tmux send-keys -t 2 "cd $n2_peggo_dir" C-m "$peggo_cmd" C-m

# Attach to the tmux session to view the processes
tmux attach-session -t mysession


