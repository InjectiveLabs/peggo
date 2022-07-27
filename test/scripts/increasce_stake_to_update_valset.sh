#!/bin/bash -eu

CWD="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
UMMED_BUILD_PATH=$CWD/../umeed-builds
UMEED_BIN=${UMEED_BIN:-$UMMED_BUILD_PATH/umeed-main}
CHAIN_ID="${CHAIN_ID:-888}"
CHAIN_DIR="${CHAIN_DIR:-$CWD/../cosmos/data}"

hdir="$CHAIN_DIR/$CHAIN_ID"
# Folders for nodes
n0dir="$hdir/n0"
n1dir="$hdir/n1"
n2dir="$hdir/n2"

# Common flags
kbt="--keyring-backend test"
cid="--chain-id $CHAIN_ID"
hardhatNetwork="--network ganache"

bridgeDeployed=$hdir/bridge_deployed.txt

bridgeAddr=$(cat $bridgeDeployed | grep Address | awk '{print $2}')

$CWD/print_block_number.sh

cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces

n0ValAddr=$($UMEED_BIN keys --home $n0dir $kbt show val --bech val -a)

amount=1000000000uumee

$UMEED_BIN tx staking delegate $n0ValAddr $amount \
  --home $n0dir $kbt --from val $cid -y --broadcast-mode block

echo .
echo Delegated $amount to $n0ValAddr
echo .

$CWD/print_block_number.sh

cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces

echo Wait for the orchestrator to read and check if the valset need to be updated
sleep 20

$CWD/print_block_number.sh
cd $CWD/../eth && npx hardhat $hardhatNetwork getCurrentValset $bridgeAddr --show-stack-traces