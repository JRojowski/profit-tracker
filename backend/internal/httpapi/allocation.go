package httpapi

import (
	"maps"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/allocation"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// allocationResponse is the body of GET /api/accounts/{id}/allocation and GET /api/all-accounts/allocation.
// Amounts and shares are decimal text. A share is the value × 100 / the total, with at most 16 decimal places.
// The frontend rounds it for display.
type allocationResponse struct {
	// Available is false when a needed Exchange Rate is missing. Then Warnings tell why, Total is nil
	// and the lists are empty.
	Available bool     `json:"available"`
	Warnings  []string `json:"warnings"`
	// Currency is the Account currency, or PLN for all Accounts.
	Currency        string                      `json:"currency"`
	Total           *string                     `json:"total"`
	ByInstrument    allocationByInstrument      `json:"byInstrument"`
	ByQuoteCurrency []allocationCurrencyPartDTO `json:"byQuoteCurrency"`
	// UnpricedHoldings is the number of Holdings with no known value. They are not parts.
	UnpricedHoldings int `json:"unpricedHoldings"`
}

// allocationByInstrument is the Allocation by Instrument.
type allocationByInstrument struct {
	// Holdings are at most allocation.MaxHoldings parts, the largest value first.
	Holdings []allocationPartDTO `json:"holdings"`
	// Cash is nil when the cash is zero or less.
	Cash *allocationPartDTO `json:"cash"`
	// Other is nil when there are allocation.MaxHoldings Holdings or fewer.
	Other *allocationOtherDTO `json:"other"`
}

// allocationPartDTO is one part of the Allocation. The cash part has an empty ticker.
type allocationPartDTO struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Share  string `json:"share"`
}

// allocationOtherDTO holds the Holdings after the allocation.MaxHoldings largest.
type allocationOtherDTO struct {
	Count    int                 `json:"count"`
	Value    string              `json:"value"`
	Share    string              `json:"share"`
	Holdings []allocationPartDTO `json:"holdings"`
}

// allocationCurrencyPartDTO is the value of one Quote Currency.
type allocationCurrencyPartDTO struct {
	Currency string `json:"currency"`
	Value    string `json:"value"`
	Share    string `json:"share"`
}

// accountAllocation returns the Allocation of the Account in the Account currency.
func (a *api) accountAllocation(w http.ResponseWriter, r *http.Request) {
	view, ok := a.accountView(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, a.allocation([]accountView{view}, view.account.Currency))
}

// allAccountsAllocation returns the Allocation of all Accounts in PLN, at the latest Exchange Rate. ADR 0004.
func (a *api) allAccountsAllocation(w http.ResponseWriter, r *http.Request) {
	views, err := a.allAccountViews(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a.allocation(views, ledger.PLN))
}

// allocation calculates the Allocation of the views in the currency and returns it as a response.
// Each view has the same Instruments and latest Exchange Rates, because readAccountView reads them for all Accounts.
func (a *api) allocation(views []accountView, currency string) allocationResponse {
	input := make([]allocation.Account, 0, len(views))
	quoteCurrencies := map[string]string{}
	rates := map[string]exchange.Latest{}
	for _, v := range views {
		input = append(input, allocation.Account{Currency: v.account.Currency, State: v.state})
		for ticker, inst := range v.instruments {
			quoteCurrencies[ticker] = inst.QuoteCurrency
		}
		maps.Copy(rates, v.rates)
	}
	result := allocation.Calculate(input, currency, quoteCurrencies, allocation.Rates{
		Latest: func(c string) (decimal.Decimal, bool) {
			rate, ok := rates[c]
			return rate.Mid, ok
		},
	})

	resp := allocationResponse{
		Available:        result.Available,
		Warnings:         make([]string, 0, len(result.MissingRates)),
		Currency:         currency,
		ByInstrument:     allocationByInstrument{Holdings: toPartDTOs(result.Holdings)},
		ByQuoteCurrency:  make([]allocationCurrencyPartDTO, 0, len(result.ByQuoteCurrency)),
		UnpricedHoldings: result.UnpricedHoldings,
	}
	for _, c := range result.MissingRates {
		resp.Warnings = append(resp.Warnings, noRateWarning(c))
	}
	if !result.Available {
		return resp
	}
	resp.Total = new(formatAmount(result.Total))
	if result.Cash != nil {
		resp.ByInstrument.Cash = new(toPartDTO(*result.Cash))
	}
	if o := result.Other; o != nil {
		resp.ByInstrument.Other = &allocationOtherDTO{
			Count: len(o.Holdings), Value: formatAmount(o.Value), Share: o.Share.String(), Holdings: toPartDTOs(o.Holdings),
		}
	}
	for _, c := range result.ByQuoteCurrency {
		resp.ByQuoteCurrency = append(resp.ByQuoteCurrency, allocationCurrencyPartDTO{
			Currency: c.Currency, Value: formatAmount(c.Value), Share: c.Share.String(),
		})
	}
	return resp
}

// toPartDTOs returns the parts as decimal text. It returns an empty list, not nil, for no parts.
func toPartDTOs(parts []allocation.Part) []allocationPartDTO {
	dtos := make([]allocationPartDTO, 0, len(parts))
	for _, p := range parts {
		dtos = append(dtos, toPartDTO(p))
	}
	return dtos
}

// toPartDTO returns the part as decimal text.
func toPartDTO(p allocation.Part) allocationPartDTO {
	return allocationPartDTO{Ticker: p.Ticker, Name: p.Name, Value: formatAmount(p.Value), Share: p.Share.String()}
}
