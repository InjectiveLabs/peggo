#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

passphrase=12345678
deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
cosmos_token_contract="../solidity/contracts/CosmosToken.sol"

peggy_contract_address=0x5048019d259217e6b7BC8e1E6aEfa9976B1ADFfe
inj_coin_contract_address=0x7E5C521F8515017487750c13C3bF3B15f3f5f654

# Set up 11 txs with amount 1000000000000000000inj
# Threshold is:             10000000000000000000inj
# The last tx will be add to pending queue
etherman --name CosmosERC20 --source "$cosmos_token_contract" -P "$deployer_pk" call "$inj_coin_contract_address" balanceOf "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e"
# Send INJ tokens
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 10inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5

# Check ratelimit & balance after send to eth
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0
etherman --name CosmosERC20 --source "$cosmos_token_contract" -P "$deployer_pk" call "$inj_coin_contract_address" balanceOf "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e"

# Tx 11 should be pending
injectived query ratelimit pending-tx 11 --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0

# Waiting for a minute
sleep 120

# Tx 11 should be processed
injectived query ratelimit pending-tx 11 --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0
etherman --name CosmosERC20 --source "$cosmos_token_contract" -P "$deployer_pk" call "$inj_coin_contract_address" balanceOf "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e"

# Sleep 2 minute then outflow should reduce to 0
sleep 120
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0

