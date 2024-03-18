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

yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 2nd tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 3th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 4th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 5th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 6th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 7th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 8th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 9th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 10th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 11th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user


sleep 5
echo "--------Sending to eth 12nd tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 13nd tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 14th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 15th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 16th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 17th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 18th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 19th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 20th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 21th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

sleep 5
echo "--------Sending to eth 22th tx--------"
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 1000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 3000000000000000000peggy0x1ccec198630f2024c64c0afc5ae2427bc8e2dce8 --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

