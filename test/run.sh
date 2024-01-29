#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

cwd=$(pwd)
cosmos_dir="$cwd/cosmos"
eth_dir="$cwd/ethereum"
peggo_dir="$cwd/peggo"

rm -rf "$cosmos_dir/data"
rm -rf "$eth_dir/data"
rm -rf "$peggo_dir/data"
rm -rf "$peggo_dir/build"

# Start the Ethereum chain
"$eth_dir"/geth-init.sh
"$eth_dir"/geth.sh

# Start the Cosmos chain
"$cosmos_dir"/multinode.sh injectived

# Deploy Peggy contract suite and start Peggo orchestrators
"$peggo_dir"/deploy_bridge.sh
