// Package xtb reads the xlsx export of one XTB Account.
//
// The reader knows the sheet layout and the header rows. It knows nothing about the database.
package xtb

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// Errors of Read. Each error that Read returns wraps one of them.
var (
	// ErrNotExport tells that the file is not an XTB export.
	ErrNotExport = errors.New("the file is not an XTB export")
	// ErrUnknownOperationType tells that a Cash Operation has a type that the reader does not know.
	ErrUnknownOperationType = errors.New("unknown cash operation type")
	// ErrInvalidRow tells that a Cash Operation row has a value that the reader cannot use.
	ErrInvalidRow = errors.New("invalid cash operation row")
)

// OperationType is the type of a Cash Operation, as the export writes it.
type OperationType string

// The Cash Operation types that the reader knows.
const (
	TypeDeposit       OperationType = "Deposit"
	TypeStockPurchase OperationType = "Stock purchase"
)

// knownTypes are the types that the reader accepts. A later ticket adds the other types of the spec.
var knownTypes = []OperationType{TypeDeposit, TypeStockPurchase}

// sheetCashOperations is the name of the sheet with the Cash Operations.
const sheetCashOperations = "Cash Operations"

// cashHeader is row 5 of the Cash Operations sheet.
var cashHeader = []string{"Type", "Instrument", "Ticker", "Category", "Time", "Amount", "ID", "Comment", "Product", "Position ID"}

// Rows and columns of the Cash Operations sheet.
const (
	headerRow = 5
	firstRow  = 6

	colType       = 0
	colInstrument = 1
	colTicker     = 2
	colCategory   = 3
	colTime       = 4
	colAmount     = 5
	colID         = 6
	colComment    = 7
	colPositionID = 9
)

// Export is the content of one XTB export that the tool uses.
type Export struct {
	// AccountNumber is the broker account number from row 1.
	AccountNumber string
	// CashOperations are the rows of the Cash Operations sheet, in the order of the file.
	CashOperations []CashOperation
	// Total is the amount in the Total row of the Cash Operations sheet.
	Total decimal.Decimal
}

// CashOperation is one row of the Cash Operations sheet.
type CashOperation struct {
	// Row is the row number in the sheet, for error messages.
	Row  int
	Type OperationType
	// Instrument is the Instrument name. It is empty for an operation with no Instrument.
	Instrument string
	// Ticker is the broker ticker, for example "FPS.US". It is empty for an operation with no Instrument.
	Ticker   string
	Category string
	Time     time.Time
	Amount   decimal.Decimal
	// ID is the XTB ID of the operation.
	ID      string
	Comment string
	// PositionID is the XTB position ID, which is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
	// Purchase is the parsed Comment of a Stock purchase. It is nil for the other types.
	Purchase *Purchase
}

// Purchase is the parsed Comment of a Stock purchase, for example "OPEN BUY FPS.US 1/1.27 @ 39.24".
type Purchase struct {
	// Volume is the volume of this Fill.
	Volume decimal.Decimal
	// OrderVolume is the volume of the full order. It is Volume when the Comment has no order volume.
	OrderVolume decimal.Decimal
	// Price is the price of one unit.
	Price decimal.Decimal
}

// purchaseComment matches "OPEN BUY [TICKER] <volume>[/<order volume>] @ <price>".
// The ticker must contain a letter. Thus the volume never matches as a ticker.
var purchaseComment = regexp.MustCompile(`^OPEN BUY (?:\S*[A-Za-z]\S* )?(\d+(?:\.\d+)?)(?:/(\d+(?:\.\d+)?))? @ (\d+(?:\.\d+)?)$`)

// fileCurrency matches the currency prefix of an export file name, for example "USD_".
var fileCurrency = regexp.MustCompile(`^([A-Z]{3})_`)

// CurrencyFromFileName returns the currency prefix of an export file name,
// for example "USD" for "USD_12345678_2006-01-01_2026-10-22.xlsx". It returns "" if there is no prefix.
func CurrencyFromFileName(name string) string {
	m := fileCurrency.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	return m[1]
}

// Read reads an XTB export from the bytes of an xlsx file.
func Read(data []byte) (Export, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return Export{}, fmt.Errorf("%w: it is not an xlsx file", ErrNotExport)
	}
	defer func() { _ = f.Close() }()

	if idx, err := f.GetSheetIndex(sheetCashOperations); err != nil || idx < 0 {
		return Export{}, fmt.Errorf("%w: it has no %q sheet", ErrNotExport, sheetCashOperations)
	}
	rows, err := f.GetRows(sheetCashOperations, excelize.Options{RawCellValue: true})
	if err != nil {
		return Export{}, fmt.Errorf("read %s sheet: %w", sheetCashOperations, err)
	}
	return readCashOperations(rows)
}

// readCashOperations reads the rows of the Cash Operations sheet.
func readCashOperations(rows [][]string) (Export, error) {
	if len(rows) < headerRow || cellAt(rows[0], 0) != "Account number" || cellAt(rows[0], 1) == "" {
		return Export{}, fmt.Errorf("%w: the %q sheet has no account number in row 1", ErrNotExport, sheetCashOperations)
	}
	if header := rows[headerRow-1]; len(header) < len(cashHeader) || !slices.Equal(header[:len(cashHeader)], cashHeader) {
		return Export{}, fmt.Errorf("%w: the %q sheet has an unknown header in row %d", ErrNotExport, sheetCashOperations, headerRow)
	}

	export := Export{AccountNumber: strings.TrimSpace(rows[0][1])}
	for i := firstRow - 1; i < len(rows); i++ {
		row, rowNumber := rows[i], i+1
		typ := strings.TrimSpace(cellAt(row, colType))
		switch typ {
		case "":
			continue
		case "Total":
			total, err := decimal.NewFromString(cellAt(row, colAmount))
			if err != nil {
				return Export{}, fmt.Errorf("%w: the Total row %d has no amount", ErrNotExport, rowNumber)
			}
			export.Total = total
			return export, nil
		}
		op, err := readCashOperation(row, rowNumber, OperationType(typ))
		if err != nil {
			return Export{}, err
		}
		export.CashOperations = append(export.CashOperations, op)
	}
	return Export{}, fmt.Errorf("%w: the %q sheet has no Total row", ErrNotExport, sheetCashOperations)
}

// readCashOperation reads one row of the Cash Operations sheet.
func readCashOperation(row []string, rowNumber int, typ OperationType) (CashOperation, error) {
	if !slices.Contains(knownTypes, typ) {
		return CashOperation{}, fmt.Errorf("%w %q in row %d", ErrUnknownOperationType, string(typ), rowNumber)
	}

	op := CashOperation{
		Row:        rowNumber,
		Type:       typ,
		Instrument: strings.TrimSpace(cellAt(row, colInstrument)),
		Ticker:     strings.TrimSpace(cellAt(row, colTicker)),
		Category:   strings.TrimSpace(cellAt(row, colCategory)),
		ID:         strings.TrimSpace(cellAt(row, colID)),
		Comment:    strings.TrimSpace(cellAt(row, colComment)),
		PositionID: strings.TrimSpace(cellAt(row, colPositionID)),
	}
	if op.ID == "" {
		return CashOperation{}, fmt.Errorf("%w: row %d has no ID", ErrInvalidRow, rowNumber)
	}
	amount, err := decimal.NewFromString(cellAt(row, colAmount))
	if err != nil {
		return CashOperation{}, fmt.Errorf("%w: row %d has the amount %q, which is not a number", ErrInvalidRow, rowNumber, cellAt(row, colAmount))
	}
	op.Amount = amount
	at, err := serialTime(cellAt(row, colTime))
	if err != nil {
		return CashOperation{}, fmt.Errorf("%w: row %d has the time %q, which is not a date", ErrInvalidRow, rowNumber, cellAt(row, colTime))
	}
	op.Time = at

	if typ == TypeStockPurchase {
		if op.Ticker == "" || op.PositionID == "" {
			return CashOperation{}, fmt.Errorf("%w: the %s in row %d has no ticker or no position ID", ErrInvalidRow, typ, rowNumber)
		}
		purchase, err := parsePurchase(op.Comment)
		if err != nil {
			return CashOperation{}, fmt.Errorf("%w: row %d: %w", ErrInvalidRow, rowNumber, err)
		}
		op.Purchase = &purchase
	}
	return op, nil
}

// parsePurchase parses the Comment of a Stock purchase.
// It accepts the Comment with and without the ticker, and with and without the order volume.
func parsePurchase(comment string) (Purchase, error) {
	m := purchaseComment.FindStringSubmatch(comment)
	if m == nil {
		return Purchase{}, fmt.Errorf("the purchase comment %q does not have the form \"OPEN BUY [TICKER] <volume> @ <price>\"", comment)
	}
	// The regular expression accepts only decimal digits. Thus these conversions cannot fail.
	volume := decimal.RequireFromString(m[1])
	orderVolume := volume
	if m[2] != "" {
		orderVolume = decimal.RequireFromString(m[2])
	}
	return Purchase{Volume: volume, OrderVolume: orderVolume, Price: decimal.RequireFromString(m[3])}, nil
}

// excelEpoch is day 0 of the Excel serial dates.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// secondsPerDay converts an Excel serial date to seconds.
var secondsPerDay = decimal.NewFromInt(int64(24 * time.Hour / time.Second))

// serialTime converts an Excel serial date to a UTC time, rounded to the second.
// It uses decimal arithmetic. A float64 serial can give a time one second early.
func serialTime(serial string) (time.Time, error) {
	days, err := decimal.NewFromString(serial)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse serial date: %w", err)
	}
	seconds := days.Mul(secondsPerDay).Round(0).IntPart()
	return excelEpoch.Add(time.Duration(seconds) * time.Second), nil
}

// cellAt returns the cell at index i of the row, or "" for a short row.
func cellAt(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return row[i]
}
