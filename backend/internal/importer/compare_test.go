package importer

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// The reader takes the volume and the price from the Comment, so no export file can change only
// the volume or only the price. This test calls compare directly for these fields.
func TestCompare_VolumeOrPrice_IsComparedAsDecimal(t *testing.T) {
	t.Parallel()

	value := func(s string) decimal.NullDecimal { return decimal.NewNullDecimal(decimal.RequireFromString(s)) }
	null := decimal.NullDecimal{}
	tests := []struct {
		name             string
		stored, imported store.CashOperationData
		wantConflict     bool
	}{
		{name: "same volume and price", stored: data(value("10"), value("10.94")), imported: data(value("10"), value("10.94"))},
		{name: "same volume with other decimal places", stored: data(value("10"), value("10.94")), imported: data(value("10.000"), value("10.940"))},
		{name: "null equals null", stored: data(null, null), imported: data(null, null)},
		{name: "other volume", stored: data(value("10"), value("10.94")), imported: data(value("12"), value("10.94")), wantConflict: true},
		{name: "other price", stored: data(value("10"), value("10.94")), imported: data(value("10"), value("9.1167")), wantConflict: true},
		{name: "null volume and a volume", stored: data(null, value("10.94")), imported: data(value("10"), value("10.94")), wantConflict: true},
		{name: "a price and null price", stored: data(value("10"), value("10.94")), imported: data(value("10"), null), wantConflict: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := store.CashOperation{
				XTBID: "102", Type: tc.imported.Type, Time: tc.imported.Time, Amount: tc.imported.Amount, Comment: tc.imported.Comment,
				Ticker: tc.imported.Ticker, PositionID: tc.imported.PositionID, Volume: tc.imported.Volume, Price: tc.imported.Price,
			}

			got := compare([]store.CashOperation{op}, map[string]store.CashOperationData{"102": tc.stored})

			if tc.wantConflict {
				assert.Len(t, got.conflicts, 1)
				assert.Equal(t, 0, got.skipped)
			} else {
				assert.Empty(t, got.conflicts)
				assert.Equal(t, 1, got.skipped)
			}
		})
	}
}

// data returns the data of a purchase with the volume and the price. The other fields are fixed.
func data(volume, price decimal.NullDecimal) store.CashOperationData {
	return store.CashOperationData{
		Type: "Stock purchase", Time: time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("-109.40"),
		Comment: "OPEN BUY 10 @ 10.94", Ticker: "FPS.US", PositionID: "2000000001", Volume: volume, Price: price,
	}
}
