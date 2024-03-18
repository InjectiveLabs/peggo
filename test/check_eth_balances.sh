#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
cosmos_token_contract="../solidity/contracts/CosmosToken.sol"

peggy_contract_address=0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe
user_address=0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e
inj_coin_contract_address=0x7E5C521F8515017487750c13C3bF3B15f3f5f654
wut_coin_contract_address=0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8

etherman --name CosmosERC20 --source "$cosmos_token_contract" call "$inj_coin_contract_address" balanceOf "$user_address"

etherman --name CosmosERC20 --source "$cosmos_token_contract" call "$wut_coin_contract_address" balanceOf "$user_address"

etherman --name CosmosERC20 --source "$cosmos_token_contract" call "$inj_coin_contract_address" balanceOf "$peggy_contract_address"

etherman --name CosmosERC20 --source "$cosmos_token_contract" call "$wut_coin_contract_address" balanceOf "$peggy_contract_address"


