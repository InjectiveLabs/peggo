#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

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

chain_dir=$(realpath "../cosmos/data/injective-333")
n0_home_dir=$chain_dir/n0
n1_home_dir=$chain_dir/n1
n2_home_dir=$chain_dir/n2

n0_eth_pk="e85344fa1e00f06bd286b716e410ee0ad73541956c4cf59520f6db13599eb3f3"
n1_eth_pk="60f6ee19454b8ff45693cd54c55860785e4af9eeb06d6c5617568458e4ca5c54"
n2_eth_pk="21eeff959d9752704e3f1ad6562fd0458c003bce0947e5aecf07b602f4e457aa"

n0_eth_addr="0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4"
n1_eth_addr="0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c"
n2_eth_addr="0x8B094eD440900CEB75B83A22eD8A2C7582B442C2"

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


