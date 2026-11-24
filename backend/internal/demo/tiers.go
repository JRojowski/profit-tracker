package demo

// TiersConfig is the body of PUT /api/tiers with the sample Tiers of the demo data.
type TiersConfig struct {
	Tiers []Tier `json:"tiers"`
}

// Tier is one sample Tier.
type Tier struct {
	Order          int          `json:"order"`
	Name           string       `json:"name"`
	TargetShare    string       `json:"targetShare"`
	SharedPartName string       `json:"sharedPartName"`
	Instruments    []Assignment `json:"instruments"`
}

// Assignment puts a demo Instrument in a Tier. A nil TargetShare puts it in the Shared Part.
type Assignment struct {
	Ticker      string  `json:"ticker"`
	TargetShare *string `json:"targetShare"`
}

func target(ticker, share string) Assignment { return Assignment{Ticker: ticker, TargetShare: &share} }

func shared(tickers ...string) []Assignment {
	list := make([]Assignment, 0, len(tickers))
	for _, ticker := range tickers {
		list = append(list, Assignment{Ticker: ticker})
	}
	return list
}

// Tiers returns the sample Tiers, Instrument Targets and assignments for the demo Instruments.
// The Target Shares of the Tiers add up to 100. INTC.US is sold in full, and it still has its Tier.
// ZZQX.US has no price.
func Tiers() TiersConfig {
	return TiersConfig{Tiers: []Tier{
		{
			Order: 1, Name: "I. Global Market Core", TargetShare: "45", SharedPartName: "Dividend Stocks",
			Instruments: append([]Assignment{target("VWCE.DE", "60")}, shared("KO.US", "PEP.US")...),
		},
		{
			Order: 2, Name: "II. Big Tech", TargetShare: "35", SharedPartName: "Mega Caps",
			Instruments: append([]Assignment{target("AAPL.US", "30")}, shared("MSFT.US", "GOOGL.US", "AMZN.US", "META.US")...),
		},
		{
			Order: 3, Name: "III. Asymmetric Bets", TargetShare: "15", SharedPartName: "Speculation",
			Instruments: append([]Assignment{target("NVDA.US", "40")}, shared("TSLA.US", "INTC.US", "ZZQX.US")...),
		},
		{Order: 4, Name: "IV. Poland", TargetShare: "5", SharedPartName: "WIG Stocks", Instruments: shared("PKN.PL", "CDR.PL")},
	}}
}
