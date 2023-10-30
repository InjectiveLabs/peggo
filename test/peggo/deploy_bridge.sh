#!/bin/bash

PASSPHRASE="12345678"

set -e

cd "${0%/*}" # cd in the script dir
#
#vote() {
#        PROPOSAL_ID=$1
#        echo $PROPOSAL_ID
#        echo "Voting on proposal: $PROPOSAL_ID"
#
#        echo "val0 voting yes"
#        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r
#
#        echo "val1 voting yes"
#        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n1 --from inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs
#
#        echo "val2 voting yes"
#        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n2 --from inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz
#}
#
#fetch_proposal_id() {
#        current_proposal_id=$(curl 'http://localhost:10337/cosmos/gov/v1beta1/proposals?proposal_status=0&pagination.limit=1&pagination.reverse=true' | jq -r '.proposals[].proposal_id')
#proposal=$((current_proposal_id))
#}
#
#TX_OPTS="--gas=2000000 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from=user"
#
##cat ./peggo_params.json | jq ".changes[0].value=\"$(cat ./peggy_proxy_address.txt)\"" > ./peggo_params.json
##cat ./peggo_params.json | jq ".changes[1].value=\"$(cat ./peggy_coin_address.txt)\"" > ./peggo_params.json
##cat ./peggo_params.json | jq ".changes[2].value=\"$(cat ./peggy_block_number.txt)\"" > ./peggo_params.json
#
##jq --arg peggy_proxy "$(cat ./peggy_proxy_address.txt)" \
##   --arg peggy_coin "$(cat ./peggy_coin_address.txt)" \
##   --arg peggy_block "$(cat ./peggy_block_number.txt)" \
##   '.changes[0].value = $peggy_proxy | .changes[1].value = $peggy_coin | .changes[2].value = $peggy_block' \
##   ./peggy_params.json > tmpfile && mv tmpfile ./peggo_params.json
#
#
#
## Use jq to update the JSON file
#jq --arg cosmos_coin_erc20 "$(cat ./peggy_coin_address.txt)" \
#   --arg bridge_contract_height "$(cat ./peggy_block_number.txt)" \
#   --arg bridge_ethereum "$(cat ./peggy_proxy_address.txt)" \
#   '.messages[0].params.cosmos_coin_erc20_contract = $cosmos_coin_erc20 |
#    .messages[0].params.bridge_contract_start_height = $bridge_contract_height |
#    .messages[0].params.bridge_ethereum_address = $bridge_ethereum' \
#   ./peggy_params.json > tmpfile
#
## Replace the original JSON file with the updated one
#mv tmpfile ./peggy_params.json
#
#
##echo "Peggy params json:"
##echo $(cat ./peggy_params.json)
#
#echo "ID before proposal: $current_proposal_id"
#
#
#echo "Submitting gov proposal for peggy params update..."
#yes $PASSPHRASE | injectived tx gov submit-proposal ./peggy_params.json $TX_OPTS
#
#sleep 2
#
#fetch_proposal_id
#
#echo "ID after proposal: $current_proposal_id"
#
#vote $proposal
#
#
##echo $(pwd)
##rm ./peggy_proxy_address.txt
##rm ./peggy_block_number.txt
#
#sleep 2
#
#injectived tx peggy set-orchestrator-address inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r 0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4 --chain-id=injective-333 --gas-prices 100000000000000inj --broadcast-mode=sync --yes --keyring-backend test --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from=val
#
#injectived tx peggy set-orchestrator-address inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs 0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c --chain-id=injective-333 --gas-prices 100000000000000inj --broadcast-mode=sync --yes --keyring-backend test --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n1 --from=val
#
#injectived tx peggy set-orchestrator-address inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz 0x8B094eD440900CEB75B83A22eD8A2C7582B442C2 --chain-id=injective-333 --gas-prices 100000000000000inj --broadcast-mode=sync --yes --keyring-backend test --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n2 --from=val
#
##sudo systemctl start peggo
##sudo systemctl start peggo1
##sudo systemctl start peggo2


# Start peggo service

cwd=$(pwd)
data_dir=$cwd/data

n0_peggo_env=$data_dir/n0/.env
n0_eth_from="0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4"
n0_eth_pk="e85344fa1e00f06bd286b716e410ee0ad73541956c4cf59520f6db13599eb3f3"
n0_cosmos_grpc="tcp://localhost:9090"
n0_tendermint_rpc="http://localhost:26657"
n0_cosmos_keyring="/Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0"

#n0_data_dir=$data_data/n0
#n0_data_dir=$data_data/n0

example_env=$cwd/example.env

mkdir -p "$data_dir/n0"
touch "$n0_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n0_eth_from\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n0_eth_pk\"|" \
    "$example_env" > "$n0_peggo_env"


