#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

PEGGY_ID="${PEGGY_ID:-0x696e6a6563746976652d70656767796964000000000000000000000000000000}" # this is arbitrary
#PEGGY_ID="${PEGGY_ID:-"peggo-333"}" # this is arbitrary
POWER_THRESHOLD="${POWER_THRESHOLD:-1431655765}"
VALIDATOR_ADDRESSES="${VALIDATOR_ADDRESSES:-0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4,0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c,0x8B094eD440900CEB75B83A22eD8A2C7582B442C2}"
VALIDATOR_POWERS="${VALIDATOR_POWERS:-1431655765,1431655765,1431655765}"

#if [[ ! -f .env ]]; then
#        echo "Please create .env file, example is in .env.example"
#        exit 1
#fi

#peggy_impl_address=`etherman \
#        --name Peggy \
#        --source ../contracts/Peggy.sol \
#        deploy`

deployer_pk=$(cat ../ethereum/geth/clique_signer.key)
peggy_impl_address=$(etherman --name Peggy --source ../../solidity/contracts/Peggy.sol -P "$deployer_pk" deploy)

echo "Deployed Peggy implementation contract: $peggy_impl_address"
echo -e "===\n"

peggy_init_data=$(etherman \
        --name Peggy \
        --source ../../solidity/contracts/Peggy.sol \
        -P "$deployer_pk" \
        tx --bytecode $peggy_impl_address initialize \
        $PEGGY_ID \
        $POWER_THRESHOLD \
        $VALIDATOR_ADDRESSES \
        $VALIDATOR_POWERS)

echo "Using PEGGY_ID $PEGGY_ID"
echo "Using POWER_THRESHOLD $POWER_THRESHOLD"
echo "Using VALIDATOR_ADDRESSES $VALIDATOR_ADDRESSES"
echo "Using VALIDATOR_POWERS $VALIDATOR_POWERS"
echo -e "===\n"
echo "Peggy Init data: $peggy_init_data"
echo -e "===\n"

proxy_admin_address=$(etherman \
        --name ProxyAdmin \
        -P "$deployer_pk" \
        --source ../../solidity/contracts/@openzeppelin/contracts/ProxyAdmin.sol \
        deploy)

echo "Deployed ProxyAdmin contract: $proxy_admin_address"
echo -e "===\n"

peggy_proxy_address=$(etherman \
        --name TransparentUpgradeableProxy \
        --source ../../solidity/contracts/@openzeppelin/contracts/TransparentUpgradeableProxy.sol \
        -P "$deployer_pk" \
        deploy $peggy_impl_address $proxy_admin_address $peggy_init_data)

peggy_block_number=$(curl http://localhost:8545 \
                -X POST \
                -H "Content-Type: application/json" \
                -d '{"id":1,"jsonrpc":"2.0", "method":"eth_getBlockByNumber","params":["latest", true]}' \
                | python3 -c "import sys, json; print(int(json.load(sys.stdin)['result']['number'], 0))")

echo "Deployed TransparentUpgradeableProxy for $peggy_impl_address (Peggy), with $proxy_admin_address (ProxyAdmin) as the admin"
echo -e "===\n"
#
#echo "Deploying Injective token on Peggy.sol:"
#inj_token=$(etherman \
#        --name Peggy \
#        --source ../../solidity/contracts/Peggy.sol \
#        -P "$deployer_pk" \
#        tx --bytecode $peggy_impl_address deployERC20 \
#        "whatever_this_is" \
#        "Injective" \
#        "inj" \
#        18)
#
#echo "Deployed inj token: $inj_token"

# Deploy Cosmos Coin ERC20 contract
coin_contract_path="/Users/dbrajovic/Desktop/dev/Injective/peggo/solidity/contracts/CosmosToken.sol"
coin_contract_address=$(etherman --name CosmosERC20 -P "$deployer_pk" --source "$coin_contract_path" deploy 0x696e6a6563746976652d70656767796964 "Injective" "inj" 18)
echo "Deployed Cosmos Coin contract: $coin_contract_address"

echo "Peggy deployment done! Use $peggy_proxy_address"
echo "Block number is $peggy_block_number"
echo "$peggy_proxy_address" > ./peggy_proxy_address.txt
echo "$peggy_block_number" > ./peggy_block_number.txt
echo "$coin_contract_address" > ./peggy_coin_address.txt