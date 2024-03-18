## Peggo Testsuite

[IN PROGRESS]

Welcome to the PegGo testing framework. The goal of this suite is aligned with the overall project goal - to move stuff onto common ground and iterate faster.
By using the same lang for module, orchestrator and test we can achieve the full test coverage of all logical branches.

To set up the testing env, just run `test/run.sh` The script initializes 3 Injective validator nodes and 1 geth instance to simulate Injective and Ethereum networks respectively. After the networks are started 3 Peggo orchestrators are run for each of the validator nodes.
For simplicity, the script runs with hardcoded values for most of the configurations. Tweaking parameters is not yet supported.

The script `test/run.sh` can be run multiple times. On each run it removes all previously written files with new ones.
Before running the script again, make sure you've killed all the injectived/geth processes (e.g. `killall injectived`).

## Prerequisites

- `injective-core`: run `make install` on the `fix/peggy-contract-redeployment` branch
- `geth`: version 1.13.10-stable
- `etherman`: from https://github.com/InjectiveLabs/etherman/
- `jq`
- `tmux`
- `perl`
- `sed`


### Injective -> Ethereum flow

- To send some `inj` tokens to Ethereum, run `test/send_to_eth.sh`. 
- To send a cosmos native token other than `inj`, the `test/deploy_token.sh` deploys a new "WAT" token on Ethereum. Afterward, tweak the `test/send_to_eth.sh` to send the new token (already premined during `test/run.sh`)   

### Ethereum -> Injective flow

- To send some `inj` tokens to Ethereum, run `test/send_to_inj.sh`
- Other tokens: TODO

### Cosmos Accounts

The script imports 3 validator accounts and 1 user account, specified by mnemonics in the script itself. Each validator account accessible as `val` on the corresponding nodes, and user account is shared across all three nodes as `user`.

## Contributing

Patches and suggestions are welcome. We're looking for better coverage and maybe some isolated benchmarks.

🍻