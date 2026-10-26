// Package httpapi is the JSON HTTP API of the tool under /api. It also serves the built frontend.
package httpapi

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/importer"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// Options holds the dependencies of the HTTP API.
type Options struct {
	// Store is the database. It must have the migrations applied.
	Store *store.Store
	// Frontend holds the built frontend, with index.html at the root.
	// If index.html is missing, the pages tell the user to build the frontend.
	Frontend fs.FS
	// Logger receives the errors that the API cannot return to the client.
	Logger *slog.Logger
	// Prices gets and stores the prices. It is required.
	// The caller shares it with the refresh at start, so that the two refreshes do not overlap.
	Prices *price.Service
	// ExchangeRates gets and stores the Exchange Rates. It is required.
	// The caller shares it with the refresh at start, so that the two refreshes do not overlap.
	ExchangeRates *exchange.Service
	// Now is the clock that tells "today" for the stale-price and stale-rate warnings. If it is nil, the API uses
	// time.Now. Give it the same clock as Prices and ExchangeRates.
	Now func() time.Time
}

// NewHandler returns the handler for the API routes and the frontend.
func NewHandler(opts Options) http.Handler {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	a := &api{
		store: opts.Store, importer: importer.NewService(opts.Store), prices: opts.Prices, rates: opts.ExchangeRates,
		now: now, logger: opts.Logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("POST /api/imports", a.previewImport)
	mux.HandleFunc("POST /api/imports/{id}/confirm", a.confirmImport)
	mux.HandleFunc("POST /api/imports/{id}/cancel", a.cancelImport)
	mux.HandleFunc("POST /api/imports/{id}/conflicts/{conflictId}", a.decideConflict)
	mux.HandleFunc("GET /api/imports", a.listImports)
	mux.HandleFunc("GET /api/accounts", a.listAccounts)
	mux.HandleFunc("PATCH /api/accounts/{id}", a.renameAccount)
	mux.HandleFunc("GET /api/accounts/{id}/summary", a.accountSummary)
	mux.HandleFunc("GET /api/accounts/{id}/holdings", a.accountHoldings)
	mux.HandleFunc("GET /api/accounts/{id}/sales", a.accountSales)
	mux.HandleFunc("GET /api/accounts/{id}/ikze-limit", a.accountIKZELimit)
	mux.HandleFunc("GET /api/accounts/{id}/allocation", a.accountAllocation)
	mux.HandleFunc("GET /api/all-accounts/summary", a.allAccountsSummary)
	mux.HandleFunc("GET /api/all-accounts/allocation", a.allAccountsAllocation)
	mux.HandleFunc("POST /api/prices/refresh", a.refreshPrices)
	mux.HandleFunc("PUT /api/instruments/{ticker}/price-ticker", a.setPriceTicker)
	mux.HandleFunc("PUT /api/instruments/{ticker}/quote-currency", a.setQuoteCurrency)
	mux.HandleFunc("/api/", notFound)
	mux.Handle("/", frontendHandler(opts.Frontend))
	return mux
}

// api holds the dependencies of the API handlers.
type api struct {
	store    *store.Store
	importer *importer.Service
	prices   *price.Service
	rates    *exchange.Service
	now      func() time.Time
	logger   *slog.Logger
}

// errorResponse is the JSON body of each API error.
type errorResponse struct {
	Error string `json:"error"`
}

// notFound answers an unknown /api path. It never returns the frontend.
func notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "no API route for "+r.Method+" "+r.URL.Path)
}

// writeJSON writes v as a JSON body with the status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The status is already sent. The client sees a cut body if the write fails.
	_ = json.NewEncoder(w).Encode(v)
}
