package benchmark

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// PLN is the currency of the Exchange Rates. Its rate is 1.
const PLN = "PLN"

// precision is the number of decimal places of the ratio of the two values of a Benchmark.
// It is the same as the precision of the Time-Weighted Return.
const precision = 20

// hundred converts a fraction to a percentage.
var hundred = decimal.NewFromInt(100)

// Close is one daily close of a Benchmark in its Quote Currency.
type Close struct {
	// Date is the price date at midnight UTC.
	Date  time.Time
	Price decimal.Decimal
}

// Rate is one Exchange Rate: the PLN for one unit of the currency on a day.
type Rate struct {
	// Date is the table date at midnight UTC.
	Date time.Time
	Mid  decimal.Decimal
}

// Status tells if a Benchmark has a Benchmark Return in the period.
type Status string

const (
	// StatusAvailable tells that the Benchmark has a Benchmark Return.
	StatusAvailable Status = "available"
	// StatusNoFullPeriod tells that the Benchmark has no close or no Exchange Rate on or before the base day,
	// or on or before the last day. The tool does not shorten the period for one Benchmark.
	StatusNoFullPeriod Status = "noFullPeriod"
	// StatusSourceFailure tells that the last refresh of the closes of the Benchmark failed. It wins over the
	// other statuses, because the stored closes can be incomplete.
	StatusSourceFailure Status = "sourceFailure"
)

// Input is the data of one comparison.
type Input struct {
	// Currency is the currency of the comparison: the Account currency, or PLN for all Accounts.
	Currency string
	// BaseDay and LastDay are those of the Time-Weighted Return of the period, at midnight UTC.
	BaseDay, LastDay time.Time
	// UserReturn is the Time-Weighted Return of the period, as a percentage.
	UserReturn decimal.Decimal
	// Closes are the closes of each Benchmark in its Quote Currency, in date order, by Benchmark identifier.
	Closes map[string][]Close
	// Rates are the Exchange Rates of each currency, in date order, by currency.
	Rates map[string][]Rate
	// Failed has the identifier of each Benchmark whose last refresh of the closes failed.
	Failed map[string]bool
}

// Result is the comparison of one Benchmark.
type Result struct {
	Benchmark
	Status Status
	// Return is the Benchmark Return as a percentage with at most 8 decimal places. It is zero unless the
	// status is StatusAvailable.
	Return decimal.Decimal
	// Difference is UserReturn minus Return, in percentage points. It is zero unless the status is StatusAvailable.
	Difference decimal.Decimal
}

// Compare returns the comparison of each Benchmark of List, in the order of List.
//
// The value of a Benchmark on a day, in the currency of the comparison, is the close × the rate of the Quote
// Currency / the rate of the comparison currency. The rate of PLN is 1. A day with no close uses the last earlier
// close. A day with no Exchange Rate uses the last earlier rate, as the Value History does. A rate of 0 or less is
// not valid. A Benchmark whose last refresh failed has the status StatusSourceFailure and no return.
// The Benchmark Return is the value on the last day / the value on the base day − 1.
func Compare(in Input) []Result {
	list := List()
	results := make([]Result, 0, len(list))
	for _, b := range list {
		r := Result{Benchmark: b, Status: StatusNoFullPeriod}
		if in.Failed[b.ID] {
			r.Status = StatusSourceFailure
			results = append(results, r)
			continue
		}
		if ret, ok := benchmarkReturn(in, b); ok {
			r.Status, r.Return, r.Difference = StatusAvailable, ret, in.UserReturn.Sub(ret)
		}
		results = append(results, r)
	}
	return results
}

// benchmarkReturn returns the Benchmark Return of b in the period, as a percentage. It returns false if a close
// or an Exchange Rate is missing on the base day or on the last day.
func benchmarkReturn(in Input, b Benchmark) (decimal.Decimal, bool) {
	last, ok := in.value(b, in.LastDay)
	if !ok {
		return decimal.Zero, false
	}
	base, ok := in.value(b, in.BaseDay)
	if !ok {
		return decimal.Zero, false
	}
	// One division keeps the rounding to the end.
	ratio := last.numerator.Mul(base.denominator).DivRound(base.numerator.Mul(last.denominator), precision)
	return ratio.Sub(decimal.NewFromInt(1)).Mul(hundred).Round(8), true
}

// fraction is the value of a Benchmark on a day as numerator / denominator, so that the return needs one division.
type fraction struct {
	numerator, denominator decimal.Decimal
}

// value returns the value of b on the day in the currency of the comparison.
func (in Input) value(b Benchmark, day time.Time) (fraction, bool) {
	c, ok := onOrBefore(in.Closes[b.ID], day, func(c Close) time.Time { return c.Date })
	if !ok || !c.Price.IsPositive() {
		return fraction{}, false
	}
	if b.QuoteCurrency == in.Currency {
		return fraction{numerator: c.Price, denominator: decimal.NewFromInt(1)}, true
	}
	quote, ok := in.rate(b.QuoteCurrency, day)
	if !ok {
		return fraction{}, false
	}
	comparison, ok := in.rate(in.Currency, day)
	if !ok {
		return fraction{}, false
	}
	return fraction{numerator: c.Price.Mul(quote), denominator: comparison}, true
}

// rate returns the Exchange Rate of the currency on the day: 1 for PLN, else the last valid rate on or before the day.
func (in Input) rate(currency string, day time.Time) (decimal.Decimal, bool) {
	if currency == PLN {
		return decimal.NewFromInt(1), true
	}
	r, ok := onOrBefore(in.Rates[currency], day, func(r Rate) time.Time { return r.Date })
	if !ok || !r.Mid.IsPositive() {
		return decimal.Zero, false
	}
	return r.Mid, true
}

// onOrBefore returns the last item with a date on the day or before it. items are in date order.
// It returns false if no item has such a date.
func onOrBefore[T any](items []T, day time.Time, date func(T) time.Time) (T, bool) {
	// i is the index of the first item after the day.
	i, found := slices.BinarySearchFunc(items, day, func(item T, d time.Time) int { return date(item).Compare(d) })
	if found {
		i++
	}
	if i == 0 {
		var zero T
		return zero, false
	}
	return items[i-1], true
}
