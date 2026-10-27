package httpapi

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/allaccounts"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

// allValueHistoryResponse is the body of GET /api/all-accounts/value-history. The amounts are in PLN.
// It has the fields of the Value History of an Account, with no accountId.
type allValueHistoryResponse struct {
	// Available tells if the Value History is available. If it is false, Points is empty,
	// each period is nil and Warnings tell why.
	Available bool `json:"available"`
	// Warnings are sentences for the user about missing Exchange Rates. They start with the warnings
	// of GET /api/all-accounts/summary. The list is empty if there is nothing to say.
	Warnings []string `json:"warnings"`
	valueHistoryBody
}

// allAccountsValueHistory returns the Value History and the Time-Weighted Returns of all Accounts in PLN.
// It calculates them on each request and stores nothing. ADR 0004.
func (a *api) allAccountsValueHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	prices, err := a.store.PriceHistory(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	stored, err := a.store.ExchangeRateSeries(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	series := rateSeries(stored)
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	warnings, err := a.readPriceWarnings(ctx, instruments, prices)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	views := make([]accountView, 0, len(accounts))
	histories := make([]allaccounts.HistoryAccount, 0, len(accounts))
	var valuedAtCost []ledger.Instrument
	var closes map[string][]valuehistory.Close
	for _, acc := range accounts {
		in, err := a.readHeroInput(ctx, acc)
		if err != nil {
			a.writeInternalError(w, r, err)
			return
		}
		view, err := viewOf(acc, in)
		if err != nil {
			a.writeInternalError(w, r, err)
			return
		}
		views = append(views, view)
		// The Instruments are the same for each Account.
		if closes == nil {
			closes = quoteCloses(in.instruments, prices)
		}
		history, err := valuehistory.Calculate(valuehistory.Input{Ledger: in.ledger, Closes: closes, ExchangeRates: series})
		if err != nil {
			a.writeInternalError(w, r, fmt.Errorf("calculate the value history of account %d: %w", acc.ID, err))
			return
		}
		histories = append(histories, allaccounts.HistoryAccount{Currency: acc.Currency, Points: history.Points})
		valuedAtCost = append(valuedAtCost, history.ValuedAtCost...)
	}
	hero, err := a.calculateAllAccounts(ctx, views)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	resp := allValueHistoryResponse{
		Warnings:         rateWarnings(views, hero.result.Rows, hero.latest),
		valueHistoryBody: unavailableValueHistoryBody(),
	}
	// The Price Warnings do not need an Exchange Rate. They show also when the history is not available.
	resp.PriceWarnings = warnings.all()
	// The history uses the same rates as the hero. It is not available when the hero is not available.
	if !hero.result.Available {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	history := allaccounts.ValueHistory(histories, hero.rates, series)
	if !history.Available {
		resp.Warnings = append(resp.Warnings, historyRateWarnings(history.Missing)...)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Available = true
	resp.valueHistoryBody = newValueHistoryBody(ledger.PLN, history.Points, uniqueInstruments(valuedAtCost))
	resp.PriceWarnings = warnings.all()
	writeJSON(w, http.StatusOK, resp)
}

// unavailableValueHistoryBody returns the body in PLN with no points. Each period is nil, and the lists are empty.
func unavailableValueHistoryBody() valueHistoryBody {
	return newValueHistoryBody(ledger.PLN, nil, nil)
}

// historyRateWarnings returns one sentence for each missing rate of the Value History, in currency order.
// The hero has no such warning. A missing latest rate or a missing rate of a flow makes the hero not available,
// and the hero warns about it.
func historyRateWarnings(missing []allaccounts.MissingRate) []string {
	slices.SortFunc(missing, func(x, y allaccounts.MissingRate) int { return cmp.Compare(x.Currency, y.Currency) })
	warnings := make([]string, 0, len(missing))
	for _, m := range missing {
		if m.Day.IsZero() {
			warnings = append(warnings, noRateWarning(m.Currency))
			continue
		}
		warnings = append(warnings, fmt.Sprintf("No Exchange Rate for %s on %s or before it.", m.Currency, m.Day.Format(time.DateOnly)))
	}
	// Two Accounts with the same currency can have the same missing rate.
	return slices.Compact(warnings)
}

// uniqueInstruments returns each Instrument once, in ticker order.
func uniqueInstruments(instruments []ledger.Instrument) []ledger.Instrument {
	slices.SortFunc(instruments, func(x, y ledger.Instrument) int { return cmp.Compare(x.Ticker, y.Ticker) })
	return slices.CompactFunc(instruments, func(x, y ledger.Instrument) bool { return x.Ticker == y.Ticker })
}
