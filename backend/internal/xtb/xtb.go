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
	// ErrInvalidClosedPosition tells that a Closed Positions row has a value that the reader cannot use.
	ErrInvalidClosedPosition = errors.New("invalid closed position row")
	// ErrInvalidOpenPosition tells that a Lot row of the Open Positions sheet has a value that the reader cannot use.
	ErrInvalidOpenPosition = errors.New("invalid open position row")
)

// OperationType is the type of a Cash Operation, as the export writes it.
type OperationType string

// The Cash Operation types that the reader knows.
const (
	TypeDeposit        OperationType = "Deposit"
	TypeWithdrawal     OperationType = "Withdrawal"
	TypeStockPurchase  OperationType = "Stock purchase"
	TypeStockSell      OperationType = "Stock sell"
	TypeDividend       OperationType = "Dividend"
	TypeWithholdingTax OperationType = "Withholding tax"
	TypeInterest       OperationType = "Free funds interest"
	TypeInterestTax    OperationType = "Free funds interest tax"
	TypeSECFee         OperationType = "SEC fee"
	TypeCorrection     OperationType = "Correction"
	// TypeTransfer is a Transfer between two Accounts of the user. See ADR 0003.
	TypeTransfer OperationType = "IKZE deposit"
)

// TransferDirection is the direction of a Transfer. ADR 0003: the sign of the amount gives it.
type TransferDirection string

// The directions of a Transfer.
const (
	// TransferIn is a Transfer with a positive amount. The Account gets the money.
	TransferIn TransferDirection = "in"
	// TransferOut is a Transfer with a negative amount. The Account sends the money.
	TransferOut TransferDirection = "out"
)

// knownTypes are the types that the reader accepts.
var knownTypes = []OperationType{
	TypeDeposit, TypeWithdrawal, TypeStockPurchase, TypeStockSell,
	TypeDividend, TypeWithholdingTax, TypeInterest, TypeInterestTax, TypeSECFee, TypeCorrection, TypeTransfer,
}

// Sheet names of the export.
const (
	sheetCashOperations  = "Cash Operations"
	sheetClosedPositions = "Closed Positions"
	sheetOpenPositions   = "Open Positions"
)

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

// closedHeader is row 5 of the Closed Positions sheet.
var closedHeader = []string{
	"Instrument", "Ticker", "Category", "Type", "Volume", "Open Price", "Open Time (UTC)",
	"Close Price", "Close Time (UTC)", "Product", "Profit/Loss", "Gross Profit", "Purchase Value",
	"Sale Value", "Stop Loss", "Take Profit", "Commission", "Margin", "Swap", "Rollover",
	"Open Conversion Rate", "Close Conversion Rate", "Close Origin", "Position ID", "Comment",
}

// Columns of the Closed Positions sheet. The rows and the header row are the same as in the Cash Operations sheet.
const (
	colClosedTicker     = 1
	colClosedVolume     = 4
	colClosedClosePrice = 7
	colClosedCloseTime  = 8
	colClosedProfitLoss = 10
	colClosedPositionID = 23
)

// openHeader is row 9 of the Open Positions sheet.
var openHeader = []string{"Product", "Instrument/Position", "Ticker", "Category", "Type", "Volume"}

// Rows and columns of the Open Positions sheet. A group row per Instrument has no Type.
// The Lot rows of the Instrument follow it, with the Position ID in place of the Instrument name.
const (
	openHeaderRow = 9

	colOpenPositionID = 1
	colOpenTicker     = 2
	colOpenType       = 4
	colOpenVolume     = 5
)

// openLotType is the Type of a Lot row in the Open Positions sheet.
const openLotType = "BUY"

// closedSummary is the text in column A of the summary row under the Closed Positions rows.
const closedSummary = "Profit/loss"

// Export is the content of one XTB export that the tool uses.
type Export struct {
	// AccountNumber is the broker account number from row 1.
	AccountNumber string
	// CashOperations are the rows of the Cash Operations sheet, in the order of the file.
	CashOperations []CashOperation
	// Total is the amount in the Total row of the Cash Operations sheet.
	Total decimal.Decimal
	// ClosedPositions are the rows of the Closed Positions sheet, in the order of the file.
	ClosedPositions []ClosedPosition
	// OpenLots are the Lot rows of the Open Positions sheet, in the order of the file.
	OpenLots []OpenLot
}

// OpenLot is one Lot row of the Open Positions sheet: the open part of a Lot at the time of the report.
// The tool uses only the Lot ID and the volume, never the prices. See ADR 0002.
type OpenLot struct {
	// Row is the row number in the sheet, for error messages.
	Row    int
	Ticker string
	// PositionID is the Lot ID, in the same form as in the Cash Operations sheet.
	PositionID string
	// Volume is the open volume.
	Volume decimal.Decimal
}

// ClosedPosition is one row of the Closed Positions sheet: one closed part of a Lot.
// The tool uses it only to check its Realized Profit. See ADR 0001.
type ClosedPosition struct {
	// Row is the row number in the sheet, for error messages.
	Row    int
	Ticker string
	// PositionID is the Lot ID, in the same form as in the Cash Operations sheet.
	PositionID string
	// Volume is the closed volume.
	Volume decimal.Decimal
	// ClosePrice is the price of one unit at the close. It is not valid if the cell is empty.
	// The cell can carry float noise, for example "17.950000000000003".
	ClosePrice decimal.NullDecimal
	CloseTime  time.Time
	// ProfitLoss is the profit of the closed part that the broker calculated, in the Account currency.
	ProfitLoss decimal.Decimal
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
	// Sale is the parsed Comment of a Stock sell. It is nil for the other types.
	Sale *Sale
	// Transfer is the direction of a Transfer. It is "" for the other types.
	Transfer TransferDirection
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

// Sale is the parsed Comment of a Stock sell, for example "CLOSE BUY 9/10 @ 10.26".
type Sale struct {
	// Volume is the closed volume of the Lot. ADR 0001: the number before the slash.
	Volume decimal.Decimal
	// OrderVolume is the volume of the full sell order, which can close more than one Lot.
	// It is Volume when the Comment has no order volume.
	OrderVolume decimal.Decimal
	// Price is the price of one unit.
	Price decimal.Decimal
}

// The Comments of purchases and sales: "<prefix> [TICKER] <volume>[/<order volume>] @ <price>".
// The ticker must contain a letter. Thus the volume never matches as a ticker.
var (
	purchaseComment = regexp.MustCompile(`^OPEN BUY (?:\S*[A-Za-z]\S* )?(\d+(?:\.\d+)?)(?:/(\d+(?:\.\d+)?))? @ (\d+(?:\.\d+)?)$`)
	saleComment     = regexp.MustCompile(`^CLOSE BUY (?:\S*[A-Za-z]\S* )?(\d+(?:\.\d+)?)(?:/(\d+(?:\.\d+)?))? @ (\d+(?:\.\d+)?)$`)
)

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
	export, err := readCashOperations(rows)
	if err != nil {
		return Export{}, err
	}

	if idx, err := f.GetSheetIndex(sheetClosedPositions); err != nil || idx < 0 {
		return Export{}, fmt.Errorf("%w: it has no %q sheet", ErrNotExport, sheetClosedPositions)
	}
	rows, err = f.GetRows(sheetClosedPositions, excelize.Options{RawCellValue: true})
	if err != nil {
		return Export{}, fmt.Errorf("read %s sheet: %w", sheetClosedPositions, err)
	}
	export.ClosedPositions, err = readClosedPositions(rows)
	if err != nil {
		return Export{}, err
	}

	if idx, err := f.GetSheetIndex(sheetOpenPositions); err != nil || idx < 0 {
		return Export{}, fmt.Errorf("%w: it has no %q sheet", ErrNotExport, sheetOpenPositions)
	}
	rows, err = f.GetRows(sheetOpenPositions, excelize.Options{RawCellValue: true})
	if err != nil {
		return Export{}, fmt.Errorf("read %s sheet: %w", sheetOpenPositions, err)
	}
	export.OpenLots, err = readOpenLots(rows)
	if err != nil {
		return Export{}, err
	}
	return export, nil
}

// readOpenLots reads the Lot rows of the Open Positions sheet. It skips the group rows, which have no Type.
func readOpenLots(rows [][]string) ([]OpenLot, error) {
	if len(rows) < openHeaderRow {
		return nil, fmt.Errorf("%w: the %q sheet has no header in row %d", ErrNotExport, sheetOpenPositions, openHeaderRow)
	}
	if header := rows[openHeaderRow-1]; len(header) < len(openHeader) || !slices.Equal(header[:len(openHeader)], openHeader) {
		return nil, fmt.Errorf("%w: the %q sheet has an unknown header in row %d", ErrNotExport, sheetOpenPositions, openHeaderRow)
	}

	var lots []OpenLot
	for i := openHeaderRow; i < len(rows); i++ {
		row, rowNumber := rows[i], i+1
		typ := strings.TrimSpace(cellAt(row, colOpenType))
		if typ == "" {
			continue
		}
		lot, err := readOpenLot(row, rowNumber, typ)
		if err != nil {
			return nil, err
		}
		lots = append(lots, lot)
	}
	return lots, nil
}

// readOpenLot reads one Lot row of the Open Positions sheet.
func readOpenLot(row []string, rowNumber int, typ string) (OpenLot, error) {
	invalid := func(column, value string) error {
		return fmt.Errorf("%w: row %d has the %s %q, which is not valid", ErrInvalidOpenPosition, rowNumber, column, value)
	}
	if typ != openLotType {
		return OpenLot{}, invalid("type", typ)
	}
	lot := OpenLot{Row: rowNumber, Ticker: strings.TrimSpace(cellAt(row, colOpenTicker))}

	raw := strings.TrimSpace(cellAt(row, colOpenPositionID))
	id, ok := positionID(raw)
	if !ok {
		return OpenLot{}, invalid("position ID", raw)
	}
	lot.PositionID = id

	var err error
	if lot.Volume, err = decimal.NewFromString(cellAt(row, colOpenVolume)); err != nil {
		return OpenLot{}, invalid("volume", cellAt(row, colOpenVolume))
	}
	return lot, nil
}

// positionID returns the Position ID in its plain form, and false if the value is not a Position ID.
// The export writes it as plain digits or as a Java double, for example "2.636532207E9".
func positionID(value string) (string, bool) {
	id, err := decimal.NewFromString(value)
	if err != nil || !id.IsInteger() || id.IsNegative() {
		return "", false
	}
	return id.String(), true
}

// readClosedPositions reads the rows of the Closed Positions sheet, up to the summary row.
func readClosedPositions(rows [][]string) ([]ClosedPosition, error) {
	if len(rows) < headerRow {
		return nil, fmt.Errorf("%w: the %q sheet has no header in row %d", ErrNotExport, sheetClosedPositions, headerRow)
	}
	if header := rows[headerRow-1]; len(header) < len(closedHeader) || !slices.Equal(header[:len(closedHeader)], closedHeader) {
		return nil, fmt.Errorf("%w: the %q sheet has an unknown header in row %d", ErrNotExport, sheetClosedPositions, headerRow)
	}

	var positions []ClosedPosition
	for i := firstRow - 1; i < len(rows); i++ {
		row, rowNumber := rows[i], i+1
		first := strings.TrimSpace(cellAt(row, 0))
		if first == closedSummary {
			break
		}
		if first == "" && strings.TrimSpace(cellAt(row, colClosedPositionID)) == "" {
			continue
		}
		p, err := readClosedPosition(row, rowNumber)
		if err != nil {
			return nil, err
		}
		positions = append(positions, p)
	}
	return positions, nil
}

// readClosedPosition reads one row of the Closed Positions sheet.
func readClosedPosition(row []string, rowNumber int) (ClosedPosition, error) {
	invalid := func(column, value string) error {
		return fmt.Errorf("%w: row %d has the %s %q, which is not valid", ErrInvalidClosedPosition, rowNumber, column, value)
	}
	p := ClosedPosition{Row: rowNumber, Ticker: strings.TrimSpace(cellAt(row, colClosedTicker))}

	// The export writes the Position ID as a Java double, for example "2.636532207E9".
	id, ok := positionID(cellAt(row, colClosedPositionID))
	if !ok {
		return ClosedPosition{}, invalid("position ID", cellAt(row, colClosedPositionID))
	}
	p.PositionID = id

	var err error
	if p.Volume, err = decimal.NewFromString(cellAt(row, colClosedVolume)); err != nil {
		return ClosedPosition{}, invalid("volume", cellAt(row, colClosedVolume))
	}
	if price := strings.TrimSpace(cellAt(row, colClosedClosePrice)); price != "" {
		closePrice, err := decimal.NewFromString(price)
		if err != nil {
			return ClosedPosition{}, invalid("close price", cellAt(row, colClosedClosePrice))
		}
		p.ClosePrice = decimal.NewNullDecimal(closePrice)
	}
	if p.CloseTime, err = serialTime(cellAt(row, colClosedCloseTime)); err != nil {
		return ClosedPosition{}, invalid("close time", cellAt(row, colClosedCloseTime))
	}
	if p.ProfitLoss, err = decimal.NewFromString(cellAt(row, colClosedProfitLoss)); err != nil {
		return ClosedPosition{}, invalid("profit/loss", cellAt(row, colClosedProfitLoss))
	}
	return p, nil
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

	if typ == TypeTransfer {
		return withTransferDirection(op)
	}
	if typ != TypeStockPurchase && typ != TypeStockSell {
		return op, nil
	}
	if op.Ticker == "" || op.PositionID == "" {
		return CashOperation{}, fmt.Errorf("%w: the %s in row %d has no ticker or no position ID", ErrInvalidRow, typ, rowNumber)
	}
	if typ == TypeStockPurchase {
		volume, orderVolume, price, err := parseComment(purchaseComment, "purchase", "OPEN BUY", op.Comment)
		if err != nil {
			return CashOperation{}, fmt.Errorf("%w: row %d: %w", ErrInvalidRow, rowNumber, err)
		}
		op.Purchase = &Purchase{Volume: volume, OrderVolume: orderVolume, Price: price}
		return op, nil
	}
	volume, orderVolume, price, err := parseComment(saleComment, "sale", "CLOSE BUY", op.Comment)
	if err != nil {
		return CashOperation{}, fmt.Errorf("%w: row %d: %w", ErrInvalidRow, rowNumber, err)
	}
	op.Sale = &Sale{Volume: volume, OrderVolume: orderVolume, Price: price}
	return op, nil
}

// withTransferDirection returns the Transfer op with its direction from the sign of the amount. See ADR 0003.
// A zero amount has no direction, so it is an invalid row.
func withTransferDirection(op CashOperation) (CashOperation, error) {
	switch op.Amount.Sign() {
	case 1:
		op.Transfer = TransferIn
	case -1:
		op.Transfer = TransferOut
	default:
		return CashOperation{}, fmt.Errorf("%w: the %s in row %d has a zero amount, so it has no direction", ErrInvalidRow, op.Type, op.Row)
	}
	return op, nil
}

// parseComment parses the Comment of a purchase or a sale with the regular expression re.
// It accepts the Comment with and without the ticker, and with and without the order volume.
// The order volume is the volume when the Comment has no order volume.
// It rejects a zero volume and a zero order volume.
func parseComment(re *regexp.Regexp, kind, prefix, comment string) (volume, orderVolume, price decimal.Decimal, err error) {
	m := re.FindStringSubmatch(comment)
	if m == nil {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{},
			fmt.Errorf("the %s comment %q does not have the form \"%s [TICKER] <volume> @ <price>\"", kind, comment, prefix)
	}
	// The regular expression accepts only decimal digits. Thus these conversions cannot fail.
	volume = decimal.RequireFromString(m[1])
	orderVolume = volume
	if m[2] != "" {
		orderVolume = decimal.RequireFromString(m[2])
	}
	// The regular expression accepts no sign. Thus a volume that is not positive is zero.
	// A zero volume gives no Lot volume to divide by, and a sale with it closes nothing. See ADR 0001.
	if volume.IsZero() || orderVolume.IsZero() {
		return decimal.Decimal{}, decimal.Decimal{}, decimal.Decimal{},
			fmt.Errorf("the %s comment %q has a zero volume or a zero order volume", kind, comment)
	}
	return volume, orderVolume, decimal.RequireFromString(m[3]), nil
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
