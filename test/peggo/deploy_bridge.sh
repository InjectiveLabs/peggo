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
example_env=$cwd/example.env
localhost_tcp="tcp://localhost"
localhost_http="http://localhost"



n0_data_dir=$cwd/data/n0
n0_peggo_env=$n0_data_dir/.env
n0_eth_from="0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4"
n0_eth_pk="e85344fa1e00f06bd286b716e410ee0ad73541956c4cf59520f6db13599eb3f3"
n0_cosmos_grpc="$localhost_tcp:9090"
n0_tendermint_rpc="$localhost_http:26657"
n0_cosmos_keyring="/Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0"

#n0_data_dir=$data_data/n0
#n0_data_dir=$data_data/n0


mkdir -p "$n0_data_dir"
touch "$n0_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n0_eth_from\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n0_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n0_cosmos_keyring\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n0_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n0_tendermint_rpc\"|" \
    "$example_env" > "$n0_peggo_env"




n1_data_dir=$cwd/data/n1
n1_peggo_env=$n1_data_dir/.env
n1_eth_from="0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c"
n1_eth_pk="60f6ee19454b8ff45693cd54c55860785e4af9eeb06d6c5617568458e4ca5c54"
n1_cosmos_keyring="/Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n1"
n1_cosmos_grpc="$localhost_tcp:9091"
n1_tendermint_rpc="$localhost_http:26667"

#n0_data_dir=$data_data/n0
#n0_data_dir=$data_data/n0


mkdir -p "$n1_data_dir"
touch "$n1_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n1_eth_from\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n1_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n1_cosmos_keyring\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n1_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n1_tendermint_rpc\"|" \
    "$example_env" > "$n1_peggo_env"



n2_data_dir=$cwd/data/n2
n2_peggo_env=$n2_data_dir/.env
n2_eth_from="0x8B094eD440900CEB75B83A22eD8A2C7582B442C2"
n2_eth_pk="21eeff959d9752704e3f1ad6562fd0458c003bce0947e5aecf07b602f4e457aa"
n2_cosmos_keyring="/Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n2"
n2_cosmos_grpc="$localhost_tcp:9092"
n2_tendermint_rpc="$localhost_http:26677"

#n0_data_dir=$data_data/n0
#n0_data_dir=$data_data/n0


mkdir -p "$n2_data_dir"
touch "$n2_peggo_env"

sed -e "s|^PEGGO_ETH_FROM=.*|PEGGO_ETH_FROM=\"$n2_eth_from\"|" \
    -e "s|^PEGGO_ETH_PK=.*|PEGGO_ETH_PK=\"$n2_eth_pk\"|" \
    -e "s|^PEGGO_COSMOS_KEYRING_DIR=.*|PEGGO_COSMOS_KEYRING_DIR=\"$n2_cosmos_keyring\"|" \
    -e "s|^PEGGO_COSMOS_GRPC=.*|PEGGO_COSMOS_GRPC=\"$n2_cosmos_grpc\"|" \
    -e "s|^PEGGO_TENDERMINT_RPC=.*|PEGGO_TENDERMINT_RPC=\"$n2_tendermint_rpc\"|" \
    "$example_env" > "$n2_peggo_env"


# Start a new tmux session
tmux new-session -d -s mysession

# Split the terminal vertically into three equally spaced panes
tmux split-window -v
tmux split-window -v
tmux select-layout even-vertical

# Select each pane and run a command from a different directory
peggo_cmd="peggo orchestrator"
tmux send-keys -t 0 "cd $n0_data_dir" C-m "$peggo_cmd" C-m
tmux send-keys -t 1 "cd $n1_data_dir" C-m "$peggo_cmd" C-m
tmux send-keys -t 2 "cd $n2_data_dir" C-m "$peggo_cmd" C-m

# Attach to the tmux session to view the processes
tmux attach-session -t mysession

