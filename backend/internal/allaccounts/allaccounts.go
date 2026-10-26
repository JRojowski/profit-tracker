// Package allaccounts calculates the view of all Accounts together in PLN, by ADR 0004.
// It is a pure calculation with no I/O.
package allaccounts

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

// Account is the input of one Account: its currency and the result of the Ledger calculator.
type Account struct {
	ID       int64
	Currency string
	State    ledger.State
}

// Rates are the Exchange Rate lookups. Each rate is the PLN amount for 1 unit of the currency.
type Rates struct {
	// On returns the rate of the currency on the day of t in Polish time, or false if the rate is missing.
	On func(currency string, t time.Time) (decimal.Decimal, bool)
	// Latest returns the latest rate of the currency, or false if the tool has no rate of the currency.
	Latest func(currency string) (decimal.Decimal, bool)
}

// Amounts are the values of an Account or of all Accounts, in PLN.
type Amounts struct {
	Value, Cash, NetDeposits, TotalResult                             decimal.Decimal
	RealizedProfit, UnrealizedProfit, NetDividends, NetInterest, Fees decimal.Decimal
	// CurrencyResult is the part of TotalResult that comes from the change of the Exchange Rate between the day
	// of each Net Deposits flow and today. See GLOSSARY.md, "Currency Result".
	CurrencyResult decimal.Decimal
}

// add returns the sum of the amounts.
func (a Amounts) add(b Amounts) Amounts {
	return Amounts{
		Value: a.Value.Add(b.Value), Cash: a.Cash.Add(b.Cash), NetDeposits: a.NetDeposits.Add(b.NetDeposits),
		TotalResult: a.TotalResult.Add(b.TotalResult), RealizedProfit: a.RealizedProfit.Add(b.RealizedProfit),
		UnrealizedProfit: a.UnrealizedProfit.Add(b.UnrealizedProfit), NetDividends: a.NetDividends.Add(b.NetDividends),
		NetInterest: a.NetInterest.Add(b.NetInterest), Fees: a.Fees.Add(b.Fees),
		CurrencyResult: a.CurrencyResult.Add(b.CurrencyResult),
	}
}

// Row is one Account in PLN.
type Row struct {
	AccountID int64
	// Available tells if the row has its amounts. A PLN row is always available.
	Available bool
	// ExchangeRate is the latest rate of the Account currency. HasExchangeRate is false for a PLN Account
	// and for a currency with no rate. A row that is not in PLN and has no rate is not available.
	ExchangeRate    decimal.Decimal
	HasExchangeRate bool
	// Amounts are zero if the row is not available.
	Amounts Amounts
	// MissingFlows are the times of the Net Deposits flows with no rate on their day, in the order of the Ledger.
	// The row is not available if there is one.
	MissingFlows []time.Time
	// Flows are the Net Deposits flows in PLN, each at the rate of its day. They are empty if the row is not available.
	Flows []returns.Flow
	// AllHoldingsHaveValue tells if each Holding of the Account has a value. See ledger.State.AllHoldingsHaveValue.
	AllHoldingsHaveValue bool
}

// Result is the view of all Accounts.
type Result struct {
	// Available tells if each row is available. If it is false, Totals are zero.
	Available bool
	// Totals are the sum of the rows.
	Totals Amounts
	// Rows are in the order of the input.
	Rows []Row
	// SimpleReturn is Totals.TotalResult × 100 / Totals.NetDeposits.
	SimpleReturn returns.Rate
	// AnnualReturn is the yearly rate that makes the flows of all Accounts in PLN grow to Totals.Value today.
	// ADR 0004, rule 7.
	AnnualReturn returns.Rate
}

// Calculate returns the view of all Accounts in PLN, by ADR 0004.
//
// A PLN row has the amounts of its Account, and its Currency Result is 0.
// A row in another currency multiplies the value, the cash and each part of the Total Result by the latest rate.
// It multiplies each Net Deposits flow by the rate of the day of that flow.
// The Currency Result is the sum, for each flow, of the flow amount × (the latest rate − the rate of the flow day).
// The Total Result is the sum of the parts plus the Currency Result.
// When each Holding has a value, this Total Result is equal to the value minus the Net Deposits (ADR 0004, rule 4).
// When a Holding has no value, the parts count it at its cost, as on the dashboard of the Account.
//
// The totals are not available if a row is not available. Then the returns are not available too.
// The Annual Return counts days in the location of today. Give today in Polish time.
func Calculate(accounts []Account, rates Rates, today time.Time) Result {
	result := Result{Available: true, Rows: make([]Row, 0, len(accounts))}
	for _, acc := range accounts {
		row := convert(acc, rates)
		result.Rows = append(result.Rows, row)
		if !row.Available {
			result.Available = false
			continue
		}
		result.Totals = result.Totals.add(row.Amounts)
	}
	if !result.Available {
		result.Totals = Amounts{}
		missing := returns.Rate{Reason: returns.ReasonMissingExchangeRate}
		result.SimpleReturn, result.AnnualReturn = missing, missing
		return result
	}

	var flows []returns.Flow
	allHoldingsHaveValue := true
	for _, row := range result.Rows {
		flows = append(flows, row.Flows...)
		allHoldingsHaveValue = allHoldingsHaveValue && row.AllHoldingsHaveValue
	}
	result.SimpleReturn = returns.Simple(result.Totals.TotalResult, result.Totals.NetDeposits)
	result.AnnualReturn = returns.Annual(flows, result.Totals.Value, allHoldingsHaveValue, today)
	return result
}

// convert returns the row of the Account in PLN.
func convert(acc Account, rates Rates) Row {
	s := acc.State
	row := Row{AccountID: acc.ID, AllHoldingsHaveValue: s.AllHoldingsHaveValue()}
	if acc.Currency == ledger.PLN {
		row.Available, row.Flows = true, s.Flows
		row.Amounts = Amounts{
			Value: s.Value, Cash: s.Cash, NetDeposits: s.NetDeposits, TotalResult: s.TotalResult,
			RealizedProfit: s.RealizedProfit, UnrealizedProfit: s.UnrealizedProfit, NetDividends: s.NetDividends,
			NetInterest: s.NetInterest, Fees: s.Fees, CurrencyResult: decimal.Zero,
		}
		return row
	}

	latest, ok := rates.Latest(acc.Currency)
	if !ok {
		return row
	}
	row.ExchangeRate, row.HasExchangeRate = latest, true

	netDeposits, currencyResult := decimal.Zero, decimal.Zero
	flows := make([]returns.Flow, 0, len(s.Flows))
	for _, f := range s.Flows {
		rate, ok := rates.On(acc.Currency, f.Time)
		if !ok {
			row.MissingFlows = append(row.MissingFlows, f.Time)
			continue
		}
		// ADR 0004, rules 3 and 6.
		inPLN := f.Amount.Mul(rate)
		flows = append(flows, returns.Flow{Time: f.Time, Amount: inPLN})
		netDeposits = netDeposits.Add(inPLN)
		currencyResult = currencyResult.Add(f.Amount.Mul(latest.Sub(rate)))
	}
	if len(row.MissingFlows) > 0 {
		return row
	}
	row.Flows = flows

	// ADR 0004, rules 2 and 5.
	a := Amounts{
		Value: s.Value.Mul(latest), Cash: s.Cash.Mul(latest), NetDeposits: netDeposits,
		RealizedProfit: s.RealizedProfit.Mul(latest), UnrealizedProfit: s.UnrealizedProfit.Mul(latest),
		NetDividends: s.NetDividends.Mul(latest), NetInterest: s.NetInterest.Mul(latest), Fees: s.Fees.Mul(latest),
		CurrencyResult: currencyResult,
	}
	a.TotalResult = a.RealizedProfit.Add(a.UnrealizedProfit).Add(a.NetDividends).Add(a.NetInterest).Add(a.Fees).Add(currencyResult)
	row.Available, row.Amounts = true, a
	return row
}
