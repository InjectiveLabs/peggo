#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
peggy_contract_address=0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe

etherman --name Peggy --source "$peggy_contract" -P "$deployer_pk" tx "$peggy_contract_address" deployERC20 "wut" "wat" "wat" 18
