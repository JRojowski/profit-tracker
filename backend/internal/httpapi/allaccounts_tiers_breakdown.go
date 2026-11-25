package httpapi

import (
	"net/http"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/allaccounts"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// allTiersBreakdownResponse is the body of GET /api/all-accounts/tiers-breakdown. All amounts are in PLN. ADR 0010.
type allTiersBreakdownResponse struct {
	// Available tells if the tool has each Exchange Rate that the conversion needs. If it is false, Warnings tell
	// why, and the body has no fields of the breakdown.
	Available bool `json:"available"`
	// Warnings are sentences for the user about missing rates, in currency order. It is empty if there is nothing to say.
	Warnings []string `json:"warnings"`
	*tiersBreakdownResponse
}

// allAccountsTiersBreakdown returns the Tiers Breakdown of all Tier Accounts together in PLN. ADR 0010.
func (a *api) allAccountsTiersBreakdown(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	views, err := a.allAccountViews(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	tiers, err := a.store.Tiers(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	rates, _, err := a.rateLookups(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	var holdings []ledger.Holding
	var closed []accountClosedPosition
	openLots := []ledger.OpenLot{}
	missing := missingRates{noLatest: map[string]bool{}, firstDay: map[string]time.Time{}}
	for _, v := range views {
		acc := v.account
		if !tier.IsTierAccount(acc.Type, acc.Provider, acc.Currency) {
			continue
		}
		res := allaccounts.ResultsInPLN(allaccounts.Account{ID: acc.ID, Currency: acc.Currency, State: v.state}, v.lots, rates)
		missing.add(acc.Currency, res)
		holdings = append(holdings, res.Holdings...)
		openLots = append(openLots, v.state.OpenLots...)
		account := &breakdownAccountResponse{ID: acc.ID, Name: acc.Name, Currency: acc.Currency}
		closed = append(closed, withAccount(res.ClosedPositions, account)...)
	}

	warnings := missing.warnings()
	resp := allTiersBreakdownResponse{Available: len(warnings) == 0, Warnings: warnings}
	if resp.Available {
		breakdown := tiersBreakdownOf(ledger.PLN, tiers, holdings, lastPurchaseTimes(openLots), closed)
		resp.tiersBreakdownResponse = &breakdown
	}
	writeJSON(w, http.StatusOK, resp)
}

// missingRates collects the Exchange Rates that the conversion of the Accounts does not have.
type missingRates struct {
	// noLatest holds the currencies with no latest rate.
	noLatest map[string]bool
	// firstDay is the earliest time with no rate on its day, by currency.
	firstDay map[string]time.Time
}

func (m missingRates) add(currency string, res allaccounts.Results) {
	if res.NoLatestRate {
		m.noLatest[currency] = true
	}
	for _, t := range res.MissingDays {
		if first, ok := m.firstDay[currency]; !ok || t.Before(first) {
			m.firstDay[currency] = t
		}
	}
}

// warnings returns one sentence per currency with a missing rate, in currency order.
func (m missingRates) warnings() []string {
	currencies := make([]string, 0, len(m.noLatest)+len(m.firstDay))
	for c := range m.noLatest {
		currencies = append(currencies, c)
	}
	for c := range m.firstDay {
		currencies = append(currencies, c)
	}
	slices.Sort(currencies)

	warnings := []string{}
	for _, c := range slices.Compact(currencies) {
		if m.noLatest[c] {
			warnings = append(warnings, noRateWarning(c))
			continue
		}
		warnings = append(warnings, missingFlowRateWarning(c, m.firstDay[c]))
	}
	return warnings
}
