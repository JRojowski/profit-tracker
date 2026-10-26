package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/allaccounts"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// plnAmountsResponse are the amounts in PLN of the totals or of one row, as decimal text.
// Each amount is nil if it is not available.
type plnAmountsResponse struct {
	Value            *string `json:"value"`
	Cash             *string `json:"cash"`
	NetDeposits      *string `json:"netDeposits"`
	TotalResult      *string `json:"totalResult"`
	RealizedProfit   *string `json:"realizedProfit"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	NetDividends     *string `json:"netDividends"`
	NetInterest      *string `json:"netInterest"`
	Fees             *string `json:"fees"`
	CurrencyResult   *string `json:"currencyResult"`
}

// allAccountsResponse is the body of GET /api/all-accounts/summary. ADR 0004.
type allAccountsResponse struct {
	// Available tells if the totals are available. If it is false, Warnings tell why.
	Available bool `json:"available"`
	plnAmountsResponse
	// PriceDate is the oldest price date of the Holdings with a value, as YYYY-MM-DD, or nil.
	PriceDate *string `json:"priceDate"`
	// ExchangeRateDate is the oldest table date of the latest Exchange Rates that the view uses, as YYYY-MM-DD, or nil.
	ExchangeRateDate *string `json:"exchangeRateDate"`
	// StalePrices tells that the summary of an Account tells stale prices.
	StalePrices bool `json:"stalePrices"`
	// StaleRates tells that the summary of an Account tells stale rates, or that the latest rate of an Account
	// currency that is not PLN is missing or the tool got it before today in Polish time.
	StaleRates bool `json:"staleRates"`
	// UnpricedHoldings is the number of Holdings with no value in all Accounts.
	UnpricedHoldings int `json:"unpricedHoldings"`
	// Warnings are sentences for the user about missing rates, in currency order.
	// It is empty if there is nothing to say.
	Warnings []string                 `json:"warnings"`
	Accounts []allAccountsRowResponse `json:"accounts"`
	// SimpleReturn and AnnualReturn have the formats of the Account summary. They use the amounts and the
	// Net Deposits flows in PLN (ADR 0004, rule 7). A note tells why a return is nil.
	SimpleReturn     *string `json:"simpleReturn"`
	SimpleReturnNote *string `json:"simpleReturnNote"`
	AnnualReturn     *string `json:"annualReturn"`
	AnnualReturnNote *string `json:"annualReturnNote"`
}

// allAccountsRowResponse is one Account in PLN, in the order of GET /api/accounts.
type allAccountsRowResponse struct {
	AccountID int64  `json:"accountId"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Currency  string `json:"currency"`
	// Available tells if the row has its amounts. A PLN row is always available. If it is false, Note tells why.
	Available bool `json:"available"`
	// ExchangeRate is the latest Exchange Rate of the Account currency. It is nil for a PLN Account.
	ExchangeRate *string `json:"exchangeRate"`
	plnAmountsResponse
	UnpricedHoldings int     `json:"unpricedHoldings"`
	Note             *string `json:"note"`
	// AnnualReturn has the format of the Account summary. It uses the Net Deposits flows of the Account in PLN
	// (ADR 0004, rule 7). AnnualReturnNote tells why it is nil.
	AnnualReturn     *string `json:"annualReturn"`
	AnnualReturnNote *string `json:"annualReturnNote"`
}

// allAccountsSummary returns all Accounts together in PLN, with one row per Account.
func (a *api) allAccountsSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	views := make([]accountView, 0, len(accounts))
	for _, acc := range accounts {
		view, err := a.readAccountView(ctx, acc)
		if err != nil {
			a.writeInternalError(w, r, err)
			return
		}
		views = append(views, view)
	}
	series, err := a.rates.Series(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	latest, err := a.rates.LatestRates(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	input := make([]allaccounts.Account, 0, len(views))
	for _, v := range views {
		input = append(input, allaccounts.Account{ID: v.account.ID, Currency: v.account.Currency, State: v.state})
	}
	today := exchange.StartOfDayInWarsaw(a.now())
	result := allaccounts.Calculate(input, allaccounts.Rates{
		On: func(currency string, t time.Time) (decimal.Decimal, bool) {
			rate, ok := series[currency].On(t)
			return rate.Mid, ok
		},
		Latest: func(currency string) (decimal.Decimal, bool) {
			rate, ok := latest[currency]
			return rate.Mid, ok
		},
	}, today)

	resp := allAccountsResponse{
		Available:        result.Available,
		PriceDate:        allPriceDate(views),
		ExchangeRateDate: allExchangeRateDate(views, latest),
		StalePrices:      slices.ContainsFunc(views, func(v accountView) bool { return v.stalePrices(a.today()) }),
		StaleRates:       allStaleRates(views, latest, today),
		Warnings:         rateWarnings(views, result.Rows, latest),
		Accounts:         make([]allAccountsRowResponse, 0, len(views)),
		SimpleReturn:     percent(result.SimpleReturn),
		SimpleReturnNote: returnNote(result.SimpleReturn),
		AnnualReturn:     percent(result.AnnualReturn),
		AnnualReturnNote: returnNote(result.AnnualReturn),
	}
	if result.Available {
		resp.plnAmountsResponse = toPLNAmounts(result.Totals)
	}
	for i, v := range views {
		row := result.Rows[i]
		resp.UnpricedHoldings += v.unpricedHoldings()
		dto := allAccountsRowResponse{
			AccountID: v.account.ID, Name: v.account.Name, Type: v.account.Type, Currency: v.account.Currency,
			Available: row.Available, UnpricedHoldings: v.unpricedHoldings(),
			AnnualReturn: percent(row.AnnualReturn), AnnualReturnNote: returnNote(row.AnnualReturn),
		}
		if row.HasExchangeRate {
			dto.ExchangeRate = new(row.ExchangeRate.String())
		}
		if row.Available {
			dto.plnAmountsResponse = toPLNAmounts(row.Amounts)
		} else {
			dto.Note = new(rowNote(v.account.Currency, row))
		}
		resp.Accounts = append(resp.Accounts, dto)
	}
	writeJSON(w, http.StatusOK, resp)
}

// toPLNAmounts returns the amounts as decimal text.
func toPLNAmounts(a allaccounts.Amounts) plnAmountsResponse {
	return plnAmountsResponse{
		Value: new(formatAmount(a.Value)), Cash: new(formatAmount(a.Cash)), NetDeposits: new(formatAmount(a.NetDeposits)),
		TotalResult: new(formatAmount(a.TotalResult)), RealizedProfit: new(formatAmount(a.RealizedProfit)),
		UnrealizedProfit: new(formatAmount(a.UnrealizedProfit)), NetDividends: new(formatAmount(a.NetDividends)),
		NetInterest: new(formatAmount(a.NetInterest)), Fees: new(formatAmount(a.Fees)),
		CurrencyResult: new(formatAmount(a.CurrencyResult)),
	}
}

// rowNote returns the sentence that tells why the row is not available.
func rowNote(currency string, row allaccounts.Row) string {
	if !row.HasExchangeRate || len(row.MissingFlows) == 0 {
		return noRateWarning(currency)
	}
	return missingFlowRateWarning(currency, slices.MinFunc(row.MissingFlows, time.Time.Compare))
}

// noRateWarning tells that the tool has no rate of the currency.
func noRateWarning(currency string) string {
	return fmt.Sprintf("No Exchange Rate for %s. Refresh the prices.", currency)
}

// missingFlowRateWarning tells that the tool has no rate of the currency on the day of the flow.
// A refresh gets the rate when an Import added a flow before the first day of the series.
func missingFlowRateWarning(currency string, flow time.Time) string {
	return fmt.Sprintf("No Exchange Rate for %s on %s. Refresh the prices.", currency, exchange.DayInWarsaw(flow).Format(time.DateOnly))
}

// allStaleRates tells if the summary of an Account tells stale rates, or if the latest rate of an Account currency
// that is not PLN is missing or the tool got it before today.
func allStaleRates(views []accountView, latest map[string]exchange.Latest, today time.Time) bool {
	for _, v := range views {
		if v.staleRates(today) {
			return true
		}
		if c := v.account.Currency; c != ledger.PLN {
			if rate, ok := latest[c]; !ok || rate.LastFetch.Before(today) {
				return true
			}
		}
	}
	return false
}

// rateWarnings returns the warnings about missing Exchange Rates that the view uses, in currency order.
// The view uses the rate of each Account currency that is not PLN and the rate of each Holding.
// For each currency, it tells that there is no rate, or it tells the earliest flow day with no rate.
func rateWarnings(views []accountView, rows []allaccounts.Row, latest map[string]exchange.Latest) []string {
	missingFlow := map[string]time.Time{}
	var used []string
	for i, v := range views {
		if c := v.account.Currency; c != ledger.PLN {
			used = append(used, c)
			for _, t := range rows[i].MissingFlows {
				if first, ok := missingFlow[c]; !ok || t.Before(first) {
					missingFlow[c] = t
				}
			}
		}
		for _, h := range v.state.Holdings {
			if h.ExchangeRateCurrency != "" {
				used = append(used, h.ExchangeRateCurrency)
			}
		}
	}
	slices.Sort(used)

	warnings := []string{}
	for _, currency := range slices.Compact(used) {
		if _, ok := latest[currency]; !ok {
			warnings = append(warnings, noRateWarning(currency))
			continue
		}
		if t, ok := missingFlow[currency]; ok {
			warnings = append(warnings, missingFlowRateWarning(currency, t))
		}
	}
	return warnings
}

// allPriceDate returns the oldest price date of the Holdings with a value in all Accounts, or nil.
func allPriceDate(views []accountView) *string {
	var dates []time.Time
	for _, v := range views {
		for _, h := range v.state.Holdings {
			if h.HasValue() {
				dates = append(dates, v.prices[h.Instrument.Ticker].Date)
			}
		}
	}
	return oldestDate(dates)
}

// allExchangeRateDate returns the oldest table date of the latest Exchange Rates that the view uses, or nil:
// the rate of each Account currency that is not PLN, and the rate of each Holding with a value.
func allExchangeRateDate(views []accountView, latest map[string]exchange.Latest) *string {
	var dates []time.Time
	for _, v := range views {
		if rate, ok := latest[v.account.Currency]; ok && v.account.Currency != ledger.PLN {
			dates = append(dates, rate.Date)
		}
		for _, h := range v.state.Holdings {
			if h.UsesExchangeRate() {
				dates = append(dates, h.ExchangeRate.Date)
			}
		}
	}
	return oldestDate(dates)
}
