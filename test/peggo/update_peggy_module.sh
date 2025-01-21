#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

PASSPHRASE="12345678"
TX_OPTS="--chain-id injective-333 --keyring-backend test --broadcast-mode sync --yes"

peggy_params_json="./peggy_params.json"
chain_dir="../cosmos/data/injective-333"
n0_home_dir=$chain_dir/n0
n1_home_dir=$chain_dir/n1
n2_home_dir=$chain_dir/n2

# usage: resp_check [resp] [err_msg]
resp_check() {
  if [ "$(echo -e "$1" | awk -F"'" '/raw_log: /{print $2}')" != "[]" ]; then
    echo "$2"
    exit 1
  fi
}

echo "Peggy params update:"
cat $peggy_params_json
echo
echo "Submitting gov proposal to update Peggy module params..."

resp="$(yes $PASSPHRASE | injectived tx gov submit-proposal $peggy_params_json --home $n0_home_dir --from user --gas 2000000 --gas-prices 500000000inj $TX_OPTS)"
echo "$resp"

sleep 2

current_proposal_id=$(curl 'http://localhost:10337/cosmos/gov/v1beta1/proposals?proposal_status=0&pagination.limit=1&pagination.reverse=true' 2>/dev/null | jq -r '.proposals[].proposal_id')

yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n0_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null
yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n1_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null
yes $PASSPHRASE | injectived tx gov vote "$current_proposal_id" yes --home $n2_home_dir --from val --gas-prices 500000000inj $TX_OPTS &>/dev/null

sleep 8
echo "Gov proposal passed"

n0_inj_addr="inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r"
n1_inj_addr="inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs"
n2_inj_addr="inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz"

n0_eth_addr="0x4e9feE2BCdf6F21b17b77BD0ac9faDD6fF16B4d4"
n1_eth_addr="0xec43B0eA83844Cbe5A20F5371604BD452Cb1012c"
n2_eth_addr="0x8B094eD440900CEB75B83A22eD8A2C7582B442C2"

echo "Registering orchestrator ETH addresses..."
injectived tx peggy set-orchestrator-address $n0_inj_addr $n0_inj_addr $n0_eth_addr --home $n0_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null
injectived tx peggy set-orchestrator-address $n1_inj_addr $n1_inj_addr $n1_eth_addr --home $n1_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null
injectived tx peggy set-orchestrator-address $n2_inj_addr $n2_inj_addr $n2_eth_addr --home $n2_home_dir --from=val --gas-prices 100000000000000inj $TX_OPTS &>/dev/null

sleep 2
echo "Orchestrator addresses registered"
echo "  * val1=$n0_inj_addr orch1=$n0_inj_addr eth1=$n0_eth_addr"
echo "  * val2=$n0_inj_addr orch2=$n0_inj_addr eth2=$n0_eth_addr"
echo "  * val3=$n0_inj_addr orch3=$n0_inj_addr eth3=$n0_eth_addr"
