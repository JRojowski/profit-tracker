// Package allocation calculates the Allocation of one Account or of all Accounts. See GLOSSARY.md, "Allocation".
// It is a pure calculation with no I/O. It uses the values that the Ledger calculator gives and does not change them.
package allocation

import (
	"cmp"
	"slices"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// MaxHoldings is the number of the largest Holdings that are parts. The other Holdings go into Result.Other.
const MaxHoldings = 8

// Account is the input of one Account: its currency and the result of the Ledger calculator.
type Account struct {
	Currency string
	State    ledger.State
}

// Rates are the Exchange Rate lookups.
type Rates struct {
	// Latest returns the latest rate of the currency: the PLN amount for 1 unit. It returns false if the tool has
	// no rate of the currency. ADR 0004, rule 2.
	Latest func(currency string) (decimal.Decimal, bool)
}

// Part is one Holding of the Allocation, or the cash. Its value is in the currency of the Allocation.
type Part struct {
	Ticker string
	Name   string
	Value  decimal.Decimal
	// Share is Value × 100 / Result.Total.
	Share decimal.Decimal
}

// Other holds the Holdings after the MaxHoldings largest.
type Other struct {
	// Value is the sum of the values of Holdings. Share is Value × 100 / Result.Total.
	Value, Share decimal.Decimal
	// Holdings are in the order of the Allocation: the largest value first.
	Holdings []Part
}

// CurrencyPart is the value of one Quote Currency. The cash counts in the currency of its Account.
type CurrencyPart struct {
	Currency string
	// Value is in the currency of the Allocation. Share is Value × 100 / Result.Total.
	Value, Share decimal.Decimal
}

// Result is the Allocation.
type Result struct {
	// Available tells if each needed Exchange Rate is known. If it is false, MissingRates tell which rates are
	// missing, and the total and the parts are empty.
	Available bool
	// MissingRates are the Account currencies with no latest Exchange Rate, in currency order.
	MissingRates []string
	// Total is the sum of all parts.
	Total decimal.Decimal
	// Holdings are the MaxHoldings largest Holdings, the largest value first. Equal values sort by ticker.
	Holdings []Part
	// Cash is the cash part. It is nil when the cash is zero or less.
	Cash *Part
	// Other is nil when there are MaxHoldings Holdings or fewer.
	Other *Other
	// ByQuoteCurrency has one part per currency, the largest value first. Equal values sort by currency.
	ByQuoteCurrency []CurrencyPart
	// UnpricedHoldings is the number of Holdings with no known value. They are not parts.
	UnpricedHoldings int
}

// CashName is the name of the cash part.
const CashName = "Cash"

// Calculate returns the Allocation of the Accounts in the currency.
//
// The parts are the cash and the open Holdings with a known value. One Instrument on two or more Accounts is
// one part, with the sum of its values. Each value of an Account in another currency than the Allocation
// currency is converted at the latest Exchange Rate (ADR 0004, rule 2). The currency must be PLN or the
// currency of each Account.
//
// quoteCurrencies are the Quote Currencies of the Instruments by broker ticker.
func Calculate(accounts []Account, currency string, quoteCurrencies map[string]string, rates Rates) Result {
	result := Result{Available: true, MissingRates: []string{}}
	for _, acc := range accounts {
		for _, h := range acc.State.Holdings {
			if !h.HasValue() {
				result.UnpricedHoldings++
			}
		}
	}

	rateOf := make(map[string]decimal.Decimal, len(accounts))
	for _, acc := range accounts {
		if acc.Currency == currency {
			rateOf[acc.Currency] = decimal.NewFromInt(1)
			continue
		}
		rate, ok := rates.Latest(acc.Currency)
		if !ok {
			result.MissingRates = append(result.MissingRates, acc.Currency)
			continue
		}
		rateOf[acc.Currency] = rate
	}
	if len(result.MissingRates) > 0 {
		slices.Sort(result.MissingRates)
		result.MissingRates = slices.Compact(result.MissingRates)
		result.Available = false
		result.Holdings = []Part{}
		result.ByQuoteCurrency = []CurrencyPart{}
		return result
	}

	holdings := map[string]*Part{}
	byCurrency := map[string]decimal.Decimal{}
	cash := decimal.Zero
	for _, acc := range accounts {
		rate := rateOf[acc.Currency]
		for _, h := range acc.State.Holdings {
			if !h.HasValue() {
				continue
			}
			value := h.Value.Mul(rate)
			p, ok := holdings[h.Instrument.Ticker]
			if !ok {
				p = &Part{Ticker: h.Instrument.Ticker, Name: h.Instrument.Name}
				holdings[h.Instrument.Ticker] = p
			}
			p.Value = p.Value.Add(value)
			qc := quoteCurrencies[h.Instrument.Ticker]
			byCurrency[qc] = byCurrency[qc].Add(value)
		}
		cash = cash.Add(acc.State.Cash.Mul(rate))
	}

	parts := make([]Part, 0, len(holdings))
	for _, p := range holdings {
		parts = append(parts, *p)
		result.Total = result.Total.Add(p.Value)
	}
	if cash.IsPositive() {
		result.Cash = &Part{Name: CashName, Value: cash}
		result.Total = result.Total.Add(cash)
		// Rule 7: the cash counts in the currency of its Account.
		for _, acc := range accounts {
			if !acc.State.Cash.IsZero() {
				byCurrency[acc.Currency] = byCurrency[acc.Currency].Add(acc.State.Cash.Mul(rateOf[acc.Currency]))
			}
		}
	}

	slices.SortFunc(parts, func(a, b Part) int {
		return cmp.Or(b.Value.Cmp(a.Value), cmp.Compare(a.Ticker, b.Ticker))
	})
	for i := range parts {
		parts[i].Share = share(parts[i].Value, result.Total)
	}
	if result.Cash != nil {
		result.Cash.Share = share(result.Cash.Value, result.Total)
	}
	result.Holdings = parts[:min(len(parts), MaxHoldings)]
	if len(parts) > MaxHoldings {
		other := &Other{Holdings: parts[MaxHoldings:]}
		for _, p := range other.Holdings {
			other.Value = other.Value.Add(p.Value)
		}
		other.Share = share(other.Value, result.Total)
		result.Other = other
	}

	result.ByQuoteCurrency = make([]CurrencyPart, 0, len(byCurrency))
	for c, value := range byCurrency {
		result.ByQuoteCurrency = append(result.ByQuoteCurrency, CurrencyPart{Currency: c, Value: value, Share: share(value, result.Total)})
	}
	slices.SortFunc(result.ByQuoteCurrency, func(a, b CurrencyPart) int {
		return cmp.Or(b.Value.Cmp(a.Value), cmp.Compare(a.Currency, b.Currency))
	})
	return result
}

// share returns value × 100 / total. The multiplication comes first, so an exact result stays exact.
// The division keeps decimal.DivisionPrecision (16) decimal places. It returns zero if total is zero.
func share(value, total decimal.Decimal) decimal.Decimal {
	if total.IsZero() {
		return decimal.Zero
	}
	return value.Mul(decimal.NewFromInt(100)).Div(total)
}
