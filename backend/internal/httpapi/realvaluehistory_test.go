package httpapi_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// TestRealExports_ValueHistory imports the real exports, stores a synthetic daily price for each Instrument and
// a synthetic Exchange Rate for each day, and measures GET /api/accounts/{id}/value-history.
// The synthetic data has the size of a full history, but no real price. The log shows only the times and
// the number of points, because the exports contain personal data.
func TestRealExports_ValueHistory(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv := startServer(t)
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}
	storeSyntheticPrices(t, srv.db, srv.clock.Now())

	for i, export := range exports {
		path := fmt.Sprintf("/api/accounts/%d/value-history", accountIDs[i])
		var got valueHistoryResponse
		srv.getRealJSON(t, path, &got) // The first request warms the caches of SQLite.
		times := make([]time.Duration, 0, 5)
		for range 5 {
			start := time.Now()
			srv.getRealJSON(t, path, &got)
			times = append(times, time.Since(start))
		}
		slices.Sort(times)
		t.Logf("export %s: %d points, median %s, max %s", export.label, len(got.Points), times[len(times)/2], times[len(times)-1])

		require.NotEmpty(t, got.Points, "export %s", export.label)
		first, err := time.Parse(time.DateOnly, got.Points[0].Date)
		require.NoError(t, err)
		last, err := time.Parse(time.DateOnly, got.Points[len(got.Points)-1].Date)
		require.NoError(t, err)
		assert.Len(t, got.Points, int(last.Sub(first)/(24*time.Hour))+1, "export %s: one point for each calendar day", export.label)
		summary := srv.summaryReturns(t, accountIDs[i])
		var unpriced struct {
			UnpricedHoldings int `json:"unpricedHoldings"`
		}
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountIDs[i]), &unpriced)
		if unpriced.UnpricedHoldings == 0 {
			assert.Equal(t, summary.Value, got.Points[len(got.Points)-1].Value, "export %s: the point of today is the value in the hero", export.label)
		}
	}
}

// storeSyntheticPrices stores a daily price of 1 in the Quote Currency for each Instrument.
// The prices go from the day of the first Fill of the Instrument to yesterday.
// It also stores an Exchange Rate of 4 for USD, EUR and GBP for each day of the last 10 years.
func storeSyntheticPrices(t *testing.T, db *store.Store, now time.Time) {
	t.Helper()

	firstFills, err := db.FirstFillTimes(t.Context())
	require.NoError(t, err)
	instruments, err := db.Instruments(t.Context())
	require.NoError(t, err)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	one := decimal.NewFromInt(1)
	for ticker, first := range firstFills {
		qc := instruments[ticker].QuoteCurrency
		if qc == "" {
			continue
		}
		from := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, time.UTC)
		var prices []store.Price
		for day := from; day.Before(today); day = day.AddDate(0, 0, 1) {
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				continue
			}
			prices = append(prices, store.Price{Ticker: ticker, Date: day, Close: one, Currency: qc, Source: "synthetic", FetchedAt: now})
		}
		asked := store.PriceRange{Ticker: ticker, FirstDay: from, LastDay: today.AddDate(0, 0, -1), FetchedAt: now}
		require.NoError(t, db.SaveDailyPrices(t.Context(), asked, prices))
	}

	from := today.AddDate(-10, 0, 0)
	rate := decimal.NewFromInt(4)
	for _, currency := range []string{"USD", "EUR", "GBP"} {
		var rates []store.ExchangeRate
		for day := from; !day.After(today); day = day.AddDate(0, 0, 1) {
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				continue
			}
			rates = append(rates, store.ExchangeRate{Currency: currency, Date: day, Rate: rate, TableNumber: "synthetic", FetchedAt: now})
		}
		asked := store.ExchangeRateRange{Currency: currency, FirstDay: from, LastDay: today, FetchedAt: now}
		require.NoError(t, db.SaveExchangeRates(t.Context(), asked, rates))
	}
}
