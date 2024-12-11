#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

killall injectived geth &>/dev/null || true

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

# Deploy Peggy contracts suite
"$peggo_dir"/deploy_peggy_contract_suite.sh

# Update Peggy module and register orchestrators
"$peggo_dir"/update_peggy_module.sh

# Start the orchestrators
"$peggo_dir"/start_orchestrators.sh
