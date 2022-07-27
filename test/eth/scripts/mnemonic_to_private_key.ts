import { Wallet } from 'ethers'

const VAL0_MNEMONIC = "copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"
const VAL1_MNEMONIC = "maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"
const VAL2_MNEMONIC = "keep liar demand upon shed essence tip undo eagle run people strong sense another salute double peasant egg royal hair report winner student diamond"
const USER_MNEMONIC = "pony glide frown crisp unfold lawn cup loan trial govern usual matrix theory wash fresh address pioneer between meadow visa buffalo keep gallery swear"

const mnemonics: {
  [key: string]: string
} = {
  "val0": VAL0_MNEMONIC,
  "val1": VAL1_MNEMONIC,
  "val2": VAL2_MNEMONIC,
  "user": USER_MNEMONIC,
}

for (const key in mnemonics) {
  const mnemonic = mnemonics[key];
  const wallet = Wallet.fromMnemonic(mnemonic);

  console.log(key, mnemonic)
  console.log(wallet.address, wallet.privateKey)
}
