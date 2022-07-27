import * as hre from "hardhat"

async function main() {
  const blocksToMine = Number(2000);
  for (let index = 0; index < blocksToMine; index++) {
    await hre.network.provider.send("evm_mine");
  }
  // await hre.network.provider.send("evm_mine");
  // await hre.network.provider.send("hardhat_setBalance", [
  //   "0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101",
  //   "0x1000",
  // ]);
  // await network.provider.send("evm_setAutomine", [false]);
  // await network.provider.send("hardhat_mine", []);
  // await hre.network.provider.send("hardhat_mine", ["0x100"]);
  // await hre.network.provider.send("hardhat_mine", ["0x".concat(blocksToMine.toString(16))]);
  // await network.provider.send("hardhat_mine", ["0x".concat(blocksToMine.toString(16)), "0x".concat(timeToMine.toString(16))]);
  // await network.provider.send("evm_setAutomine", [true]);
  console.log("finish mining", blocksToMine.toString())
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });