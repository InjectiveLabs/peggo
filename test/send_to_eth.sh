#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

passphrase=12345678

deployer_pk=$(cat ./ethereum/geth/clique_signer.key)
peggy_contract="../solidity/contracts/Peggy.sol"
cosmos_token_contract="../solidity/contracts/CosmosToken.sol"

# Set up 11 txs with amount 1000000000000000000inj
# Threshold is:             10000000000000000000inj
# The last tx will be add to pending queue

# Send INJ tokens
echo "--------Sending to eth 1st tx--------"

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 2nd tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 3th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 4th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 5th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 6th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 7th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 8th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 9th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 10th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 11th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Checking ratelimit and balance after send to eth----------"
# Check ratelimit & balance after send to eth
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0

echo "--------Checking pending tx of 11th tx----------"
# Tx 11 should be pending
injectived query ratelimit pending-tx 11 --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0

# Waiting for 2 minute
sleep 120

echo "--------Checking 11th tx was processed-----------"
# Tx 11 should be processed
injectived query ratelimit pending-tx 11 --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0

# Sleep 2 minute then outflow should reduce to 0
sleep 120
echo "---------Check outflow was reduced to 0------------"
injectived query ratelimit list-peggy-rate-limits --chain-id=injective-333 --home /Users/hieuvu/Documents/Decentrio/peggo-decentrio/test/cosmos/data/injective-333/n0
