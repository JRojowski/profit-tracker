package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// TestSaveBenchmarkPrices_SecondSave_KeepsStoredClosesAndJoinsTheRanges checks that a stored close of a Benchmark
// never changes and that the stored range grows with each save.
func TestSaveBenchmarkPrices_SecondSave_KeepsStoredClosesAndJoinsTheRanges(t *testing.T) {
	t.Parallel()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "profit-tracker.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	day := func(d int) time.Time { return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC) }
	fetched := time.Date(2026, time.October, 23, 9, 0, 0, 0, time.UTC)
	price := func(d int, closePrice string) store.BenchmarkPrice {
		return store.BenchmarkPrice{
			BenchmarkID: "sp500", Date: day(d), Close: decimal.RequireFromString(closePrice), Currency: "USD",
			Source: "fake", FetchedAt: fetched,
		}
	}

	require.NoError(t, db.SaveBenchmarkPrices(t.Context(),
		store.BenchmarkPriceRange{BenchmarkID: "sp500", FirstDay: day(5), LastDay: day(10), FetchedAt: fetched},
		[]store.BenchmarkPrice{price(5, "100"), price(9, "101")}))
	require.NoError(t, db.SaveBenchmarkPrices(t.Context(),
		store.BenchmarkPriceRange{BenchmarkID: "sp500", FirstDay: day(1), LastDay: day(4), FetchedAt: fetched},
		[]store.BenchmarkPrice{price(2, "98"), price(5, "999")}))

	prices, err := db.BenchmarkPrices(t.Context())
	require.NoError(t, err)
	got := []string{}
	for _, p := range prices["sp500"] {
		got = append(got, p.Date.Format(time.DateOnly)+"="+p.Close.String()+" "+p.Currency)
	}
	assert.Equal(t, []string{"2026-10-02=98 USD", "2026-10-05=100 USD", "2026-10-09=101 USD"}, got,
		"the second save does not replace the close of 2026-10-05")
	ranges, err := db.BenchmarkPriceRanges(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]store.BenchmarkPriceRange{
		"sp500": {BenchmarkID: "sp500", FirstDay: day(1), LastDay: day(10), FetchedAt: fetched},
	}, ranges)
}
