#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

echo "*** Peggy Contract Suite deployment ***"

### USER PROVIDED

# Initial Validator Set on Injective
PEGGY_ID="${PEGGY_ID:-0x696e6a6563746976652d70656767796964000000000000000000000000000000}" # bytes32 encoding of "injective-peggyid"
POWER_THRESHOLD="${POWER_THRESHOLD:-1431655765}" # how to get: 2/3 of total validator power on Injective
VALIDATOR_ADDRESSES="${VALIDATOR_ADDRESSES:-0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4,0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c,0x8B094eD440900CEB75B83A22eD8A2C7582B442C2}"
VALIDATOR_POWERS="${VALIDATOR_POWERS:-1431655765,1431655765,1431655765}"

# Peggy contracts
PEGGY_CONTRACT_PATH="../../solidity/contracts/Peggy.sol"
PROXY_ADMIN_CONTRACT_PATH="../../solidity/contracts/@openzeppelin/contracts/ProxyAdmin.sol"
PROXY_CONTRACT_PATH="../../solidity/contracts/@openzeppelin/contracts/TransparentUpgradeableProxy.sol"
COSMOS_TOKEN_CONTRACT_PATH="../../solidity/contracts/CosmosToken.sol"
COSMOS_TOKEN_DEPLOY_ARGS="Nasud nas 18"
COSMOS_TOKEN_MAX_AMOUNT=100000000000000000000000000 # 100 million tokens that will be minted straight to Peggy proxy

# Ethereum
DEPLOYER_PK="b8fe92b390dc8a830a9544be585bf87a4b1aa318beb629a8d60e0d83fa68eb72"
ETH_ENDPOINT="https://eth-sepolia.g.alchemy.com/v2/VranSAqL6UIW7YSsj_5mkHg7UMMHP6jR"

### DEPLOYMENT

PEGGY_INIT_ARGS="$PEGGY_ID $POWER_THRESHOLD $VALIDATOR_ADDRESSES $VALIDATOR_POWERS"
TX_OPTS="-P $DEPLOYER_PK --endpoint $ETH_ENDPOINT --tx-timeout 60s"
COSMOS_TOKEN_OPTS="$TX_OPTS --name CosmosERC20 --source $COSMOS_TOKEN_CONTRACT_PATH"
PEGGY_OPTS="$TX_OPTS --name Peggy --source $PEGGY_CONTRACT_PATH"
PROXY_ADMIN_OPTS="$TX_OPTS --name ProxyAdmin --source $PROXY_ADMIN_CONTRACT_PATH"
PEGGY_PROXY_OPTS="$TX_OPTS --name TransparentUpgradeableProxy --source $PROXY_CONTRACT_PATH"

echo "Deploying Peggy.sol ..."
peggy_address=$(etherman $PEGGY_OPTS deploy)
echo "$peggy_address"

echo "Initializing Peggy.sol ..."
peggy_init_data=$(etherman $PEGGY_OPTS tx --bytecode "$peggy_address" initialize $PEGGY_INIT_ARGS)
echo "$peggy_init_data"

echo "Deploying ProxyAdmin.sol ..."
proxy_admin_address=$(etherman $PROXY_ADMIN_OPTS deploy)
echo "$proxy_admin_address"

echo "Deploying TransparentUpgradeableProxy.sol ..."
peggy_proxy_address=$(etherman $PEGGY_PROXY_OPTS deploy "$peggy_address" "$proxy_admin_address" "$peggy_init_data")
echo "$peggy_proxy_address"


echo "Deploying Injective (CosmosERC20.sol) token ..."
cosmos_coin_address=$(etherman $COSMOS_TOKEN_OPTS deploy $COSMOS_TOKEN_DEPLOY_ARGS)
echo "$cosmos_coin_address"

echo "Minting 100_000_000 Injective tokens to Peggy.sol proxy ..."
etherman $COSMOS_TOKEN_OPTS tx "$cosmos_coin_address" mint "$peggy_proxy_address" $COSMOS_TOKEN_MAX_AMOUNT

echo "Done!"

echo "Contract addresses:"
echo "  * $peggy_address Peggy.sol"
echo "  * $proxy_admin_address ProxyAdmin.sol:"
echo "  * $peggy_proxy_address TransparentUpgradeableProxy.sol"
echo "  * $cosmos_coin_address Injective token"
echo -e "\n"
