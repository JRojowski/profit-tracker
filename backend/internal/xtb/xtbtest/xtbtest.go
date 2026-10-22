// Package xtbtest builds small xlsx files in the layout of an XTB export, for tests.
//
// A test gives only the fields that it needs. Build fills the other fields with defaults.
// Amounts, prices and volumes are decimal text, for example "-109.40". Build writes them as number cells,
// with the same text, so that a reader sees no float rounding.
package xtbtest

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Sheet names, in the order of a real export.
const (
	SheetClosedPositions = "Closed Positions"
	SheetCashOperations  = "Cash Operations"
	SheetOpenPositions   = "Open Positions"
)

// Defaults for the fields that a test does not give.
const (
	DefaultAccountNumber = "12345678"
	DefaultCurrency      = "USD"
	DefaultProduct       = "My Trades"
	DefaultCategory      = "STOCK"
)

var (
	// DefaultFrom is the start of the export period.
	DefaultFrom = time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)
	// DefaultTo is the end of the export period and the time of the report.
	DefaultTo = time.Date(2026, 10, 22, 8, 41, 37, 0, time.UTC)
	// defaultOperationTime is the time of the first Cash Operation with no time.
	defaultOperationTime = time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	// excelEpoch is day 0 of the Excel serial dates.
	excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
)

// Export is the content of one XTB export file.
type Export struct {
	// AccountNumber is the number in row 1 of each sheet. The default is DefaultAccountNumber.
	AccountNumber string
	// Currency is the prefix of the file name. The default is DefaultCurrency.
	Currency string
	// From and To are the export period. The defaults are DefaultFrom and DefaultTo.
	From, To time.Time

	// CashOperations are the rows of the Cash Operations sheet, in the given order.
	// A real export puts the newest operation first.
	CashOperations []CashOperation
	// CashTotal is the amount in the Total row. If it is empty, Build writes the sum of the amounts.
	CashTotal string

	// ClosedPositions are the rows of the Closed Positions sheet.
	ClosedPositions []ClosedPosition
	// OpenLots are the Lot rows of the Open Positions sheet.
	// Build writes one group row per Instrument, before its Lots, in the order of the first Lot.
	OpenLots []OpenLot
}

// CashOperation is one row of the Cash Operations sheet.
type CashOperation struct {
	// Type is the operation type, for example "Stock purchase".
	Type string
	// Instrument is the Instrument name. The default is the Ticker.
	Instrument string
	// Ticker is the broker ticker, for example "FPS.US". It is empty for a Deposit.
	Ticker string
	// Category is for example "STOCK" or "ETF". The default is DefaultCategory when Ticker is set.
	Category string
	// Time is the operation time in UTC. The default is a different minute for each row.
	Time time.Time
	// Amount is the decimal amount in the Account currency, for example "-109.40".
	Amount string
	// ID is the XTB ID of the operation. The default is a different number for each row.
	ID string
	// Comment is for example "OPEN BUY 10 @ 10.94".
	Comment string
	// Product is the default DefaultProduct.
	Product string
	// PositionID is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
}

// ClosedPosition is one row of the Closed Positions sheet: one closed part of a Lot.
type ClosedPosition struct {
	// Instrument is the Instrument name. The default is the Ticker.
	Instrument string
	Ticker     string
	// Category is the default DefaultCategory.
	Category string
	// Volume is the closed volume.
	Volume              string
	OpenPrice           string
	OpenTime            time.Time
	ClosePrice          string
	CloseTime           time.Time
	ProfitLoss          string
	PurchaseValue       string
	SaleValue           string
	OpenConversionRate  string
	CloseConversionRate string
	// Product is the default DefaultProduct.
	Product string
	// PositionID is the Lot ID. Build writes it as a number, like the export does.
	PositionID string
	Comment    string
}

// OpenLot is one Lot row of the Open Positions sheet.
type OpenLot struct {
	// Instrument is the Instrument name for the group row. The default is the Ticker.
	Instrument string
	Ticker     string
	// Category is the Category for the group row. The default is DefaultCategory.
	Category string
	// PositionID is the Lot ID.
	PositionID string
	// Volume is the open volume of the Lot.
	Volume       string
	Value        string
	CurrentPrice string
	OpenPrice    string
	OpenTime     time.Time
	// Product is the default DefaultProduct.
	Product string
}

// FileName returns the file name of a real export: <CURRENCY>_<accountNumber>_<from>_<to>.xlsx.
func (e Export) FileName() string {
	e = e.withDefaults()
	return fmt.Sprintf("%s_%s_%s_%s.xlsx", e.Currency, e.AccountNumber, e.From.Format(time.DateOnly), e.To.Format(time.DateOnly))
}

// Build returns the bytes of an xlsx file with the three sheets of an XTB export.
// It stops the test if a field holds a value that is not valid, for example an amount that is not a decimal.
func Build(tb testing.TB, e Export) []byte {
	tb.Helper()

	data, err := build(e.withDefaults())
	require.NoError(tb, err, "build xtb export fixture")
	return data
}

func (e Export) withDefaults() Export {
	e.AccountNumber = or(e.AccountNumber, DefaultAccountNumber)
	e.Currency = or(e.Currency, DefaultCurrency)
	if e.From.IsZero() {
		e.From = DefaultFrom
	}
	if e.To.IsZero() {
		e.To = DefaultTo
	}

	e.CashOperations = append([]CashOperation(nil), e.CashOperations...)
	for i := range e.CashOperations {
		op := &e.CashOperations[i]
		op.Instrument = or(op.Instrument, op.Ticker)
		if op.Ticker != "" {
			op.Category = or(op.Category, DefaultCategory)
		}
		if op.Time.IsZero() {
			op.Time = defaultOperationTime.Add(time.Duration(i) * time.Minute)
		}
		op.ID = or(op.ID, strconv.Itoa(1_000_000_000+i+1))
		op.Product = or(op.Product, DefaultProduct)
	}

	e.ClosedPositions = append([]ClosedPosition(nil), e.ClosedPositions...)
	for i := range e.ClosedPositions {
		p := &e.ClosedPositions[i]
		p.Instrument = or(p.Instrument, p.Ticker)
		p.Category = or(p.Category, DefaultCategory)
		p.Product = or(p.Product, DefaultProduct)
	}

	e.OpenLots = append([]OpenLot(nil), e.OpenLots...)
	for i := range e.OpenLots {
		l := &e.OpenLots[i]
		l.Instrument = or(l.Instrument, l.Ticker)
		l.Category = or(l.Category, DefaultCategory)
		l.Product = or(l.Product, DefaultProduct)
	}
	return e
}

func build(e Export) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	w := &writer{f: f}
	if err := f.SetSheetName(f.GetSheetName(0), SheetClosedPositions); err != nil {
		return nil, fmt.Errorf("name first sheet: %w", err)
	}
	for _, name := range []string{SheetCashOperations, SheetOpenPositions} {
		if _, err := f.NewSheet(name); err != nil {
			return nil, fmt.Errorf("add sheet %s: %w", name, err)
		}
	}

	w.closedPositions(e)
	w.cashOperations(e)
	w.openPositions(e)
	if w.err != nil {
		return nil, w.err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// cell is one value for a row: a text, a decimal number, or a time.
type cell interface{ isCell() }

type (
	text   string
	number string // decimal text, written as a number cell
	serial time.Time
)

func (text) isCell()   {}
func (number) isCell() {}
func (serial) isCell() {}

// writer writes rows and keeps the first error.
type writer struct {
	f   *excelize.File
	err error
}

func (w *writer) row(sheet string, rowNumber int, cells ...cell) {
	for i, c := range cells {
		if w.err != nil {
			return
		}
		name, err := excelize.CoordinatesToCellName(i+1, rowNumber)
		if err != nil {
			w.err = err
			return
		}
		w.err = w.set(sheet, name, c)
	}
}

func (w *writer) set(sheet, name string, c cell) error {
	switch v := c.(type) {
	case text:
		if v == "" {
			return nil
		}
		return w.f.SetCellStr(sheet, name, string(v))
	case number:
		if v == "" {
			return nil
		}
		if _, err := decimal.NewFromString(string(v)); err != nil {
			return fmt.Errorf("%s!%s: %q is not a decimal: %w", sheet, name, string(v), err)
		}
		// SetCellDefault writes the text as it is, in a number cell. No float64 rounding.
		return w.f.SetCellDefault(sheet, name, string(v))
	case serial:
		if time.Time(v).IsZero() {
			return nil
		}
		return w.f.SetCellDefault(sheet, name, excelSerial(time.Time(v)))
	default:
		return fmt.Errorf("%s!%s: unknown cell kind %T", sheet, name, c)
	}
}

// title writes rows 1 and 2 of a sheet: the account number and the sheet title.
func (w *writer) title(sheet string, e Export) {
	w.row(sheet, 1, text("Account number"), text(e.AccountNumber))
	w.row(sheet, 2, text(sheet))
}

// period writes rows 3 and 4 of the Closed Positions and Cash Operations sheets.
func (w *writer) period(sheet string, e Export) {
	w.row(sheet, 3, text("Date from (UTC)"), serial(e.From))
	w.row(sheet, 4, text("Date to (UTC)"), serial(e.To))
}

func (w *writer) closedPositions(e Export) {
	const sheet = SheetClosedPositions
	w.title(sheet, e)
	w.period(sheet, e)
	w.row(sheet, 5, texts(
		"Instrument", "Ticker", "Category", "Type", "Volume", "Open Price", "Open Time (UTC)",
		"Close Price", "Close Time (UTC)", "Product", "Profit/Loss", "Gross Profit", "Purchase Value",
		"Sale Value", "Stop Loss", "Take Profit", "Commission", "Margin", "Swap", "Rollover",
		"Open Conversion Rate", "Close Conversion Rate", "Close Origin", "Position ID", "Comment",
	)...)

	sum := decimal.Zero
	rowNumber := 6
	for _, p := range e.ClosedPositions {
		w.row(sheet, rowNumber,
			text(p.Instrument), text(p.Ticker), text(p.Category), text("BUY"), number(p.Volume),
			number(p.OpenPrice), serial(p.OpenTime), number(p.ClosePrice), serial(p.CloseTime),
			text(p.Product), number(p.ProfitLoss), number(p.ProfitLoss), number(p.PurchaseValue),
			number(p.SaleValue), text(""), text(""), number("0.0"), text(""), text(""), text(""),
			number(p.OpenConversionRate), number(p.CloseConversionRate), text("Web"),
			number(javaDouble(p.PositionID)), text(p.Comment),
		)
		sum = w.add(sum, p.ProfitLoss)
		rowNumber++
	}
	w.row(sheet, rowNumber, text("Profit/loss"))
	w.setAt(sheet, "K", rowNumber, number(sum.String()))
	w.setAt(sheet, "L", rowNumber, number(sum.String()))
}

func (w *writer) cashOperations(e Export) {
	const sheet = SheetCashOperations
	w.title(sheet, e)
	w.period(sheet, e)
	w.row(sheet, 5, texts("Type", "Instrument", "Ticker", "Category", "Time", "Amount", "ID", "Comment", "Product", "Position ID")...)

	sum := decimal.Zero
	rowNumber := 6
	for _, op := range e.CashOperations {
		w.row(sheet, rowNumber,
			text(op.Type), text(op.Instrument), text(op.Ticker), text(op.Category), serial(op.Time),
			number(op.Amount), text(op.ID), text(op.Comment), text(op.Product), text(op.PositionID),
		)
		sum = w.add(sum, op.Amount)
		rowNumber++
	}
	total := or(e.CashTotal, sum.String())
	w.row(sheet, rowNumber, text("Total"))
	w.setAt(sheet, "F", rowNumber, number(total))
}

func (w *writer) openPositions(e Export) {
	const sheet = SheetOpenPositions
	w.title(sheet, e)
	w.row(sheet, 3, text("Data as of report generated"), serial(e.To))
	w.row(sheet, 4, texts("Product", "Metric", "Amount", "Currency")...)

	value := decimal.Zero
	for _, l := range e.OpenLots {
		value = w.add(value, l.Value)
	}
	w.row(sheet, 5, text(DefaultProduct), text("Open position value"), number(value.String()), text(e.Currency))
	w.row(sheet, 6, text(DefaultProduct), text("Open position profit"), number("0.0"), text(e.Currency))
	w.row(sheet, 8, text("Note"), text("Summary values and open positions are shown as of the report generation time"))
	w.row(sheet, 9, texts(
		"Product", "Instrument/Position", "Ticker", "Category", "Type", "Volume", "Value", "Current price",
		"Open price", "Open time (UTC)", "Stop Loss", "Take Profit", "Net Profit %", "Net Profit",
		"Gross Profit", "Margin", "Open Commission", "Swap", "Rollover",
	)...)

	rowNumber := 10
	for _, group := range groupByTicker(e.OpenLots) {
		first := group[0]
		volume := decimal.Zero
		for _, l := range group {
			volume = w.add(volume, l.Volume)
		}
		w.row(sheet, rowNumber,
			text(first.Product), text(first.Instrument), text(first.Ticker), text(first.Category), text(""),
			number(volume.String()),
		)
		rowNumber++
		for _, l := range group {
			w.row(sheet, rowNumber,
				text(l.Product), text(l.PositionID), text(l.Ticker), text(""), text("BUY"), number(l.Volume),
				number(l.Value), number(l.CurrentPrice), number(l.OpenPrice), serial(l.OpenTime),
			)
			rowNumber++
		}
	}
}

// setAt writes one cell in column col of row rowNumber.
func (w *writer) setAt(sheet, col string, rowNumber int, c cell) {
	if w.err != nil {
		return
	}
	w.err = w.set(sheet, fmt.Sprintf("%s%d", col, rowNumber), c)
}

// add returns sum plus the decimal text v. An empty v adds nothing.
func (w *writer) add(sum decimal.Decimal, v string) decimal.Decimal {
	if v == "" || w.err != nil {
		return sum
	}
	d, err := decimal.NewFromString(v)
	if err != nil {
		w.err = fmt.Errorf("%q is not a decimal: %w", v, err)
		return sum
	}
	return sum.Add(d)
}

// groupByTicker groups the Lots by Ticker, in the order of the first Lot of each Ticker.
func groupByTicker(lots []OpenLot) [][]OpenLot {
	var groups [][]OpenLot
	index := map[string]int{}
	for _, l := range lots {
		i, ok := index[l.Ticker]
		if !ok {
			i = len(groups)
			index[l.Ticker] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], l)
	}
	return groups
}

// excelSerial returns t as an Excel serial date: days since 1899-12-30, with the time as the fraction.
// It divides with decimal arithmetic to 15 places. A float64 serial can read back one second early.
func excelSerial(t time.Time) string {
	nanos := decimal.NewFromInt(int64(t.UTC().Sub(excelEpoch)))
	return nanos.DivRound(decimal.NewFromInt(int64(24*time.Hour)), 15).String()
}

// javaDouble writes an integer ID in the form of the export, for example "2664874862" as "2.664874862E9".
// The export writes Position IDs in Closed Positions as Java doubles.
// For a value below 10^7, Java writes the plain form, for example "1000.0".
func javaDouble(id string) string {
	if id == "" || strings.ContainsAny(id, ".eE-") {
		return id
	}
	digits := strings.TrimLeft(id, "0")
	if len(digits) < 8 {
		return or(digits, "0") + ".0"
	}
	fraction := strings.TrimRight(digits[1:], "0")
	if fraction == "" {
		fraction = "0"
	}
	return fmt.Sprintf("%s.%sE%d", digits[:1], fraction, len(digits)-1)
}

func texts(values ...string) []cell {
	cells := make([]cell, len(values))
	for i, v := range values {
		cells[i] = text(v)
	}
	return cells
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
