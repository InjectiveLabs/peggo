package relayer

func SetSymbolRetriever(coinGecko SymbolRetriever) func(GravityRelayer) {
	return func(s GravityRelayer) { s.SetSymbolRetriever(coinGecko) }
}

func (s *gravityRelayer) SetSymbolRetriever(symbolRetriever SymbolRetriever) {
	s.symbolRetriever = symbolRetriever
}

// SetOracle sets a new oracle to the Gravity Relayer.
func SetOracle(o Oracle) func(GravityRelayer) {
	return func(s GravityRelayer) { s.SetOracle(o) }
}

// SetOracle sets a new oracle to the Gravity Relayer.
func (s *gravityRelayer) SetOracle(o Oracle) {
	s.oracle = o
}
