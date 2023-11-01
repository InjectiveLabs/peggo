#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
cosmos_token_contract="../solidity/contracts/CosmosToken.sol"

peggy_contract_address=0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe
inj_coin_contract_address=$(cat ./peggo/peggy_coin_address.txt)

etherman --name CosmosERC20 --source "$cosmos_token_contract" -P "$deployer_pk" tx "$inj_coin_contract_address" approve "$peggy_contract_address" 100
etherman --name Peggy --source "$peggy_contract" -P "$deployer_pk" tx "$peggy_contract_address" sendToInjective "$inj_coin_contract_address" 727aee334987c52fa7b567b2662bdbb68614e48c 10 ""

