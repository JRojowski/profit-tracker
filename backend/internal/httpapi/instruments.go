package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// instrumentResponse is the body of PUT /api/instruments/{ticker}/price-ticker and PUT /api/instruments/{ticker}/quote-currency.
type instrumentResponse struct {
	// Ticker is the broker ticker.
	Ticker string `json:"ticker"`
	priceSourceResponse
}

// priceSourceResponse is the price source of an Instrument. The Instrument body and each Holding row embed it,
// so its fields are at the top level of the JSON object.
type priceSourceResponse struct {
	// PriceTicker is the price ticker that the tool uses: the manual price ticker, or the default mapping.
	PriceTicker       string `json:"priceTicker"`
	PriceTickerManual bool   `json:"priceTickerManual"`
	// QuoteCurrency is nil when the Quote Currency of the Instrument is not known.
	QuoteCurrency       *string `json:"quoteCurrency"`
	QuoteCurrencyManual bool    `json:"quoteCurrencyManual"`
}

// priceTickerRequest is the body of PUT /api/instruments/{ticker}/price-ticker.
type priceTickerRequest struct {
	// PriceTicker is the manual price ticker. An empty value removes it.
	PriceTicker string `json:"priceTicker"`
}

// quoteCurrencyRequest is the body of PUT /api/instruments/{ticker}/quote-currency.
type quoteCurrencyRequest struct {
	// QuoteCurrency is the manual Quote Currency. An empty value restores the Quote Currency from the ticker suffix.
	QuoteCurrency string `json:"quoteCurrency"`
}

// setPriceTicker sets or removes the manual price ticker of the Instrument. It does not get a price.
// The frontend refreshes the prices after the change.
func (a *api) setPriceTicker(w http.ResponseWriter, r *http.Request) {
	var req priceTickerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	a.writeInstrument(w, r, func(ctx context.Context, ticker string) (store.Instrument, error) {
		return a.prices.SetPriceTicker(ctx, ticker, req.PriceTicker)
	})
}

// setQuoteCurrency sets or removes the manual Quote Currency of the Instrument.
func (a *api) setQuoteCurrency(w http.ResponseWriter, r *http.Request) {
	var req quoteCurrencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	a.writeInstrument(w, r, func(ctx context.Context, ticker string) (store.Instrument, error) {
		return a.prices.SetQuoteCurrency(ctx, ticker, req.QuoteCurrency)
	})
}

// writeInstrument runs the change on the Instrument of the path and writes the changed Instrument,
// or the error response.
func (a *api) writeInstrument(w http.ResponseWriter, r *http.Request, change func(ctx context.Context, ticker string) (store.Instrument, error)) {
	inst, err := change(r.Context(), r.PathValue("ticker"))
	switch {
	case errors.Is(err, price.ErrInvalidPriceTicker), errors.Is(err, price.ErrInvalidQuoteCurrency):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		a.writeInternalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, toInstrumentResponse(inst))
	}
}

func toInstrumentResponse(inst store.Instrument) instrumentResponse {
	return instrumentResponse{Ticker: inst.Ticker, priceSourceResponse: toPriceSourceResponse(inst)}
}

func toPriceSourceResponse(inst store.Instrument) priceSourceResponse {
	return priceSourceResponse{
		PriceTicker:         price.EffectiveTicker(inst.Ticker, inst.PriceTicker),
		PriceTickerManual:   inst.PriceTicker != "",
		QuoteCurrency:       optional(inst.QuoteCurrency),
		QuoteCurrencyManual: inst.QuoteCurrencyManual,
	}
}

// optional returns nil for an empty string, else a pointer to s.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
