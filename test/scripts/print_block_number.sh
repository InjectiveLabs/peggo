#!/bin/bash -eu

ETH_RPC=${ETH_RPC:-"http://127.0.0.1:8545"}

blockNumberHex=$(curl -X POST -s  \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":83}' \
  $ETH_RPC | jq -r '.result')

echo "Hex block number $blockNumberHex"
blockNumberHex=${blockNumberHex:2}
echo "Dec block number" $((16#$blockNumberHex))