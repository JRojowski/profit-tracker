//go:build live

package httpapi_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/httpapi"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/price/yahoo"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// The live test sends real requests to Yahoo Finance. It is not in `make test`.
// Start it by hand from the backend folder:
//
//	go test -tags live -count=1 -timeout 30m -run TestLiveHistory -v ./internal/httpapi/
//
// It imports the real exports from "exports/" and runs the full refresh with the real pause between
// the history requests. It logs the time, the number of rows and each HTTP 429.
// The log shows no ticker and no amount, because the exports contain personal data.

// liveProvider is the Yahoo provider that counts the requests and logs each HTTP 429.
// It is safe for concurrent use.
type liveProvider struct {
	*yahoo.Provider
	t *testing.T

	mu                       sync.Mutex
	currentRequests          int
	historyRequests, history int
	tooManyRequests          int
}

func (p *liveProvider) LastClose(ctx context.Context, ticker string) (price.Quote, error) {
	q, err := p.Provider.LastClose(ctx, ticker)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentRequests++
	if errors.Is(err, price.ErrTooManyRequests) {
		p.tooManyRequests++
		p.t.Logf("HTTP 429 at current price request %d", p.currentRequests)
	}
	return q, err
}

func (p *liveProvider) DailyCloses(ctx context.Context, ticker string, from, to time.Time) (price.History, error) {
	start := time.Now()
	h, err := p.Provider.DailyCloses(ctx, ticker, from, to)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.historyRequests++
	p.history += len(h.Closes)
	if errors.Is(err, price.ErrTooManyRequests) {
		p.tooManyRequests++
		p.t.Logf("HTTP 429 at history request %d", p.historyRequests)
	}
	p.t.Logf("history request %d: %d days asked, %d rows, %s, error: %t",
		p.historyRequests, int(to.Sub(from).Hours()/24)+1, len(h.Closes), time.Since(start).Round(time.Millisecond), err != nil)
	return h, err
}

// liveRefreshResponse is the body of POST /api/prices/refresh, with the counts only.
type liveRefreshResponse struct {
	Refreshed       int              `json:"refreshed"`
	Failures        []refreshFailure `json:"failures"`
	HistoryStored   int              `json:"historyStored"`
	HistoryFailures []refreshFailure `json:"historyFailures"`
	TooManyRequests bool             `json:"tooManyRequests"`
}

func TestLiveHistory_RealExports_GetsFullHistoryFromYahoo(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "profit-tracker.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	provider := &liveProvider{Provider: yahoo.New(), t: t}
	prices := price.NewService(db, provider, time.Now, price.PauseBetweenRequests)
	// The test is about Yahoo. The Exchange Rates come from the fake, so that the test sends no NBP request.
	rates := exchange.NewService(db, newFakeRateProvider(), time.Now)
	server := httptest.NewServer(httpapi.NewHandler(httpapi.Options{
		Store: db, Frontend: testFrontend, Logger: slog.New(slog.DiscardHandler), Prices: prices, ExchangeRates: rates,
	}))
	t.Cleanup(server.Close)
	srv := &testServer{URL: server.URL, client: server.Client(), prices: prices, rates: rates, db: db}
	for _, e := range exports {
		srv.importRealExport(t, e)
	}

	for _, run := range []string{"first", "second"} {
		before := provider.historyRequests
		start := time.Now()
		status, body := srv.postJSON(t, "/api/prices/refresh", nil)
		elapsed := time.Since(start)
		require.Equal(t, http.StatusOK, status, "refresh: %s", apiError(body))
		var got liveRefreshResponse
		decode(t, body, &got)
		stored, err := db.PriceHistory(t.Context())
		require.NoError(t, err)
		rows := 0
		for _, p := range stored {
			rows += len(p)
		}
		t.Logf("%s refresh: %s, %d current prices, %d current failures, %d history requests, "+
			"%d daily closes stored, %d history failures, stop on HTTP 429: %t, %d Instruments with prices, %d rows in prices",
			run, elapsed.Round(time.Millisecond), got.Refreshed, len(got.Failures), provider.historyRequests-before,
			got.HistoryStored, len(got.HistoryFailures), got.TooManyRequests, len(stored), rows)
		t.Logf("%s refresh: history failures by kind: %v", run, failureKinds(got.HistoryFailures))
	}
	t.Logf("HTTP 429 answers: %d", provider.tooManyRequests)
}

// failureKinds returns the number of failures of each kind. A kind is the error text with no ticker,
// so that the log shows no personal data.
func failureKinds(failures []refreshFailure) map[string]int {
	kinds := map[string]int{}
	for _, f := range failures {
		switch {
		case strings.HasPrefix(f.Error, "the price is in "):
			// For example "the price is in USD, but the Instrument is quoted in GBP".
			kinds[f.Error]++
		case strings.Contains(f.Error, "status 404"):
			kinds["HTTP 404"]++
		default:
			kinds["other"]++
		}
	}
	return kinds
}
