#!/bin/bash

PASSPHRASE="12345678"

vote() {
        PROPOSAL_ID=$1
        echo $PROPOSAL_ID
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-777 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=block --yes --home ~/injective/injective-exchange/var/data/injective-777/n0 --from=val
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-777 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=block --yes --home ~/injective/injective-exchange/var/data/injective-777/n1 --from=val
        yes $PASSPHRASE | injectived tx gov vote $PROPOSAL_ID yes --chain-id=injective-777 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=block --yes --home ~/injective/injective-exchange/var/data/injective-777/n2 --from=val
}

fetch_proposal_id() {
        current_proposal_id=$(curl 'http://localhost:10337/cosmos/gov/v1beta1/proposals?proposal_status=0&pagination.limit=1&pagination.reverse=true' | jq -r '.proposals[].proposal_id')
proposal=$((current_proposal_id))
}

TX_OPTS="--gas=2000000 --chain-id=injective-777 --gas-prices 500000000inj --keyring-backend test --broadcast-mode=block --yes --home ~/injective/injective-exchange/var/data/injective-777/n0 --from=val"

cat /root/scripts/peggo_params.json | jq ".changes[0].value=\"$(cat /root/scripts/peggy_proxy_address.txt)\"" > /root/scripts/peggo_params.json

cat /root/scripts/peggo_params.json | jq ".changes[2].value=\"$(cat /root/scripts/peggy_block_number.txt)\"" > /root/scripts/peggo_params.json

yes $PASSPHRASE | injectived tx gov submit-proposal param-change /root/scripts/peggo_params.json $TX_OPTS
fetch_proposal_id
vote $proposal

rm /root/scripts/peggy_proxy_address.txt
rm /root/scripts/peggy_block_number.txt

injectived tx peggy set-orchestrator-address inj15gnk95hvqrsr343ecqjuv7yf2af9rkdqeax52d inj15gnk95hvqrsr343ecqjuv7yf2af9rkdqeax52d 0x5ae7c0fcbf5014972e71a2841be295f57fbae929 --chain-id=injective-777 --broadcast-mode=block --yes --keyring-backend test --home ~/injective/injective-exchange/var/data/injective-777/n0 --from=val

injectived tx peggy set-orchestrator-address inj1q8nh58utz78ree7e4cnxnvmyt0mrq5ww307jsk inj1q8nh58utz78ree7e4cnxnvmyt0mrq5ww307jsk 0xc1858d219ef878a4e774b3558556bb4b7bd6d286 --chain-id=injective-777 --broadcast-mode=block --yes --keyring-backend test --home ~/injective/injective-exchange/var/data/injective-777/n1 --from=val

injectived tx peggy set-orchestrator-address inj1lrr6tf29yjz4q8ewjgcd4sjj97mh9xy90t56zh inj1lrr6tf29yjz4q8ewjgcd4sjj97mh9xy90t56zh 0x7590dF78DE45a72F02724435d3ca164DA894B5b9 --chain-id=injective-777 --broadcast-mode=block --yes --keyring-backend test --home ~/injective/injective-exchange/var/data/injective-777/n2 --from=val

sudo systemctl start peggo
sudo systemctl start peggo1
sudo systemctl start peggo2