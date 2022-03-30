package coingecko

import ethcmn "github.com/ethereum/go-ethereum/common"

// Every time a new ERC20 is deployed in the Gravity Bridge contract, to avoid
// getting the symbol from CoinGecko, we should add it here and make a new
// release of Peggo.
var bridgeTokensCoinSymbols = map[ethcmn.Address]string{
	ethcmn.HexToAddress("0xc0a4Df35568F116C370E6a6A6022Ceb908eedDaC"): "UMEE",
	ethcmn.HexToAddress("0x3339add5c1c1647B554D96c379a430273f5f59f2"): "OSMO",
	ethcmn.HexToAddress("0xEa5A82B35244d9e5E48781F00b11B14E627D2951"): "ATOM",
	ethcmn.HexToAddress("0xbdCbe7fe6Fd2E4C163205ca9D192cF3D3f70CBa5"): "ION",
	ethcmn.HexToAddress("0x7C1Cab5d766091dd65B1FE58400c82D071D9700E"): "JUNO",
	ethcmn.HexToAddress("0x3FE814741C4d0C84044150927a8e22EC5919014E"): "LUNA",
	ethcmn.HexToAddress("0x6B59D96cB4bBe7A34dA325583C5A91d8370FE63E"): "UST",
	ethcmn.HexToAddress("0x351CCfaC7f6f3836d062AbC3525AB0A48ca2e8f3"): "AKT",
	ethcmn.HexToAddress("0x305C6fCe11b8dB61a8355aFCDb2F857472C5FF8a"): "EROWAN",
}
