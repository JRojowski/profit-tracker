package xtbtest_test

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

var (
	closedHeader = []string{
		"Instrument", "Ticker", "Category", "Type", "Volume", "Open Price", "Open Time (UTC)",
		"Close Price", "Close Time (UTC)", "Product", "Profit/Loss", "Gross Profit", "Purchase Value",
		"Sale Value", "Stop Loss", "Take Profit", "Commission", "Margin", "Swap", "Rollover",
		"Open Conversion Rate", "Close Conversion Rate", "Close Origin", "Position ID", "Comment",
	}
	cashHeader = []string{
		"Type", "Instrument", "Ticker", "Category", "Time", "Amount", "ID", "Comment", "Product", "Position ID",
	}
	openHeader = []string{
		"Product", "Instrument/Position", "Ticker", "Category", "Type", "Volume", "Value", "Current price",
		"Open price", "Open time (UTC)", "Stop Loss", "Take Profit", "Net Profit %", "Net Profit",
		"Gross Profit", "Margin", "Open Commission", "Swap", "Rollover",
	}
)

// adrExampleB is ADR 0002 Example B: two Fills with different IDs, merged into one Lot.
func adrExampleB() xtbtest.Export {
	buyTime := time.Date(2024, 3, 4, 9, 30, 0, 0, time.UTC)
	sellTime := time.Date(2025, 6, 2, 14, 15, 0, 0, time.UTC)
	return xtbtest.Export{
		AccountNumber: "50000001",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "LSMC.DE", Time: sellTime, Amount: "198.43", ID: "300", Comment: "CLOSE BUY 0.4396 @ 107.040", PositionID: "1851988998"},
			{Type: "Stock purchase", Ticker: "LSMC.DE", Time: buyTime.Add(2 * time.Second), Amount: "-204.59", ID: "200", Comment: "OPEN BUY 1/1.6169 @ 47.705", PositionID: "1851989133"},
			{Type: "Stock purchase", Ticker: "LSMC.DE", Time: buyTime, Amount: "-126.35", ID: "100", Comment: "OPEN BUY 0.6169/1.6169 @ 47.755", PositionID: "1851988998"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "LSMC.DE", Volume: "0.4396", OpenTime: buyTime, CloseTime: sellTime, ProfitLoss: "108.45", PositionID: "1851988998"},
		},
		OpenLots: []xtbtest.OpenLot{
			{Ticker: "LSMC.DE", PositionID: "1851988998", Volume: "1.1773", OpenTime: buyTime},
		},
	}
}

func TestBuild_WritesTheThreeSheetsInTheXTBLayout(t *testing.T) {
	t.Parallel()

	f := open(t, xtbtest.Build(t, adrExampleB()))

	assert.Equal(t, []string{"Closed Positions", "Cash Operations", "Open Positions"}, f.GetSheetList())
	assert.Equal(t, []string{"Account number", "50000001"}, row(t, f, "Closed Positions", 1))
	assert.Equal(t, []string{"Closed Positions"}, row(t, f, "Closed Positions", 2))
	assert.Equal(t, "Date from (UTC)", cell(t, f, "Closed Positions", "A3"))
	assert.Equal(t, "Date to (UTC)", cell(t, f, "Closed Positions", "A4"))
	assert.Equal(t, closedHeader, row(t, f, "Closed Positions", 5))
	assert.Equal(t, []string{"Account number", "50000001"}, row(t, f, "Cash Operations", 1))
	assert.Equal(t, []string{"Cash Operations"}, row(t, f, "Cash Operations", 2))
	assert.Equal(t, cashHeader, row(t, f, "Cash Operations", 5))
	assert.Equal(t, []string{"Account number", "50000001"}, row(t, f, "Open Positions", 1))
	assert.Equal(t, []string{"Open Positions"}, row(t, f, "Open Positions", 2))
	assert.Equal(t, "Data as of report generated", cell(t, f, "Open Positions", "A3"))
	assert.Equal(t, []string{"Product", "Metric", "Amount", "Currency"}, row(t, f, "Open Positions", 4))
	assert.Equal(t, "Note", cell(t, f, "Open Positions", "A8"))
	assert.Equal(t, openHeader, row(t, f, "Open Positions", 9))
}

func TestBuild_CashOperations(t *testing.T) {
	t.Parallel()

	f := open(t, xtbtest.Build(t, adrExampleB()))

	assert.Equal(t, []string{
		"Stock sell", "LSMC.DE", "LSMC.DE", "STOCK", cell(t, f, "Cash Operations", "E6"),
		"198.43", "300", "CLOSE BUY 0.4396 @ 107.040", "My Trades", "1851988998",
	}, row(t, f, "Cash Operations", 6))
	assert.Equal(t, []string{"Stock purchase", "LSMC.DE", "LSMC.DE", "STOCK"}, row(t, f, "Cash Operations", 8)[:4])
	assert.Equal(t, "-126.35", cell(t, f, "Cash Operations", "F8"))
	assert.Equal(t, time.Date(2024, 3, 4, 9, 30, 0, 0, time.UTC), serialTime(t, f, "Cash Operations", "E8"))
	assertNumberCell(t, f, "Cash Operations", "F8")

	assert.Equal(t, "Total", cell(t, f, "Cash Operations", "A9"))
	assert.Equal(t, "-132.51", cell(t, f, "Cash Operations", "F9"), "the default Total is the sum of the amounts")
	assert.Len(t, rows(t, f, "Cash Operations"), 9)
}

func TestBuild_CashTotalFromTheTest(t *testing.T) {
	t.Parallel()
	export := adrExampleB()
	export.CashTotal = "-100.00"

	f := open(t, xtbtest.Build(t, export))

	assert.Equal(t, "-100.00", cell(t, f, "Cash Operations", "F9"))
}

func TestBuild_ClosedPositions(t *testing.T) {
	t.Parallel()

	f := open(t, xtbtest.Build(t, adrExampleB()))

	got := row(t, f, "Closed Positions", 6)
	require.Len(t, got, 24, "the Comment column is empty")
	assert.Equal(t, []string{"LSMC.DE", "LSMC.DE", "STOCK", "BUY", "0.4396"}, got[:5])
	assert.Equal(t, time.Date(2024, 3, 4, 9, 30, 0, 0, time.UTC), serialTime(t, f, "Closed Positions", "G6"))
	assert.Equal(t, time.Date(2025, 6, 2, 14, 15, 0, 0, time.UTC), serialTime(t, f, "Closed Positions", "I6"))
	assert.Equal(t, []string{"My Trades", "108.45", "108.45"}, got[9:12])
	assert.Equal(t, "1.851988998E9", got[23], "the export writes the Position ID as a number")
	assertNumberCell(t, f, "Closed Positions", "X6")

	assert.Equal(t, "Profit/loss", cell(t, f, "Closed Positions", "A7"))
	assert.Equal(t, "108.45", cell(t, f, "Closed Positions", "K7"))
	assert.Equal(t, "108.45", cell(t, f, "Closed Positions", "L7"))
}

func TestBuild_OpenPositionsGroupLotsByInstrument(t *testing.T) {
	t.Parallel()
	export := adrExampleB()
	export.OpenLots = append(export.OpenLots,
		xtbtest.OpenLot{Ticker: "VWRA.UK", Instrument: "FTSE All-World", Category: "ETF", PositionID: "2875780059", Volume: "0.524"},
		xtbtest.OpenLot{Ticker: "LSMC.DE", PositionID: "1900000001", Volume: "2"},
	)

	f := open(t, xtbtest.Build(t, export))

	assert.Equal(t, []string{"My Trades", "LSMC.DE", "LSMC.DE", "STOCK", "", "3.1773"}, row(t, f, "Open Positions", 10))
	assert.Equal(t, []string{"My Trades", "1851988998", "LSMC.DE", "", "BUY", "1.1773"}, row(t, f, "Open Positions", 11)[:6])
	assert.Equal(t, time.Date(2024, 3, 4, 9, 30, 0, 0, time.UTC), serialTime(t, f, "Open Positions", "J11"))
	assert.Equal(t, []string{"My Trades", "1900000001", "LSMC.DE", "", "BUY", "2"}, row(t, f, "Open Positions", 12)[:6])
	assert.Equal(t, []string{"My Trades", "FTSE All-World", "VWRA.UK", "ETF", "", "0.524"}, row(t, f, "Open Positions", 13))
	assert.Equal(t, []string{"My Trades", "2875780059", "VWRA.UK", "", "BUY", "0.524"}, row(t, f, "Open Positions", 14)[:6])
	assert.Len(t, rows(t, f, "Open Positions"), 14)
}

func TestExport_FileName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "PLN_50000001_2006-01-01_2026-10-22.xlsx", adrExampleB().FileName())
	assert.Equal(t, "USD_12345678_2006-01-01_2026-10-22.xlsx", xtbtest.Export{}.FileName())
}

func open(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })
	return f
}

// rows returns the raw cell values of a sheet, as a reader sees them.
func rows(t *testing.T, f *excelize.File, sheet string) [][]string {
	t.Helper()
	got, err := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	return got
}

func row(t *testing.T, f *excelize.File, sheet string, number int) []string {
	t.Helper()
	all := rows(t, f, sheet)
	require.GreaterOrEqual(t, len(all), number, "sheet %s has no row %d", sheet, number)
	return all[number-1]
}

func cell(t *testing.T, f *excelize.File, sheet, name string) string {
	t.Helper()
	got, err := f.GetCellValue(sheet, name, excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	return got
}

// assertNumberCell checks that a cell is a number cell. In OOXML, a cell with no type is a number.
func assertNumberCell(t *testing.T, f *excelize.File, sheet, name string) {
	t.Helper()
	got, err := f.GetCellType(sheet, name)
	require.NoError(t, err)
	assert.Contains(t, []excelize.CellType{excelize.CellTypeUnset, excelize.CellTypeNumber}, got, "%s!%s must be a number cell", sheet, name)
}

// serialTime reads an Excel serial date and rounds it to the second.
func serialTime(t *testing.T, f *excelize.File, sheet, name string) time.Time {
	t.Helper()
	serial, err := strconv.ParseFloat(cell(t, f, sheet, name), 64)
	require.NoError(t, err)
	got, err := excelize.ExcelDateToTime(serial, false)
	require.NoError(t, err)
	return got.Round(time.Second).UTC()
}

func TestBuild_SerialTimesReadBackToTheSameSecond(t *testing.T) {
	t.Parallel()
	times := []time.Time{
		time.Date(2026, 10, 22, 8, 41, 37, 0, time.UTC),
		time.Date(2026, 10, 22, 23, 59, 59, 0, time.UTC),
		time.Date(2024, 2, 29, 0, 0, 1, 0, time.UTC),
		time.Date(2019, 7, 15, 13, 7, 11, 0, time.UTC),
		time.Date(2026, 3, 1, 9, 30, 0, 0, time.UTC),
	}
	export := xtbtest.Export{}
	for _, at := range times {
		export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{Type: "Deposit", Time: at, Amount: "1"})
	}

	f := open(t, xtbtest.Build(t, export))

	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	for i, want := range times {
		name := fmt.Sprintf("E%d", 6+i)
		serial, err := decimal.NewFromString(cell(t, f, "Cash Operations", name))
		require.NoError(t, err)

		seconds := serial.Mul(decimal.NewFromInt(86400))
		wantSeconds := decimal.NewFromInt(int64(want.Sub(epoch) / time.Second))
		assert.True(t, seconds.Sub(wantSeconds).Abs().LessThan(decimal.New(1, -9)),
			"%s: serial %s is %s s, want %s s", name, serial, seconds, wantSeconds)
		got := epoch.Add(time.Duration(seconds.Round(0).IntPart()) * time.Second)
		assert.Equal(t, want, got, "%s: serial %s", name, serial)
	}
}
