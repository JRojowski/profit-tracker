package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// refreshResponse is the body of POST /api/prices/refresh.
type refreshResponse struct {
	// Refreshed is the number of stored prices.
	Refreshed int                      `json:"refreshed"`
	Failures  []refreshFailureResponse `json:"failures"`
}

// refreshFailureResponse is one Instrument that got no price.
type refreshFailureResponse struct {
	// Ticker is the broker ticker.
	Ticker string `json:"ticker"`
	Error  string `json:"error"`
}

// refreshPrices gets new prices for the Instruments in Holdings. The status is 200 also when all Instruments fail.
func (a *api) refreshPrices(w http.ResponseWriter, r *http.Request) {
	result, err := a.prices.Refresh(r.Context())
	if err != nil {
		a.writeInternalError(w, r, fmt.Errorf("refresh prices: %w", err))
		return
	}
	resp := refreshResponse{Refreshed: result.Refreshed, Failures: make([]refreshFailureResponse, 0, len(result.Failures))}
	for _, f := range result.Failures {
		resp.Failures = append(resp.Failures, refreshFailureResponse{Ticker: f.Ticker, Error: f.Err.Error()})
	}
	writeJSON(w, http.StatusOK, resp)
}

// accountView is the state of one Account with the price data of its Instruments.
type accountView struct {
	account store.Account
	state   ledger.State
	// instruments are the price sources of the Instruments by broker ticker.
	instruments map[string]store.Instrument
	// prices are the last stored prices by broker ticker, in any currency.
	prices map[string]store.Price
}

// accountPrices returns the close prices of the Instruments that are quoted in the Account currency.
// The spec puts the other Instruments out of scope: the tool does no currency conversion.
// The check of the price currency also ignores a stored price from before a change of the quote currency.
func accountPrices(currency string, instruments map[string]store.Instrument, last map[string]store.Price) map[string]decimal.Decimal {
	prices := make(map[string]decimal.Decimal, len(last))
	for ticker, p := range last {
		if instruments[ticker].QuoteCurrency == currency && p.Currency == currency {
			prices[ticker] = p.Close
		}
	}
	return prices
}

// priceDate returns the oldest price date of the Holdings with a price, or nil if no Holding has a price.
func (v accountView) priceDate() *string {
	var oldest time.Time
	for _, h := range v.state.Holdings {
		if !h.HasPrice {
			continue
		}
		if date := v.prices[h.Instrument.Ticker].Date; oldest.IsZero() || date.Before(oldest) {
			oldest = date
		}
	}
	if oldest.IsZero() {
		return nil
	}
	return new(oldest.Format(time.DateOnly))
}

// stalePrices tells if a Holding in the Account currency has no price, or its price was fetched before today.
func (v accountView) stalePrices(today time.Time) bool {
	for _, h := range v.state.Holdings {
		if v.instruments[h.Instrument.Ticker].QuoteCurrency != v.account.Currency {
			continue
		}
		if !h.HasPrice || v.prices[h.Instrument.Ticker].FetchedAt.Before(today) {
			return true
		}
	}
	return false
}

// unpricedHoldings returns the number of Holdings with no value.
func (v accountView) unpricedHoldings() int {
	n := 0
	for _, h := range v.state.Holdings {
		if !h.HasPrice {
			n++
		}
	}
	return n
}

// holdingRow returns the response row of the Holding.
func (v accountView) holdingRow(h ledger.Holding) holdingResponse {
	ticker := h.Instrument.Ticker
	// An Instrument of a Holding is always in the map. The zero value gives the default mapping and no quote currency.
	inst := v.instruments[ticker]
	inst.Ticker = ticker
	row := holdingResponse{
		Ticker: ticker,
		Name:   h.Instrument.Name,
		Volume: h.Volume.String(),
		Cost:   formatAmount(h.Cost),
		// The frontend rounds for display. ADR 0001: the tool rounds only for display.
		AverageCost:         h.AverageCost().String(),
		priceSourceResponse: toPriceSourceResponse(inst),
	}
	if !h.HasPrice {
		row.Note = new(v.noValueNote(ticker))
		return row
	}
	row.Price = new(h.Price.String())
	row.PriceDate = new(v.prices[ticker].Date.Format(time.DateOnly))
	row.Value = new(formatAmount(h.Value))
	row.UnrealizedProfit = new(formatAmount(h.UnrealizedProfit))
	if percent, ok := h.UnrealizedProfitPercent(); ok {
		row.UnrealizedProfitPercent = new(percent.String())
	}
	return row
}

// noValueNote returns the sentence that tells the user why the Holding of the Instrument has no value.
func (v accountView) noValueNote(ticker string) string {
	switch qc := v.instruments[ticker].QuoteCurrency; qc {
	case "":
		return "The quote currency of the Instrument is unknown. Set it in the price source of the Instrument."
	case v.account.Currency:
		return "There is no price for the Instrument. Refresh the prices."
	default:
		return fmt.Sprintf("The Instrument is quoted in %s. The tool shows no value in %s.", qc, v.account.Currency)
	}
}

// today returns the start of the current day in UTC.
func (a *api) today() time.Time {
	return price.StartOfDay(a.now())
}
