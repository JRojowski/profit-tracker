// Package pricecheck finds the Price Warnings of an Instrument: the signs that its price ticker or its
// Quote Currency can be wrong. See GLOSSARY.md, "Price Warning". It is a pure calculation with no I/O.
// The tool checks the Instruments on each request and stores no result.
package pricecheck

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// Kind is the kind of a Price Warning. The order of the values is the order of the warnings of one Instrument.
type Kind int

const (
	// NoData tells that the price source has no data for the price ticker, and the tool stores no close.
	NoData Kind = iota
	// DifferentCurrency tells that the price source gives a currency that is not the Quote Currency.
	DifferentCurrency
	// PriceMismatch tells that the closes of the price source do not agree with the XTB prices.
	PriceMismatch
)

// The limits of the price mismatch, in percent. A difference must be larger than the limit.
var (
	// medianLimit finds a constant offset, for example a EUR line in place of a USD line.
	medianLimit = decimal.NewFromInt(5)
	// singleLimit finds a large error in an Instrument with few Fills and sales.
	singleLimit = decimal.NewFromInt(30)
)

// Input is the data of one Instrument that the price check uses.
type Input struct {
	// QuoteCurrency is the Quote Currency of the Instrument. An empty value gives no warnings,
	// because the Holdings already tell the user to set it.
	QuoteCurrency string
	// SourceCurrency is the currency that the price source gave in its last answer with a price,
	// or empty if it is not known.
	SourceCurrency string
	// NoData tells that the last answer of the price source was "no data".
	NoData bool
	// Closes are the stored closes of the Instrument, in any currency.
	Closes []Close
	// Trades are the XTB prices of each Fill and each sale of the Instrument on any Account.
	Trades []Trade
	// Confirmed tells that the user confirmed that the price ticker and the Quote Currency are correct.
	Confirmed bool
}

// Close is one stored close of the Instrument.
type Close struct {
	// Date is the price date at midnight UTC.
	Date time.Time
	// Price is the close in Currency.
	Price    decimal.Decimal
	Currency string
}

// Trade is the XTB price of one unit in one Fill or one sale.
type Trade struct {
	// Time is the time of the Cash Operation. The check uses its day in UTC.
	Time time.Time
	// Price is the XTB price of one unit.
	Price decimal.Decimal
}

// Warning is one Price Warning.
type Warning struct {
	Kind Kind
	// SourceCurrency is the currency of the price source. It is set for DifferentCurrency.
	SourceCurrency string
	// MedianDifference and MaxDifference are the median and the largest difference between a close and
	// the XTB price, in percent, with no rounding. They are set for PriceMismatch.
	MedianDifference, MaxDifference decimal.Decimal
}

// Check returns the Price Warnings of the Instrument in the order of Kind.
//
//   - NoData: the last answer was "no data", and the tool stores no close. The stored closes of a delisted
//     Instrument are the only copy of its history, so they give no warning.
//   - DifferentCurrency: the source currency is known and is not the Quote Currency.
//   - PriceMismatch: for each Fill and each sale with a close in the Quote Currency on its day,
//     the difference is |close − XTB price| / XTB price. The median difference is more than 5%,
//     or one difference is more than 30%. The confirmation of the user removes only this kind.
func Check(in Input) []Warning {
	if in.QuoteCurrency == "" {
		return nil
	}
	var warnings []Warning
	if in.NoData && len(in.Closes) == 0 {
		warnings = append(warnings, Warning{Kind: NoData})
	}
	if in.SourceCurrency != "" && in.SourceCurrency != in.QuoteCurrency {
		warnings = append(warnings, Warning{Kind: DifferentCurrency, SourceCurrency: in.SourceCurrency})
	}
	if in.Confirmed {
		return warnings
	}
	diffs := differences(in)
	if len(diffs) == 0 {
		return warnings
	}
	median, largest := medianOf(diffs), slices.MaxFunc(diffs, decimal.Decimal.Cmp)
	if median.GreaterThan(medianLimit) || largest.GreaterThan(singleLimit) {
		warnings = append(warnings, Warning{Kind: PriceMismatch, MedianDifference: median, MaxDifference: largest})
	}
	return warnings
}

// differences returns the difference in percent of each trade that has a close in the Quote Currency on its day.
func differences(in Input) []decimal.Decimal {
	closes := make(map[time.Time]decimal.Decimal, len(in.Closes))
	for _, c := range in.Closes {
		if c.Currency == in.QuoteCurrency {
			closes[c.Date] = c.Price
		}
	}
	hundred := decimal.NewFromInt(100)
	var diffs []decimal.Decimal
	for _, t := range in.Trades {
		c, ok := closes[day(t.Time)]
		if !ok || !t.Price.IsPositive() {
			continue
		}
		diffs = append(diffs, c.Sub(t.Price).Abs().Mul(hundred).Div(t.Price))
	}
	return diffs
}

// medianOf returns the median of the values. values must not be empty.
func medianOf(values []decimal.Decimal) decimal.Decimal {
	sorted := slices.SortedFunc(slices.Values(values), decimal.Decimal.Cmp)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return sorted[mid-1].Add(sorted[mid]).Div(decimal.NewFromInt(2))
}

// day returns the day of t in UTC, at midnight UTC.
func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
