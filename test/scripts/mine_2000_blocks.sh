#!/bin/bash -eu

CWD="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

CHAIN_ID="${CHAIN_ID:-888}"
CHAIN_DIR="${CHAIN_DIR:-$CWD/../cosmos/data}"

hdir="$CHAIN_DIR/$CHAIN_ID"
# Folders for nodes
n0dir="$hdir/n0"
n1dir="$hdir/n1"
n2dir="$hdir/n2"

# Common flags
hardhatNetwork="--network ganache"

bridgeDeployed=$hdir/bridge_deployed.txt

bridgeAddr=$(cat $bridgeDeployed | grep Address | awk '{print $2}')

$CWD/print_block_number.sh

cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces

echo Mining 2000 blocks

cd $CWD/../eth && npx hardhat $hardhatNetwork run $CWD/../eth/scripts/mine_blocks.ts --show-stack-traces

$CWD/print_block_number.sh

cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces

echo Wait for the orchestrator to read and check if the valset need to be updated
sleep 35

cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces

sleep 10

$CWD/print_block_number.sh
cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces
