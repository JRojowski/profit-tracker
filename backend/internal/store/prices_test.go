package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

func TestOpen_InstrumentsFromOlderSchema_GetQuoteCurrencyFromTickerSuffix(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "profit-tracker.db")
	// The database of a binary before the prices: migration 5 is the last one, and the Instruments exist.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.ExecContext(t.Context(), `
		CREATE TABLE instruments (
			id       INTEGER PRIMARY KEY,
			ticker   TEXT    NOT NULL UNIQUE,
			name     TEXT    NOT NULL,
			category TEXT    NOT NULL
		) STRICT;
		INSERT INTO instruments (ticker, name, category) VALUES
			('AAPL.US', 'Apple', 'STOCK'), ('VUSA.UK', 'Vanguard', 'ETF'), ('PKO.PL', 'PKO', 'STOCK'),
			('SAP.DE', 'SAP', 'STOCK'), ('GHI.FR', 'GHI', 'STOCK');
		PRAGMA user_version = 5;`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := store.Open(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	got, err := db.Instruments(t.Context())

	require.NoError(t, err)
	assert.Equal(t, map[string]store.Instrument{
		"AAPL.US": {Ticker: "AAPL.US", Name: "Apple", QuoteCurrency: "USD"},
		"VUSA.UK": {Ticker: "VUSA.UK", Name: "Vanguard", QuoteCurrency: "GBP"},
		"PKO.PL":  {Ticker: "PKO.PL", Name: "PKO", QuoteCurrency: "PLN"},
		"SAP.DE":  {Ticker: "SAP.DE", Name: "SAP", QuoteCurrency: "EUR"},
		"GHI.FR":  {Ticker: "GHI.FR", Name: "GHI"},
	}, got, "the suffix gives a quote currency that is not manual, and no Instrument has a manual price ticker")
}

func TestLastPrices_SavedPrices_ReturnsNewestDateWithAllFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "profit-tracker.db")
	db, err := store.Open(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	// A price needs its Instrument. An Import adds the Instruments; this test adds one with SQL.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.ExecContext(t.Context(), `INSERT INTO instruments (ticker, name, category, quote_currency) VALUES ('AAPL.US', 'Apple', 'STOCK', 'USD')`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())
	older := store.Price{
		Ticker: "AAPL.US", Date: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), Close: decimal.RequireFromString("120.5"),
		Currency: "USD", Source: "yahoo", FetchedAt: time.Date(2026, 10, 22, 7, 0, 0, 0, time.UTC),
	}
	newer := store.Price{
		Ticker: "AAPL.US", Date: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), Close: decimal.RequireFromString("125.25"),
		Currency: "USD", Source: "fake", FetchedAt: time.Date(2026, 10, 23, 7, 0, 0, 0, time.UTC),
	}
	require.NoError(t, db.SavePrice(t.Context(), newer))
	require.NoError(t, db.SavePrice(t.Context(), older))

	got, err := db.LastPrices(t.Context())

	require.NoError(t, err)
	require.Contains(t, got, "AAPL.US")
	p := got["AAPL.US"]
	assert.Equal(t, newer.Date, p.Date)
	assert.Equal(t, "125.25", p.Close.String())
	assert.Equal(t, "USD", p.Currency)
	assert.Equal(t, "fake", p.Source)
	assert.Equal(t, newer.FetchedAt, p.FetchedAt)
}
