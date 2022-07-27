import { HardhatUserConfig } from "hardhat/config";
import { HardhatNetworkAccountUserConfig, HardhatNetworkUserConfig, NetworkUserConfig } from "hardhat/src/types/config"
import "@nomiclabs/hardhat-etherscan";
import "@nomiclabs/hardhat-ethers";
import { env } from "process";
import "./tasks/getCurrentValset";

// You need to export an object to set up your config
// Go to https://hardhat.org/config/ to learn more

function GetChainId(): number {
  if (env.CHAIN_ID != undefined) {
    return Number(env.CHAIN_ID);
  }
  return 888;
};

const balance = "100000000000000000000000000"

const peggoAccounts: HardhatNetworkAccountUserConfig[] = [
  {
    // val0 0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101
    privateKey: "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305",
    balance: balance,
  },
  {
    // val1 0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17
    privateKey: "0x741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544",
    balance: balance,
  },
  {
    // val2 0x6880D7bfE96D49501141375ED835C24cf70E2bD7
    privateKey: "0x39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608",
    balance: balance,
  },
  {
    // user 0x727AEE334987c52fA7b567b2662BDbb68614e48C
    privateKey: "0x6c212553111b370a8ffdc682954495b7b90a73cedab7106323646a4f2c4e668f",
    balance: balance,
  },
]

const peggoTestNetwork: HardhatNetworkUserConfig = {
  chainId: GetChainId(),
  accounts: peggoAccounts,
}

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.10",
    settings: {
      optimizer: {
        enabled: true
      }
    }
  },
  networks: {
    hardhat: peggoTestNetwork,
    ganache: {
      chainId: GetChainId(),
      url: "http://127.0.0.1:8545",
      accounts: [
        "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305",
        "0x741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544",
        "0x39a4c898dda351d54875d5ebb3e1c451189116faa556c3c04adc860dd1000608",
        "0x6c212553111b370a8ffdc682954495b7b90a73cedab7106323646a4f2c4e668f",
      ]
    },
  },
  etherscan: {
    apiKey: "QCT9NFXK6QMK7H1UX9WU3RXJ972RR2H2G1",
  },
  paths: {
    sources: "./contracts",
    artifacts: "./artifacts"
  }
};

export default config;