#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
inj_coin_contract=$(cat ./peggo/peggy_coin_address.txt)

etherman --name Peggy --source "$peggy_contract" -P "$deployer_pk" tx 0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe sendToInjective "$inj_coin_contract" 727aee334987c52fa7b567b2662bdbb68614e48c 1 ""

