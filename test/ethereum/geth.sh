#!/bin/bash

set -e

cd "${0%/*}" # cd to current script dir

CWD=$(pwd)

# These options can be overridden by env
GETH_PORT="${GETH_PORT:-8545}"
GETH_NETWORK_ID="${GETH_NETWORK_ID:-50}"
GETH_ALGO="${GETH_ALGO:-clique}"
GETH_BLOCK_GAS_LIMIT="${GETH_BLOCK_GAS_LIMIT:-60000000}"
CHAIN_DIR="${CHAIN_DIR:-$CWD/data}"
MINER_ADDR="0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e"

DATA_DIR="$CHAIN_DIR/$GETH_NETWORK_ID"

if [[ $GETH_ALGO != "clique" ]]; then
  echo "Unsupported geth algo: $GETH_ALGO. Must use clique"
  exit 1
fi

# Kill the node if it's already running
pid="$(cat "$DATA_DIR.geth.pid")"
if kill "$pid"  &>/dev/null; then
    rm "$DATA_DIR.geth.pid"
fi

sleep 1

# Start the local geth node
geth --datadir "$DATA_DIR" --networkid "$GETH_NETWORK_ID" --nodiscover \
  --http --http.port "$GETH_PORT" --http.api personal,eth,net,web3 --allow-insecure-unlock \
  --miner.etherbase $MINER_ADDR --unlock $MINER_ADDR --password ./geth/clique_password.txt \
  --mine --miner.gaslimit "$GETH_BLOCK_GAS_LIMIT" > "$DATA_DIR".geth.log 2>&1 &

echo $! > "$DATA_DIR".geth.pid # overwrite previous PID
PID=$(cat ./data/$GETH_NETWORK_ID.geth.pid)

sleep 1

echo "Geth:"
echo "  http://localhost:$GETH_PORT"
echo "Logs:"
echo "  tail -f ./data/$GETH_NETWORK_ID.geth.log"
echo "Shutdown:"
echo "  kill $PID"
echo
echo "Ethereum network setup complete!"
echo