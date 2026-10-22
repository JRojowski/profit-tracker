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

// An unused Instrument row is not visible at the HTTP API, so this test reads the table.
func TestSaveImport_KeepExistingWithOtherTicker_AddsNoInstrument(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "profit-tracker.db")
	db, err := store.Open(ctx, path)
	require.NoError(t, err)

	at := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	stored := store.CashOperation{
		XTBID: "102", Type: "Stock purchase", Ticker: "FPS.US", InstrumentName: "FPS.US", Category: "STOCK",
		Time: at, Amount: decimal.RequireFromString("-109.40"), Comment: "OPEN BUY 10 @ 10.94", PositionID: "2000000001",
		Volume: decimal.NewNullDecimal(decimal.RequireFromString("10")), Price: decimal.NewNullDecimal(decimal.RequireFromString("10.94")), IsFill: true,
	}
	account := store.Account{AccountNumber: "56000001", Name: "XTB USD", Type: "regular", Currency: "USD"}
	firstID, err := db.CreatePreview(ctx, store.NewPreview{AccountNumber: account.AccountNumber, FileName: "first.xlsx", CreatedAt: at})
	require.NoError(t, err)
	_, err = db.SaveImport(ctx, store.SaveImport{ImportID: firstID, Account: account, ImportedAt: at, CashOperations: []store.CashOperation{stored}})
	require.NoError(t, err)

	changed := stored
	changed.Ticker, changed.InstrumentName, changed.Comment = "LSMC.DE", "LSMC.DE", "OPEN BUY LSMC.DE 10 @ 10.94"
	secondID, err := db.CreatePreview(ctx, store.NewPreview{
		AccountNumber: account.AccountNumber, FileName: "second.xlsx", CreatedAt: at,
		Conflicts: []store.NewImportConflict{{Kind: "changed-data", XTBID: "102", Existing: stored.Data(), New: changed.Data()}},
	})
	require.NoError(t, err)
	conflicts, err := db.ImportConflicts(ctx, secondID)
	require.NoError(t, err)
	require.Len(t, conflicts, 1)
	_, err = db.DecideImportConflict(ctx, secondID, conflicts[0].ID, store.DecisionKeepExisting)
	require.NoError(t, err)

	saved, err := db.SaveImport(ctx, store.SaveImport{ImportID: secondID, Account: account, ImportedAt: at, CashOperations: []store.CashOperation{changed}})
	require.NoError(t, err)
	assert.Equal(t, 1, saved.SkippedCashOperations)
	require.NoError(t, db.Close())

	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, raw.Close()) })
	var count int
	require.NoError(t, raw.QueryRowContext(ctx, "SELECT COUNT(*) FROM instruments WHERE ticker = 'LSMC.DE'").Scan(&count))
	assert.Zero(t, count, "keep-existing writes nothing")
}
