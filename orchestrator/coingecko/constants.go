package coingecko

// Everytime a new ERC20 is deployed in the Gravity Bridge contract, we need to
// add it here and make a new release of Peggo.
var bridgeTokensCoinIDs = map[string]string{
	"0x0000000000000000000000000000000000000000": "eth",
}
