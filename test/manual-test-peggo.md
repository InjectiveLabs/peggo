# Manually test peggo with goerli (eth testnet)

## Assumptions

- You have `umeed` in your path
- You have `jq`, `perl` and `curl`
- You have 3 funded wallets in [goerli](https://goerli.etherscan.io/)
  - [Faucet](https://goerli-faucet.mudit.blog/) (sometimes it does not work)
- You should set the following enviroment variables

```shell
$~ UMEE_PEGGO_PATH=${peggo_root_path_in_your_local_enviroment}
$~ ETHRPC=https://goerli-infura.brave.com/f7106c838853428280fa0c585acc9485
$~ MYETH=0xfac5EC50BdfbB803f5cFc9BF0A0C2f52aDE5b6dd
$~ MYETHPK=06e48d48a55cc6843acb2c3c23431480ec42fca02683f4d8d3d471372e5317ee
$~ MYETH2=0x02fa1b44e2EF8436e6f35D5F56607769c658c225
$~ MYETH2PK=4faf826f3d3a5fa60103392446a72dea01145c6158c6dd29f6faab9ec9917a1b
$~ MYETH3=0xd8f468c1B719cc2d50eB1E3A55cFcb60e23758CD
$~ MYETH3PK=11f746395f0dd459eff05d1bc557b81c3f7ebb1338a8cc9d36966d0bb2dcea21
$~ CHAIN_ID=888
```

<!--
```fish
$~ set ETHRPC https://goerli-infura.brave.com/f7106c838853428280fa0c585acc9485
set MYETH 0xfac5EC50BdfbB803f5cFc9BF0A0C2f52aDE5b6dd
set MYETHPK 06e48d48a55cc6843acb2c3c23431480ec42fca02683f4d8d3d471372e5317ee
set MYETH2 0x02fa1b44e2EF8436e6f35D5F56607769c658c225
set MYETH2PK 4faf826f3d3a5fa60103392446a72dea01145c6158c6dd29f6faab9ec9917a1b
set MYETH3 0xd8f468c1B719cc2d50eB1E3A55cFcb60e23758CD
set MYETH3PK 11f746395f0dd459eff05d1bc557b81c3f7ebb1338a8cc9d36966d0bb2dcea21
set CHAIN_ID 888
set BRIDGEADDR 0x61be4C0D3631f684CFfeD7FCf7087FFD4b4b127C
set ERC20_UMEE_TX_HASH 0x412e6f389d5b59dba62951d7f162bb7328d712aa1f49515a0e2e9a29162c3e26
```
-->

## Steps

- To test the peggo bridge manually execute the next steps

### Start umee chain multinode

- Run the `multinode.sh`

```shell
$~ bash $UMEE_PEGGO_PATH/test/cosmos/multinode.sh umeed
```

### Deploy a new Gravity bridge smartcontract

- You can also use a old one already created if the contract didn't change so just
set the `BRIDGEADDR` env variable with the contract address of the bridge smartcontract

```shell
$~ BRIDGEADDR=0x32FDBf26a106d57f99d7B2caBa67eD1a115D8d0c
```

- Or you can deploy a new bridge

```shell
$~ PEGGO_ETH_PK=$MYETHPK peggo bridge deploy-gravity --eth-rpc $ETHRPC
```

__Expected Result__
![image](https://user-images.githubusercontent.com/17556614/160243283-bad93a66-7b09-467c-b1a8-80e2a9336b68.png)

- Set the `BRIDGEADDR` variable

```shell
$~ BRIDGEADDR=0x61be4C0D3631f684CFfeD7FCf7087FFD4b4b127C
```

- Wait until the gravity bridge is confirmed in ethereum (14 blocks)

### Start the orchestrators

- Open 3 new shells with the env variables set and run
the following commands one in each shell

```shell
$~ PEGGO_ETH_PK=$MYETHPK peggo orchestrator $BRIDGEADDR \
  --eth-rpc=$ETHRPC \
  --relay-batches=true \
  --valset-relay-mode="all" \
  --cosmos-chain-id=$CHAIN_ID \
  --cosmos-grpc="tcp://0.0.0.0:9090" \
  --tendermint-rpc="http://0.0.0.0:26657" \
  --cosmos-keyring=test \
  --cosmos-keyring-dir=$UMEE_PEGGO_PATH/test/cosmos/data/$CHAIN_ID/n0/ \
  --cosmos-from=val  --log-level debug --log-format text --profit-multiplier=0
```

```shell
$~ PEGGO_ETH_PK=$MYETH2PK peggo orchestrator $BRIDGEADDR \
  --eth-rpc=$ETHRPC \
  --relay-batches=true \
  --valset-relay-mode="all" \
  --cosmos-chain-id=$CHAIN_ID \
  --cosmos-grpc="tcp://0.0.0.0:9091" \
  --tendermint-rpc="http://0.0.0.0:26667" \
  --cosmos-keyring=test \
  --cosmos-keyring-dir=$UMEE_PEGGO_PATH/test/cosmos/data/$CHAIN_ID/n1/ \
  --cosmos-from=val  --log-level debug --log-format text --profit-multiplier=0
```

```shell
$~ PEGGO_ETH_PK=$MYETH3PK peggo orchestrator $BRIDGEADDR \
  --eth-rpc=$ETHRPC \
  --relay-batches=true \
  --valset-relay-mode="all" \
  --cosmos-chain-id=$CHAIN_ID \
  --cosmos-grpc="tcp://0.0.0.0:9092" \
  --tendermint-rpc="http://0.0.0.0:26677" \
  --cosmos-keyring=test \
  --cosmos-keyring-dir=$UMEE_PEGGO_PATH/test/cosmos/data/$CHAIN_ID/n2/ \
  --cosmos-from=val  --log-level debug --log-format text --profit-multiplier=0
```

### Deploy Umee ERC20

- Deploy a [ERC20](https://eips.ethereum.org/EIPS/eip-20) representation of uumee
token in eth

```shell
$~ PEGGO_ETH_PK=$MYETHPK peggo bridge deploy-erc20 $BRIDGEADDR uumee --eth-rpc $ETHRPC
```

__Expected Result__
![image](https://user-images.githubusercontent.com/17556614/160244050-4317c0c7-1328-4654-ae41-7b1069aa1624.png)

- Set transaction hash in `ERC20_UMEE_TX_HASH`

```shell
$~ ERC20_UMEE_TX_HASH="0xd1940e0501545e2d0935b36719ace1df28f88f333a60026ee43c56f97386cadc"
```

- You can get the contract address of the deployed umee contract

```shell
$~ curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt",
"params":["'$ERC20_UMEE_TX_HASH'"],"id":1}' $ETHRPC | jq -r '.result.logs[0].address'
```

- Or even directly set the `TOKEN_ADDRESS` env variable with the contract
address of the deployed umee contract

```shell
$~ TOKEN_ADDRESS=`curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt",
"params":["'$ERC20_UMEE_TX_HASH'"],"id":1}' $ETHRPC | jq -r '.result.logs[0].address'`
```

<!--
```fish
$~ set TOKEN_ADDRESS (curl -X POST --data '{"jsonrpc":"2.0","method":"eth_getTransactionReceipt", "params":["'$ERC20_UMEE_TX_HASH'"],"id":1}' $ETHRPC | jq -r '.result.logs[0].address')
```
 -->

- Wait until its the new deployed contract is confirmed (14 blocks) by all Peggos

### Send transaction from umee to eth

```shell
$~ umeed tx gravity send-to-eth $MYETH 10000uumee 1uumee \
  --from val \
  --chain-id $CHAIN_ID \
  --keyring-backend=test \
  --home=$UMEE_PEGGO_PATH/test/cosmos/data/$CHAIN_ID/n0/
```

### Send transaction from eth to umee

```shell $~ PEGGO_ETH_PK=$MYETHPK peggo bridge send-to-cosmos \
  $BRIDGEADDR $TOKEN_ADDRESS umee1y6xz2ggfc0pcsmyjlekh0j9pxh6hk87ymc9due 1 \
  --eth-rpc $ETHRPC
```
