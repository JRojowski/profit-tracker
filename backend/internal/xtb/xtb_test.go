package xtb_test

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

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

func TestRead_SaleComments_GiveClosedVolumeAndPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                             string
		comment                          string
		wantVolume, wantOrder, wantPrice string
	}{
		{name: "part of the order", comment: "CLOSE BUY 9/10 @ 10.26", wantVolume: "9", wantOrder: "10", wantPrice: "10.26"},
		{name: "full order", comment: "CLOSE BUY 18 @ 26.90", wantVolume: "18", wantOrder: "18", wantPrice: "26.9"},
		{name: "fractional part with ticker", comment: "CLOSE BUY FPS.US 0.1504/0.426 @ 256.60", wantVolume: "0.1504", wantOrder: "0.426", wantPrice: "256.6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
				{Type: "Stock sell", Ticker: "FPS.US", Amount: "92.34", ID: "78", Comment: tc.comment, PositionID: "2000000001"},
			}})

			got, err := xtb.Read(data)

			require.NoError(t, err)
			require.Len(t, got.CashOperations, 1)
			op := got.CashOperations[0]
			assert.Equal(t, xtb.TypeStockSell, op.Type)
			assert.Nil(t, op.Purchase)
			require.NotNil(t, op.Sale)
			assert.Equal(t, tc.wantVolume, op.Sale.Volume.String())
			assert.Equal(t, tc.wantOrder, op.Sale.OrderVolume.String())
			assert.Equal(t, tc.wantPrice, op.Sale.Price.String())
			assert.Equal(t, "2000000001", op.PositionID)
		})
	}
}

func TestRead_SaleWithUnknownComment_ReturnsInvalidRow(t *testing.T) {
	t.Parallel()
	data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
		{Type: "Stock sell", Ticker: "FPS.US", Amount: "92.34", Comment: "SELL SOME", PositionID: "2000000001"},
	}})

	_, err := xtb.Read(data)

	require.ErrorIs(t, err, xtb.ErrInvalidRow)
	assert.Contains(t, err.Error(), "row 6")
	assert.Contains(t, err.Error(), `"SELL SOME"`)
}

func TestRead_ClosedPositions_GivePositionIDVolumeCloseTimeAndProfit(t *testing.T) {
	t.Parallel()
	closedAt := time.Date(2026, 3, 2, 14, 15, 7, 0, time.UTC)
	data := xtbtest.Build(t, xtbtest.Export{ClosedPositions: []xtbtest.ClosedPosition{
		{Ticker: "FPS.US", Volume: "9", CloseTime: closedAt, ProfitLoss: "-6.12", PositionID: "2636532207"},
		{Ticker: "AAPL.US", Volume: "0.1504", CloseTime: closedAt.Add(time.Hour), ProfitLoss: "57.08", PositionID: "1000"},
	}})

	got, err := xtb.Read(data)

	require.NoError(t, err)
	require.Len(t, got.ClosedPositions, 2)
	first := got.ClosedPositions[0]
	assert.Equal(t, 6, first.Row)
	assert.Equal(t, "FPS.US", first.Ticker)
	assert.Equal(t, "2636532207", first.PositionID, "the export writes the ID as a Java double")
	assert.Equal(t, "9", first.Volume.String())
	assert.Equal(t, closedAt, first.CloseTime)
	assert.Equal(t, "-6.12", first.ProfitLoss.String())
	second := got.ClosedPositions[1]
	assert.Equal(t, "1000", second.PositionID)
	assert.Equal(t, "0.1504", second.Volume.String())
	assert.Equal(t, "57.08", second.ProfitLoss.String())
}

func TestRead_ZeroVolumeInComment_ReturnsInvalidRow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, typ, comment string
	}{
		{name: "purchase with zero volume", typ: "Stock purchase", comment: "OPEN BUY 0 @ 10.00"},
		{name: "purchase with zero fractional volume", typ: "Stock purchase", comment: "OPEN BUY 0.0000 @ 10.00"},
		{name: "purchase with zero order volume", typ: "Stock purchase", comment: "OPEN BUY 1/0 @ 10.00"},
		{name: "sale with zero volume", typ: "Stock sell", comment: "CLOSE BUY 0 @ 10.00"},
		{name: "sale with zero volume and ticker", typ: "Stock sell", comment: "CLOSE BUY FPS.US 0/10 @ 10.00"},
		{name: "sale with zero order volume", typ: "Stock sell", comment: "CLOSE BUY 9/0 @ 10.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
				{Type: tc.typ, Ticker: "FPS.US", Amount: "10.00", Comment: tc.comment, PositionID: "2000000001"},
			}})

			_, err := xtb.Read(data)

			require.ErrorIs(t, err, xtb.ErrInvalidRow)
			assert.Contains(t, err.Error(), "row 6")
			assert.Contains(t, err.Error(), strconv.Quote(tc.comment))
			assert.Contains(t, err.Error(), "zero")
		})
	}
}

func TestRead_OpenPositions_GiveLotIDsAndVolumes(t *testing.T) {
	t.Parallel()
	data := xtbtest.Build(t, xtbtest.Export{OpenLots: []xtbtest.OpenLot{
		{Ticker: "LSMC.DE", PositionID: "1851988998", Volume: "1.1773", Value: "100.00"},
		{Ticker: "FPS.US", PositionID: "2860602916", Volume: "1.27", Value: "50.00"},
		{Ticker: "LSMC.DE", PositionID: "1851990000", Volume: "2", Value: "170.00"},
	}})

	got, err := xtb.Read(data)

	require.NoError(t, err)
	assert.Equal(t, []xtb.OpenLot{
		{Row: 11, Ticker: "LSMC.DE", PositionID: "1851988998", Volume: decimal.RequireFromString("1.1773")},
		{Row: 12, Ticker: "LSMC.DE", PositionID: "1851990000", Volume: decimal.RequireFromString("2")},
		{Row: 14, Ticker: "FPS.US", PositionID: "2860602916", Volume: decimal.RequireFromString("1.27")},
	}, got.OpenLots, "the group rows 10 and 13 are not Lots")
}

func TestRead_OpenPositionIDAsJavaDouble_GivesPlainID(t *testing.T) {
	t.Parallel()
	data := xtbtest.Build(t, xtbtest.Export{OpenLots: []xtbtest.OpenLot{
		{Ticker: "FPS.US", PositionID: "2.860602916E9", Volume: "1.27"},
	}})

	got, err := xtb.Read(data)

	require.NoError(t, err)
	require.Len(t, got.OpenLots, 1)
	assert.Equal(t, "2860602916", got.OpenLots[0].PositionID)
	assert.Equal(t, "1.27", got.OpenLots[0].Volume.String())
}

func TestRead_InvalidOpenPositionRow_ReturnsInvalidOpenPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		lot        xtbtest.OpenLot
		wantInText string
	}{
		{name: "position ID that is not a number", lot: xtbtest.OpenLot{Ticker: "FPS.US", PositionID: "ABC", Volume: "1"}, wantInText: `"ABC"`},
		{name: "no position ID", lot: xtbtest.OpenLot{Ticker: "FPS.US", Volume: "1"}, wantInText: "position ID"},
		{name: "no volume", lot: xtbtest.OpenLot{Ticker: "FPS.US", PositionID: "2860602916"}, wantInText: "volume"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := xtbtest.Build(t, xtbtest.Export{OpenLots: []xtbtest.OpenLot{tc.lot}})

			_, err := xtb.Read(data)

			require.ErrorIs(t, err, xtb.ErrInvalidOpenPosition)
			assert.Contains(t, err.Error(), "row 11")
			assert.Contains(t, err.Error(), tc.wantInText)
		})
	}
}

func TestRead_NoOpenPositionsSheet_ReturnsNotExport(t *testing.T) {
	t.Parallel()
	f, err := excelize.OpenReader(bytes.NewReader(xtbtest.Build(t, xtbtest.Export{})))
	require.NoError(t, err)
	require.NoError(t, f.DeleteSheet(xtbtest.SheetOpenPositions))
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))

	_, err = xtb.Read(buf.Bytes())

	require.ErrorIs(t, err, xtb.ErrNotExport)
	assert.Contains(t, err.Error(), "Open Positions")
}

func TestRead_Transfer_GivesDirectionFromSign(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		amount string
		want   xtb.TransferDirection
	}{
		{name: "positive amount", amount: "10000.00", want: xtb.TransferIn},
		{name: "negative amount", amount: "-10000.00", want: xtb.TransferOut},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
				{Type: "IKZE deposit", Amount: tc.amount, ID: "4001", Comment: "Transfer operation on account with id 12345678"},
			}})

			got, err := xtb.Read(data)

			require.NoError(t, err)
			require.Len(t, got.CashOperations, 1)
			assert.Equal(t, xtb.TypeTransfer, got.CashOperations[0].Type)
			assert.Equal(t, tc.want, got.CashOperations[0].Transfer)
		})
	}
}

func TestRead_OtherTypes_HaveNoTransferDirection(t *testing.T) {
	t.Parallel()
	data := xtbtest.Build(t, xtbtest.Export{CashOperations: []xtbtest.CashOperation{
		{Type: "Deposit", Amount: "100.00", ID: "4002", Comment: "Blik deposit"},
	}})

	got, err := xtb.Read(data)

	require.NoError(t, err)
	require.Len(t, got.CashOperations, 1)
	assert.Empty(t, got.CashOperations[0].Transfer)
}
