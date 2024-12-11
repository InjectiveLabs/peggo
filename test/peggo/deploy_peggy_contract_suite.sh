#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

# get the bridge_contract_start_height early so orchestrators can catch the first Valset Updated event from Peggy.sol
peggy_block_number=$(curl http://localhost:8545 \
                -X POST \
                -H "Content-Type: application/json" \
                -d '{"id":1,"jsonrpc":"2.0", "method":"eth_getBlockByNumber","params":["latest", true]}' 2>/dev/null \
                | python3 -c "import sys, json; print(int(json.load(sys.stdin)['result']['number'], 0))")


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
COSMOS_TOKEN_DEPLOY_ARGS="Injective INJ 18"
COSMOS_TOKEN_MAX_AMOUNT=100000000000000000000000000 # 100 million tokens that will be minted straight to Peggy proxy

# Ethereum opts
DEPLOYER_PK=$(cat ../ethereum/geth/clique_signer.key)
ETH_ENDPOINT="http://localhost:8545"
TX_OPTS="-P $DEPLOYER_PK --endpoint $ETH_ENDPOINT"
COSMOS_TOKEN_OPTS="$TX_OPTS --name CosmosERC20 --source $COSMOS_TOKEN_CONTRACT_PATH"
PEGGY_OPTS="$TX_OPTS --name Peggy --source $PEGGY_CONTRACT_PATH"
PROXY_ADMIN_OPTS="$TX_OPTS --name ProxyAdmin --source $PROXY_ADMIN_CONTRACT_PATH"
PEGGY_PROXY_OPTS="$TX_OPTS --name TransparentUpgradeableProxy --source $PROXY_CONTRACT_PATH"


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
etherman $COSMOS_TOKEN_OPTS tx "$coin_contract_address" mint "$peggy_proxy_address" $COSMOS_TOKEN_MAX_AMOUNT

sleep 1

echo "Contract addresses:"
echo "  $peggy_impl_address Peggy.sol"
echo "  $proxy_admin_address ProxyAdmin.sol:"
echo "  $peggy_proxy_address TransparentUpgradeableProxy.sol"
echo "  $coin_contract_address Injective token"
echo

# Update peggy_params.json
peggy_params_json="./peggy_params.json"
jq --arg cosmos_coin_erc20 "$coin_contract_address" \
   --arg bridge_contract_height "$peggy_block_number" \
   --arg bridge_ethereum "$peggy_proxy_address" \
   '.messages[0].params.cosmos_coin_erc20_contract = $cosmos_coin_erc20 |
    .messages[0].params.bridge_contract_start_height = $bridge_contract_height |
    .messages[0].params.bridge_ethereum_address = $bridge_ethereum' \
   $peggy_params_json > tmpfile && mv tmpfile $peggy_params_json

echo "Peggy contracts deployed!"
echo
