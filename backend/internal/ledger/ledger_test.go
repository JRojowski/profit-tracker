package ledger_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// The import always stores a price, so the HTTP API cannot give an operation with no price.
// This test covers the missing prices at the Ledger.
func TestCalculate_ClosedPositionWithMissingPrices_HasNoPrices(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	d := decimal.RequireFromString
	price := func(s string) decimal.NullDecimal { return decimal.NewNullDecimal(d(s)) }

	tests := []struct {
		name          string
		fills         []ledger.Fill
		sellPrice     decimal.NullDecimal
		wantOpenPrice *string
		wantClose     *string
	}{
		{
			name:          "Fill with no price",
			fills:         []ledger.Fill{{XTBID: "1", Time: at, Volume: d("1"), Price: price("10"), Amount: d("-10")}, {XTBID: "2", Time: at, Volume: d("1"), Amount: d("-30")}},
			sellPrice:     price("40"),
			wantOpenPrice: nil,
			wantClose:     new("40"),
		},
		{
			name:          "sell operation with no price",
			fills:         []ledger.Fill{{XTBID: "1", Time: at, Volume: d("2"), Price: price("10"), Amount: d("-20")}},
			wantOpenPrice: new("10"),
			wantClose:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var volume decimal.Decimal
			for _, f := range tc.fills {
				volume = volume.Add(f.Volume)
			}
			state, err := ledger.Calculate(ledger.Ledger{
				Currency: "PLN",
				CashOperations: []ledger.CashOperation{
					{XTBID: "9", Type: ledger.TypeStockSell, Time: at.Add(time.Hour), Amount: d("50"), PositionID: "P1", Volume: volume, Price: tc.sellPrice},
				},
				Lots: []ledger.Lot{{PositionID: "P1", Instrument: ledger.Instrument{Ticker: "ABC.PL"}, Fills: tc.fills}},
			})

			require.NoError(t, err)
			require.Len(t, state.ClosedPositions, 1)
			got := state.ClosedPositions[0]
			assert.Equal(t, tc.wantOpenPrice, nullString(got.OpenPrice))
			assert.Equal(t, tc.wantClose, nullString(got.ClosePrice))
		})
	}
}

func nullString(p decimal.NullDecimal) *string {
	if !p.Valid {
		return nil
	}
	return new(p.Decimal.String())
}
