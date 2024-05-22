#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

passphrase=12345678

# Send INJ tokens
yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 10inj 3000000000000000000inj --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user

# Send WAT tokens (premined)
#yes $passphrase | injectived tx peggy send-to-eth "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" 10wut 1500000000000000000wut --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user
