#!/bin/bash -eu

CWD="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
ETH_RPC=${ETH_RPC:-"http://127.0.0.1:8545"}
UMMED_BUILD_PATH=$CWD/../umeed-builds
UMEED_BIN=${UMEED_BIN:-$UMMED_BUILD_PATH/umeed-main}
PEGGO_BUILD_PATH=$CWD/../../build
PEGGO_BIN=${PEGGO_BIN:-$PEGGO_BUILD_PATH/peggo}
CHAIN_ID="${CHAIN_ID:-888}"
CHAIN_DIR="${CHAIN_DIR:-$CWD/../cosmos/data}"

rpc="--eth-rpc $ETH_RPC"

hdir="$CHAIN_DIR/$CHAIN_ID"
# Folders for nodes
n0dir="$hdir/n0"
n1dir="$hdir/n1"
n2dir="$hdir/n2"

umeeBasename=$(basename $UMEED_BIN)
ganache="ganache-cli"

# Common flags
kbt="--keyring-backend test"
cid="--chain-id $CHAIN_ID"

if pgrep -x $umeeBasename >/dev/null
then
  echo "$umeeBasename is running, going to kill all"
  ps -ef | grep $umeeBasename | grep -v grep | awk '{print $2}' | xargs kill
fi

if pgrep -x $ganache >/dev/null
then
  echo "$ganache is running, going to kill all"
  ps -ef | grep $ganache | grep -v grep | awk '{print $2}' | xargs kill
fi

CLEANUP=1 $CWD/../cosmos/multinode.sh $UMEED_BIN

# val0 0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101
val0PrivateKey="0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
# val1 0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17
val1PrivateKey="0x741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544"
# val2 0x6880D7bfE96D49501141375ED835C24cf70E2bD7
val2PrivateKey="0x39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608"
# user 0x727AEE334987c52fA7b567b2662BDbb68614e48C
userPrivateKey="0x6c212553111b370a8ffdc682954495b7b90a73cedab7106323646a4f2c4e668f"

$CWD/../eth/run_ganache.sh

echo Wait for ganache to start and produce some blocks
sleep 20

bridgeDeployed=$hdir/bridge_deployed.txt

PEGGO_ETH_PK=$val0PrivateKey $PEGGO_BIN bridge deploy-gravity $rpc 2> $bridgeDeployed

bridgeAddr=$(cat $bridgeDeployed | grep Address | awk '{print $2}')

defaultFlags="$rpc --relay-batches=true --valset-relay-mode=minimum \
  --cosmos-chain-id=$CHAIN_ID --cosmos-keyring=test \
  --cosmos-from=val --log-level debug \
  --profit-multiplier=0
"

peggoLogPath=$hdir/peggo
mkdir -p $peggoLogPath

PEGGO_ETH_PK=$val0PrivateKey $PEGGO_BIN orchestrator $bridgeAddr \
  $defaultFlags \
  --cosmos-grpc="tcp://0.0.0.0:9090" \
  --tendermint-rpc="http://0.0.0.0:26657" \
  --cosmos-keyring-dir=$n0dir > $peggoLogPath/n0.peggo.log 2>&1 &

PEGGO_ETH_PK=$val1PrivateKey $PEGGO_BIN orchestrator $bridgeAddr \
  $defaultFlags \
  --cosmos-grpc="tcp://0.0.0.0:9091" \
  --tendermint-rpc="http://0.0.0.0:26667" \
  --cosmos-keyring-dir=$n1dir > $peggoLogPath/n1.peggo.log 2>&1 &

PEGGO_ETH_PK=$val2PrivateKey $PEGGO_BIN orchestrator $bridgeAddr \
  $defaultFlags \
  --cosmos-grpc="tcp://0.0.0.0:9092" \
  --tendermint-rpc="http://0.0.0.0:26677" \
  --cosmos-keyring-dir=$n2dir > $peggoLogPath/n2.peggo.log 2>&1 &

echo Wait for a few seconds to get the current valset
sleep 15

$CWD/print_block_number.sh

echo .
echo Increasing the stake of one validator, it should not update the valset
echo Because the members did not changed and 2000 blocks did not pass
echo Since the last updated valset and valset-relay-mode=minimum
echo .
sleep 1

$CWD/increasce_stake_to_update_valset.sh

echo Stake increasced
sleep 2

$CWD/mine_2000_blocks.sh

echo after mining 2000 blocks, it should update
