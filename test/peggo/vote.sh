#!/bin/bash


PASSPHRASE="12345678"

set -e

cd "${0%/*}" # cd in the script dir

vote() {
        PROPOSAL_ID=$1
        echo $PROPOSAL_ID
        echo "Voting on proposal: $PROPOSAL_ID"
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n0 --from inj1cml96vmptgw99syqrrz8az79xer2pcgp0a885r
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n1 --from inj1jcltmuhplrdcwp7stlr4hlhlhgd4htqhe4c0cs
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-333 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=sync --yes --home /Users/dbrajovic/Desktop/dev/Injective/peggo/test/cosmos/data/injective-333/n2 --from inj1dzqd00lfd4y4qy2pxa0dsdwzfnmsu27hgttswz
}

fetch_proposal_id() {
        current_proposal_id=$(curl 'http://localhost:10337/cosmos/gov/v1beta1/proposals?proposal_status=0&pagination.limit=1&pagination.reverse=true' | jq -r '.proposals[].proposal_id')
        echo "Current proposal ID: $current_proposal_id"
        proposal=$((current_proposal_id))
}


fetch_proposal_id
vote $proposal

