// Package benchmark holds the list of the Benchmarks and calculates the Benchmark Return of a period.
// See GLOSSARY.md, "Benchmark" and "Benchmark Return". The package does no input or output.
package benchmark

// Benchmark is one market index, commodity or crypto asset that the tool compares with the Time-Weighted Return of
// the user. ADR 0008: an accumulating ETF or ETC represents each market index and each commodity. The price of the
// asset itself represents a crypto asset (ADR 0008, amendment).
type Benchmark struct {
	// ID is the stable identifier, for example "sp500". The store and the API use it.
	ID string
	// Name is the display name, for example "S&P 500".
	Name string
	// Fund is the name of the ETF or ETC that represents the Benchmark. For a crypto asset, it tells that the price
	// of the asset itself represents the Benchmark.
	Fund string
	// Ticker is the price ticker of the fund or of the crypto asset at the price source.
	Ticker string
	// QuoteCurrency is the currency of the price at the price source.
	QuoteCurrency string
}

// List returns the Benchmarks in the order of the API and of the frontend.
// ADR 0008 records the list and the check of each price ticker.
// To add a Benchmark, add one entry here.
func List() []Benchmark {
	return []Benchmark{
		{
			ID: "sp500", Name: "S&P 500", Fund: "iShares Core S&P 500 UCITS ETF USD (Acc)",
			Ticker: "CSPX.L", QuoteCurrency: "USD",
		},
		{
			ID: "wig20tr", Name: "WIG20TR", Fund: "Beta ETF WIG20TR Portfelowy FIZ",
			Ticker: "ETFBW20TR.WA", QuoteCurrency: "PLN",
		},
		{
			ID: "mwig40tr", Name: "mWIG40TR", Fund: "Beta ETF mWIG40TR Portfelowy FIZ",
			Ticker: "ETFBM40TR.WA", QuoteCurrency: "PLN",
		},
		{
			ID: "world", Name: "World equities", Fund: "iShares Core MSCI World UCITS ETF USD (Acc)",
			Ticker: "IWDA.L", QuoteCurrency: "USD",
		},
		{
			ID: "gold", Name: "Gold", Fund: "iShares Physical Gold ETC",
			Ticker: "IGLN.L", QuoteCurrency: "USD",
		},
		{
			ID: "bitcoin", Name: "Bitcoin", Fund: "Bitcoin spot price",
			Ticker: "BTC-USD", QuoteCurrency: "USD",
		},
	}
}
