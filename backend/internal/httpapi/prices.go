package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// tooManyRequestsMessage is the stop message of a refresh that the price source blocks.
const tooManyRequestsMessage = "Yahoo blocks the requests, try again later"

// refreshResponse is the body of POST /api/prices/refresh.
type refreshResponse struct {
	// Refreshed is the number of stored current prices.
	Refreshed int `json:"refreshed"`
	// Failures are the Instruments that got no current price.
	Failures []refreshFailureResponse `json:"failures"`
	// RateFailures are the currencies that got no Exchange Rate.
	RateFailures []rateFailureResponse `json:"rateFailures"`
	// HistoryStored is the number of stored daily closes.
	HistoryStored int `json:"historyStored"`
	// HistoryFailures are the Instruments that got no history.
	HistoryFailures []refreshFailureResponse `json:"historyFailures"`
	// TooManyRequests tells that the price source blocked the requests and the refresh stopped.
	TooManyRequests bool `json:"tooManyRequests"`
	// StopMessage is the message for the user when the refresh stopped, or nil.
	StopMessage *string `json:"stopMessage"`
	// PriceWarnings are the Price Warnings of each Instrument that an Account ever held, after the refresh.
	PriceWarnings []priceWarningResponse `json:"priceWarnings"`
}

// progressResponse is the body of GET /api/prices/refresh/progress.
type progressResponse struct {
	// Running tells that a refresh runs.
	Running bool `json:"running"`
	// Done and Total are the numbers of the completed and of all history requests of the running refresh.
	Done  int `json:"done"`
	Total int `json:"total"`
	// Ticker is the broker ticker of the running history request, or nil.
	Ticker *string `json:"ticker"`
}

// rateFailureResponse is one currency that got no Exchange Rate.
type rateFailureResponse struct {
	// Currency is the ISO code, for example "USD".
	Currency string `json:"currency"`
	Error    string `json:"error"`
}

// refreshFailureResponse is one Instrument that got no price.
type refreshFailureResponse struct {
	// Ticker is the broker ticker.
	Ticker string `json:"ticker"`
	Error  string `json:"error"`
}

// refreshPrices gets new prices for the Instruments in Holdings and the missing daily history,
// and then the missing Exchange Rates. The status is 200 also when all Instruments and all currencies fail.
func (a *api) refreshPrices(w http.ResponseWriter, r *http.Request) {
	result, err := a.prices.Refresh(r.Context())
	if err != nil {
		a.writeInternalError(w, r, fmt.Errorf("refresh prices: %w", err))
		return
	}
	rateResult, err := a.rates.Refresh(r.Context())
	if err != nil {
		a.writeInternalError(w, r, fmt.Errorf("refresh exchange rates: %w", err))
		return
	}
	warnings, err := a.readAllPriceWarnings(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	resp := refreshResponse{
		PriceWarnings:   warnings.all(),
		Refreshed:       result.Refreshed,
		Failures:        toRefreshFailures(result.Failures),
		RateFailures:    make([]rateFailureResponse, 0, len(rateResult.Failures)),
		HistoryStored:   result.HistoryStored,
		HistoryFailures: toRefreshFailures(result.HistoryFailures),
		TooManyRequests: result.TooManyRequests,
	}
	if result.TooManyRequests {
		resp.StopMessage = new(tooManyRequestsMessage)
	}
	for _, f := range rateResult.Failures {
		resp.RateFailures = append(resp.RateFailures, rateFailureResponse{Currency: f.Currency, Error: f.Err.Error()})
	}
	writeJSON(w, http.StatusOK, resp)
}

// toRefreshFailures returns the response rows of the failures. It returns an empty slice and not nil,
// so that the JSON has a list.
func toRefreshFailures(failures []price.Failure) []refreshFailureResponse {
	rows := make([]refreshFailureResponse, 0, len(failures))
	for _, f := range failures {
		rows = append(rows, refreshFailureResponse{Ticker: f.Ticker, Error: f.Err.Error()})
	}
	return rows
}

// refreshProgress returns the progress of the running price refresh. The frontend asks for it while
// POST /api/prices/refresh runs.
func (a *api) refreshProgress(w http.ResponseWriter, _ *http.Request) {
	p := a.prices.Progress()
	writeJSON(w, http.StatusOK, progressResponse{Running: p.Running, Done: p.Done, Total: p.Total, Ticker: optional(p.Ticker)})
}

// accountView is the state of one Account with the price data of its Instruments.
type accountView struct {
	account store.Account
	state   ledger.State
	// instruments are the price sources of the Instruments by broker ticker.
	instruments map[string]store.Instrument
	// prices are the last stored prices by broker ticker, in any currency.
	prices map[string]store.Price
	// rates are the latest Exchange Rates by currency.
	rates map[string]exchange.Latest
}

// quotePrices returns the close prices of the Instruments in their Quote Currency.
// The check of the price currency ignores a stored price from before a change of the Quote Currency.
func quotePrices(instruments map[string]store.Instrument, last map[string]store.Price) map[string]decimal.Decimal {
	prices := make(map[string]decimal.Decimal, len(last))
	for ticker, p := range last {
		if qc := instruments[ticker].QuoteCurrency; qc != "" && p.Currency == qc {
			prices[ticker] = p.Close
		}
	}
	return prices
}

// priceDate returns the oldest price date of the Holdings with a value, or nil if no Holding has a value.
func (v accountView) priceDate() *string {
	var dates []time.Time
	for _, h := range v.state.Holdings {
		if h.HasValue() {
			dates = append(dates, v.prices[h.Instrument.Ticker].Date)
		}
	}
	return oldestDate(dates)
}

// exchangeRateDate returns the oldest table date of the Exchange Rates that the Holdings use,
// or nil if no Holding uses an Exchange Rate.
func (v accountView) exchangeRateDate() *string {
	var dates []time.Time
	for _, h := range v.state.Holdings {
		if h.UsesExchangeRate() {
			dates = append(dates, h.ExchangeRate.Date)
		}
	}
	return oldestDate(dates)
}

// oldestDate returns the oldest date as YYYY-MM-DD, or nil if dates is empty.
func oldestDate(dates []time.Time) *string {
	if len(dates) == 0 {
		return nil
	}
	return new(slices.MinFunc(dates, time.Time.Compare).Format(time.DateOnly))
}

// stalePrices tells if a Holding with a known Quote Currency has no price in that currency,
// or its price was fetched before today. A Holding with an unknown Quote Currency gets no price from a refresh.
func (v accountView) stalePrices(today time.Time) bool {
	for _, h := range v.state.Holdings {
		qc := v.instruments[h.Instrument.Ticker].QuoteCurrency
		if qc == "" {
			continue
		}
		p, ok := v.prices[h.Instrument.Ticker]
		if !ok || p.Currency != qc || p.FetchedAt.Before(today) {
			return true
		}
	}
	return false
}

// staleRates tells if a Holding needs an Exchange Rate that is missing, or that the tool fetched before today.
// A Holding needs a rate when its Quote Currency is not the Account currency and one of the two is PLN.
func (v accountView) staleRates(today time.Time) bool {
	for _, h := range v.state.Holdings {
		if h.ExchangeRateCurrency == "" {
			continue
		}
		r, ok := v.rates[h.ExchangeRateCurrency]
		if !ok || r.LastFetch.Before(today) {
			return true
		}
	}
	return false
}

// unpricedHoldings returns the number of Holdings with no value.
func (v accountView) unpricedHoldings() int {
	n := 0
	for _, h := range v.state.Holdings {
		if !h.HasValue() {
			n++
		}
	}
	return n
}

// holdingRow returns the response row of the Holding with its Price Warnings.
func (v accountView) holdingRow(h ledger.Holding, warnings priceWarnings) holdingResponse {
	ticker := h.Instrument.Ticker
	// An Instrument of a Holding is always in the map. The zero value gives the default mapping and no Quote Currency.
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
		PriceWarnings:       warnings.of(ticker),
	}
	if !h.HasValue() {
		row.Note = new(v.noValueNote(h))
		return row
	}
	row.Price = new(h.Price.String())
	row.PriceDate = new(v.prices[ticker].Date.Format(time.DateOnly))
	if h.UsesExchangeRate() {
		row.ExchangeRate = new(h.ExchangeRate.Rate.String())
		row.ExchangeRateCurrency = new(h.ExchangeRateCurrency)
		row.ExchangeRateDate = new(h.ExchangeRate.Date.Format(time.DateOnly))
	}
	row.Value = new(formatAmount(h.Value))
	row.UnrealizedProfit = new(formatAmount(h.UnrealizedProfit))
	if percent, ok := h.UnrealizedProfitPercent(); ok {
		row.UnrealizedProfitPercent = new(percent.String())
	}
	return row
}

// noValueNote returns the sentence that tells the user why the Holding has no value.
func (v accountView) noValueNote(h ledger.Holding) string {
	switch h.NoValueReason {
	case ledger.ReasonUnknownQuoteCurrency:
		return "The quote currency of the Instrument is unknown. Set it in the price source of the Instrument."
	case ledger.ReasonCrossRate:
		qc, account := v.instruments[h.Instrument.Ticker].QuoteCurrency, v.account.Currency
		return fmt.Sprintf("The Instrument is quoted in %s. The tool has no Exchange Rate between %s and %s.", qc, qc, account)
	case ledger.ReasonNoExchangeRate:
		return fmt.Sprintf("There is no Exchange Rate for %s. Refresh the prices.", h.ExchangeRateCurrency)
	default:
		return "There is no price for the Instrument. Refresh the prices."
	}
}

// today returns the start of the current day in UTC.
func (a *api) today() time.Time {
	return price.StartOfDay(a.now())
}
