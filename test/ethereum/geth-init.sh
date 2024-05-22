#!/bin/bash

set -e

cd "${0%/*}" # cd to current script dir

CWD=$(pwd)

# These options can be overridden by env
GETH_NETWORK_ID="${GETH_NETWORK_ID:-50}"
GETH_ALGO="${GETH_ALGO:-clique}"
CHAIN_DIR="${CHAIN_DIR:-$CWD/data}"

if [[ $GETH_ALGO != "clique" ]]; then
  echo "Unsupported geth algo: $GETH_ALGO. Must use clique"
  exit 1
fi

DATA_DIR="$CHAIN_DIR/$GETH_NETWORK_ID"

# Initialize geth dir and setup account
geth init --datadir "$DATA_DIR" ./geth/clique_genesis.json
geth account import --datadir "$DATA_DIR" --lightkdf --password ./geth/clique_password.txt ./geth/clique_signer.key

# Create PID and log file
touch "$CHAIN_DIR/$GETH_NETWORK_ID.geth.pid"
touch "$CHAIN_DIR/$GETH_NETWORK_ID.geth.log"