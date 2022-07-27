import { task } from "hardhat/config";

task("getCurrentValset", "Returns the current valset on the contract")
  .addPositionalParam("gravityAddr", "Gravity bridge contract address")
  .setAction(async (taskArgs, hre) => {
    const gravityAddr = String(taskArgs.gravityAddr)

    const gravityArtifact = await hre.artifacts.readArtifact("Gravity")
    const gravityContract = await hre.ethers.getContractAt(gravityArtifact.abi, gravityAddr);
    console.log("Gravity addr", gravityContract.address);

    const valsetCheckpoint = await gravityContract.state_lastValsetCheckpoint();
    const valsetNonce = await gravityContract.state_lastValsetNonce();
    console.log("valsetCheckpoint:", valsetCheckpoint);
    console.log("valsetNonce:", valsetNonce.toString());
  });