// gravityId bytes32 the gravity identifier
const gravityId = [100, 101, 102, 97, 117, 108, 116, 103, 114, 97, 118, 105, 116, 121, 105, 100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
// validatorsAddrs are the address used in set orchestrator address
const validatorsAddrs = [0x6880D7bfE96D49501141375ED835C24cf70E2bD7, 0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17, 0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101]; // eth address of validators
const power = [1431655765, 1431655765, 1431655765]; // power of each validator

module.exports = [ gravityId, validatorsAddrs, power];