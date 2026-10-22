package xtb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

func TestRead_PurchaseComments_GiveExactVolumeAndPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                             string
		comment                          string
		wantVolume, wantOrder, wantPrice string
	}{
		{name: "whole volume", comment: "OPEN BUY 2 @ 58.29", wantVolume: "2", wantOrder: "2", wantPrice: "58.29"},
		{name: "whole volume with ticker", comment: "OPEN BUY PENG.US 2 @ 58.29", wantVolume: "2", wantOrder: "2", wantPrice: "58.29"},
		{name: "partial fill", comment: "OPEN BUY 0.6169/1.6169 @ 47.755", wantVolume: "0.6169", wantOrder: "1.6169", wantPrice: "47.755"},
		{name: "partial fill with ticker", comment: "OPEN BUY FPS.US 1/1.27 @ 39.24", wantVolume: "1", wantOrder: "1.27", wantPrice: "39.24"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			at := time.Date(2026, 3, 1, 9, 30, 1, 0, time.UTC)
			data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
				{Type: "Stock purchase", Ticker: "PENG.US", Time: at, Amount: "-116.58", ID: "77", Comment: tc.comment, PositionID: "2000000002"},
			}})

			got, err := xtb.Read(data)

			require.NoError(t, err)
			require.Len(t, got.CashOperations, 1)
			op := got.CashOperations[0]
			require.NotNil(t, op.Purchase)
			assert.Equal(t, tc.wantVolume, op.Purchase.Volume.String())
			assert.Equal(t, tc.wantOrder, op.Purchase.OrderVolume.String())
			assert.Equal(t, tc.wantPrice, op.Purchase.Price.String())
			assert.Equal(t, "-116.58", op.Amount.StringFixed(2))
			assert.Equal(t, at, op.Time)
			assert.Equal(t, "77", op.ID)
			assert.Equal(t, "2000000002", op.PositionID)
			assert.Equal(t, 6, op.Row)
			assert.Equal(t, xtbtest.DefaultAccountNumber, got.AccountNumber)
			assert.Equal(t, "-116.58", got.Total.StringFixed(2))
		})
	}
}

func TestCurrencyFromFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, fileName, want string
	}{
		{name: "USD prefix", fileName: "USD_12345678_2006-01-01_2026-10-22.xlsx", want: "USD"},
		{name: "PLN prefix", fileName: "PLN_50000001_2006-01-01_2026-09-23.xlsx", want: "PLN"},
		{name: "IKZE prefix is not a currency", fileName: "IKZE_54000001_2006-01-01_2026-09-23.xlsx", want: ""},
		{name: "no prefix", fileName: "export.xlsx", want: ""},
		{name: "lowercase prefix", fileName: "usd_12345678.xlsx", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, xtb.CurrencyFromFileName(tc.fileName))
		})
	}
}
