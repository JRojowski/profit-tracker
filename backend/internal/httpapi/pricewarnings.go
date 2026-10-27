package httpapi

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/pricecheck"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// priceWarningResponse is one Price Warning. It embeds the price source of the Instrument, so that the frontend
// can open the price ticker and Quote Currency dialog also for a sold Instrument.
type priceWarningResponse struct {
	// Ticker is the broker ticker.
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// Kind is "noData", "differentCurrency" or "priceMismatch".
	Kind string `json:"kind"`
	// SourceCurrency is the currency of the price source. It is set only for "differentCurrency".
	SourceCurrency *string `json:"sourceCurrency"`
	// MedianDifference and MaxDifference are percentages in decimal text, with no rounding.
	// They are set only for "priceMismatch".
	MedianDifference *string `json:"medianDifference"`
	MaxDifference    *string `json:"maxDifference"`
	// Confirmable tells that the user can confirm the price source to remove the warning.
	// It is true only for "priceMismatch".
	Confirmable bool `json:"confirmable"`
	priceSourceResponse
}

// priceConfirmationRequest is the body of PUT /api/instruments/{ticker}/price-confirmation.
type priceConfirmationRequest struct {
	// Confirmed tells that the price ticker and the Quote Currency of the Instrument are correct.
	// False removes the confirmation.
	Confirmed bool `json:"confirmed"`
}

// setPriceConfirmation sets or removes the confirmation of the user that the price ticker and the Quote Currency
// of the Instrument are correct. The confirmation removes only the Price Warning "price mismatch".
func (a *api) setPriceConfirmation(w http.ResponseWriter, r *http.Request) {
	var req priceConfirmationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	a.writeInstrument(w, r, func(ctx context.Context, ticker string) (store.Instrument, error) {
		return a.store.SetPriceConfirmed(ctx, ticker, req.Confirmed)
	})
}

// priceWarnings are the Price Warnings of each Instrument that an Account ever held, by broker ticker.
// An Instrument with no warning is not in the map.
type priceWarnings map[string][]priceWarningResponse

// readPriceWarnings reads the data of the price check and checks each Instrument that an Account ever held.
// history is the stored prices of each Instrument, from store.PriceHistory.
func (a *api) readPriceWarnings(ctx context.Context, instruments map[string]store.Instrument, history map[string][]store.Price) (priceWarnings, error) {
	trades, err := a.store.TradePrices(ctx)
	if err != nil {
		return nil, err
	}
	warnings := priceWarnings{}
	for ticker, tickerTrades := range trades {
		inst := instruments[ticker]
		inst.Ticker = ticker
		in := pricecheck.Input{
			QuoteCurrency: inst.QuoteCurrency, SourceCurrency: inst.SourceCurrency, NoData: inst.NoData,
			Confirmed: inst.PriceConfirmed,
		}
		for _, p := range history[ticker] {
			in.Closes = append(in.Closes, pricecheck.Close{Date: p.Date, Price: p.Close, Currency: p.Currency})
		}
		for _, t := range tickerTrades {
			in.Trades = append(in.Trades, pricecheck.Trade{Time: t.Time, Price: t.Price})
		}
		for _, w := range pricecheck.Check(in) {
			warnings[ticker] = append(warnings[ticker], toPriceWarningResponse(inst, w))
		}
	}
	return warnings, nil
}

// readAllPriceWarnings reads the stored prices and the price sources, and returns the Price Warnings.
func (a *api) readAllPriceWarnings(ctx context.Context) (priceWarnings, error) {
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		return nil, err
	}
	history, err := a.store.PriceHistory(ctx)
	if err != nil {
		return nil, err
	}
	return a.readPriceWarnings(ctx, instruments, history)
}

// of returns the Price Warnings of the Instrument. It returns an empty slice and not nil,
// so that the JSON has a list.
func (p priceWarnings) of(ticker string) []priceWarningResponse {
	return append([]priceWarningResponse{}, p[ticker]...)
}

// all returns the Price Warnings of each Instrument, in broker ticker order and then in kind order.
func (p priceWarnings) all() []priceWarningResponse {
	return p.forTickers(slices.Collect(maps.Keys(p)))
}

// forLedger returns the Price Warnings of the Instruments of the Lots of the Ledger, also of the sold ones.
func (p priceWarnings) forLedger(l ledger.Ledger) []priceWarningResponse {
	tickers := make([]string, 0, len(l.Lots))
	for _, lot := range l.Lots {
		tickers = append(tickers, lot.Instrument.Ticker)
	}
	return p.forTickers(tickers)
}

// forTickers returns the Price Warnings of the Instruments, in broker ticker order and then in kind order.
// pricecheck.Check gives the warnings of one Instrument in kind order.
func (p priceWarnings) forTickers(tickers []string) []priceWarningResponse {
	slices.Sort(tickers)
	list := []priceWarningResponse{}
	for _, ticker := range slices.Compact(tickers) {
		list = append(list, p[ticker]...)
	}
	return list
}

// toPriceWarningResponse returns the response of the Price Warning of the Instrument.
// The frontend builds the sentence for the user from the fields.
func toPriceWarningResponse(inst store.Instrument, w pricecheck.Warning) priceWarningResponse {
	resp := priceWarningResponse{Ticker: inst.Ticker, Name: inst.Name, priceSourceResponse: toPriceSourceResponse(inst)}
	switch w.Kind {
	case pricecheck.NoData:
		resp.Kind = "noData"
	case pricecheck.DifferentCurrency:
		resp.Kind = "differentCurrency"
		resp.SourceCurrency = new(w.SourceCurrency)
	case pricecheck.PriceMismatch:
		resp.Kind = "priceMismatch"
		// The frontend rounds the percentages for display.
		resp.MedianDifference, resp.MaxDifference = new(w.MedianDifference.String()), new(w.MaxDifference.String())
		resp.Confirmable = true
	}
	return resp
}
