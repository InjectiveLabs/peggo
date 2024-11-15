#!/bin/bash

set -e

cd "${0%/*}" # cd in the script dir

passphrase=12345678
tx_opts="--chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from user"

# Send INJ tokens
yes $passphrase | injectived tx peggy blacklist-ethereum-addresses "0xBbDf3283d1Cf510c17B4FfA1b900F444bE4A4A4e" $tx_opts

