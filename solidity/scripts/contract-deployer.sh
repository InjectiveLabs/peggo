#!/bin/bash
yarn ts-node \
contract-deployer.ts \
--cosmos-node="http://localhost:26657" \
--eth-node="http://localhost:8545" \
--eth-privkey="0xd49743deccbccc5dc7baa8e69e5be03298da8688a15dd202e20f15d5e0e9a9fb" \
--contract=artifacts/contracts/Peggy.sol/Peggy.json \
--test-mode=true