// Package kraken reads the ledger export (CSV) of the Provider Kraken.
// It returns the Cash Operations of the spot wallet: the EUR deposits, the crypto purchases and the Rewards.
// Each other entry is an unsupported entry. One unsupported entry stops the read.
package kraken

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// header is the header row of a Kraken ledger export.
var header = []string{
	"txid", "refid", "time", "type", "subtype", "aclass", "subclass", "asset", "wallet",
	"amount", "fee", "balance", "amountusd", "feeusd", "balanceusd", "feecurrency",
}

// The columns of the ledger that the reader uses, by their index in header.
const (
	colRefID       = 1
	colTime        = 2
	colType        = 3
	colSubtype     = 4
	colAsset       = 7
	colWallet      = 8
	colAmount      = 9
	colFee         = 10
	colFeeCurrency = 15
)

// timeLayout is the layout of the time column. The times are in UTC.
const timeLayout = "2006-01-02 15:04:05"

// The values of the ledger that the reader supports.
const (
	// Currency is the currency of the cash of the Kraken Account.
	Currency = "EUR"
	// spotWallet is the only wallet that the reader supports.
	spotWallet = "spot / main"
)

// assetCodes maps an old Kraken asset code to the code that the tool uses as the Instrument ticker.
var assetCodes = map[string]string{"XBT": "BTC"}

// Errors of Read.
var (
	// ErrNotKraken tells that the file is not a Kraken ledger export.
	ErrNotKraken = errors.New("the file is not a Kraken ledger export")
	// ErrUnsupportedEntries tells that the export has entries that the tool does not support.
	ErrUnsupportedEntries = errors.New("the Kraken export has entries that the tool does not support")
)

// The reasons why an entry is not supported.
const (
	ReasonUnknownEntry    = "unknown entry"
	ReasonSale            = "sale"
	ReasonWithdrawal      = "withdrawal"
	ReasonFee             = "fee that is not in the crypto asset"
	ReasonDepositNotEUR   = "deposit that is not EUR"
	ReasonWallet          = "wallet that is not spot / main"
	ReasonIncompleteTrade = "trade that is not one EUR row and one crypto row"
)

// Kinds of a Cash Operation.
const (
	KindDeposit  = "deposit"
	KindPurchase = "purchase"
	// KindReward is a Reward: an amount of a crypto asset that Kraken pays for a balance, for example Auto Earn.
	KindReward = "reward"
)

// Export is the content of a Kraken ledger export.
type Export struct {
	// CashOperations are the Cash Operations, in time order and then in reference ID order.
	CashOperations []CashOperation
}

// CashOperation is one deposit, one crypto purchase or one Reward.
type CashOperation struct {
	// RefID is the Kraken reference ID. It is the Provider ID of the Cash Operation.
	RefID string
	// Kind is KindDeposit or KindPurchase.
	Kind string
	Time time.Time
	// Amount is in EUR. A purchase has a negative amount. A Reward has the amount zero.
	Amount decimal.Decimal
	// Asset is the asset code of the crypto asset of a purchase or a Reward, for example "BTC". It is empty for a deposit.
	Asset string
	// Volume is the crypto amount after the fee. It is zero for a deposit.
	Volume decimal.Decimal
	// Price is the EUR amount divided by the crypto amount before the fee. It is zero for a deposit and a Reward.
	Price decimal.Decimal
	// Fee is the fee in the crypto asset. It is zero for a deposit.
	Fee decimal.Decimal
}

// Entry is one row of the ledger that the tool does not support.
type Entry struct {
	Type    string
	Subtype string
	Asset   string
	Time    time.Time
	// Reason tells why the tool does not support the entry, for example ReasonSale.
	Reason string
}

// UnsupportedError lists the entries that the tool does not support.
type UnsupportedError struct {
	// Entries are in the order of the file.
	Entries []Entry
}

func (e *UnsupportedError) Error() string {
	parts := make([]string, 0, len(e.Entries))
	for _, entry := range e.Entries {
		parts = append(parts, fmt.Sprintf("%s at %s (type %q, subtype %q, asset %q)",
			entry.Reason, entry.Time.UTC().Format(time.RFC3339), entry.Type, entry.Subtype, entry.Asset))
	}
	return fmt.Sprintf("%v: %s", ErrUnsupportedEntries, strings.Join(parts, "; "))
}

// Unwrap returns ErrUnsupportedEntries.
func (e *UnsupportedError) Unwrap() error { return ErrUnsupportedEntries }

// IsExport tells if data is a CSV file with the header row of a Kraken ledger export.
func IsExport(data []byte) bool {
	r := csv.NewReader(bytes.NewReader(trimBOM(data)))
	first, err := r.Read()
	return err == nil && slices.Equal(first, header)
}

// row is one parsed row of the ledger.
type row struct {
	refID       string
	time        time.Time
	typ         string
	subtype     string
	asset       string
	wallet      string
	amount      decimal.Decimal
	fee         decimal.Decimal
	feeCurrency string
}

// entry returns the row as an unsupported entry with the reason.
func (r row) entry(reason string) Entry {
	return Entry{Type: r.typ, Subtype: r.subtype, Asset: r.asset, Time: r.time, Reason: reason}
}

// Read reads a Kraken ledger export. It returns an error that wraps ErrNotKraken if the file is not a Kraken export,
// and an *UnsupportedError if an entry is not supported.
func Read(data []byte) (Export, error) {
	rows, err := readRows(data)
	if err != nil {
		return Export{}, err
	}

	// The rows of one reference ID make one Cash Operation, for example the two rows of a trade.
	groups := map[string][]row{}
	var order []string
	for _, r := range rows {
		if _, ok := groups[r.refID]; !ok {
			order = append(order, r.refID)
		}
		groups[r.refID] = append(groups[r.refID], r)
	}

	var export Export
	var unsupported []Entry
	for _, refID := range order {
		op, entries := cashOperation(groups[refID])
		if len(entries) > 0 {
			unsupported = append(unsupported, entries...)
			continue
		}
		export.CashOperations = append(export.CashOperations, op)
	}
	if len(unsupported) > 0 {
		return Export{}, &UnsupportedError{Entries: unsupported}
	}
	slices.SortStableFunc(export.CashOperations, func(a, b CashOperation) int {
		return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(a.RefID, b.RefID))
	})
	return export, nil
}

// readRows reads and parses the rows of the file after the header row.
func readRows(data []byte) ([]row, error) {
	r := csv.NewReader(bytes.NewReader(trimBOM(data)))
	r.FieldsPerRecord = len(header)
	first, err := r.Read()
	if err != nil || !slices.Equal(first, header) {
		return nil, fmt.Errorf("%w: the first row is not the header row of a Kraken ledger", ErrNotKraken)
	}
	var rows []row
	for line := 2; ; line++ {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read the Kraken ledger: %w", err)
		}
		parsed, err := parseRow(line, record)
		if err != nil {
			return nil, err
		}
		rows = append(rows, parsed)
	}
}

// parseRow parses one record of the file. line is its line number in the file.
func parseRow(line int, record []string) (row, error) {
	at, err := time.ParseInLocation(timeLayout, record[colTime], time.UTC)
	if err != nil {
		return row{}, fmt.Errorf("row %d: the time %q is not in the form %s", line, record[colTime], timeLayout)
	}
	amount, err := decimal.NewFromString(record[colAmount])
	if err != nil {
		return row{}, fmt.Errorf("row %d: the amount %q is not a decimal", line, record[colAmount])
	}
	fee := decimal.Zero
	if record[colFee] != "" {
		if fee, err = decimal.NewFromString(record[colFee]); err != nil {
			return row{}, fmt.Errorf("row %d: the fee %q is not a decimal", line, record[colFee])
		}
	}
	return row{
		refID:       record[colRefID],
		time:        at,
		typ:         record[colType],
		subtype:     record[colSubtype],
		asset:       assetCode(record[colAsset]),
		wallet:      record[colWallet],
		amount:      amount,
		fee:         fee,
		feeCurrency: assetCode(record[colFeeCurrency]),
	}, nil
}

// cashOperation returns the Cash Operation of the rows of one reference ID,
// or the unsupported entries if the rows do not make a supported Cash Operation.
func cashOperation(rows []row) (CashOperation, []Entry) {
	for _, r := range rows {
		if r.wallet != spotWallet {
			return CashOperation{}, entries(rows, ReasonWallet)
		}
	}
	first := rows[0]
	switch {
	case len(rows) == 1 && first.typ == "deposit" && first.subtype == "":
		return deposit(first)
	case first.typ == "withdrawal":
		return CashOperation{}, entries(rows, ReasonWithdrawal)
	case first.typ == "trade" && first.subtype == "tradespot":
		return purchase(rows)
	case len(rows) == 1 && first.typ == "earn" && first.subtype == "reward":
		return reward(first)
	}
	return CashOperation{}, entries(rows, ReasonUnknownEntry)
}

// deposit returns the deposit of the row. Only a EUR deposit with no fee is supported.
func deposit(r row) (CashOperation, []Entry) {
	switch {
	case r.asset != Currency:
		return CashOperation{}, []Entry{r.entry(ReasonDepositNotEUR)}
	case !r.fee.IsZero():
		return CashOperation{}, []Entry{r.entry(ReasonFee)}
	case !r.amount.IsPositive():
		return CashOperation{}, []Entry{r.entry(ReasonUnknownEntry)}
	}
	return CashOperation{RefID: r.refID, Kind: KindDeposit, Time: r.time, Amount: r.amount}, nil
}

// purchase returns the crypto purchase of the two rows of a spot trade: a EUR row with a negative amount
// and a crypto row with a positive amount. The fee must be in the crypto asset.
func purchase(rows []row) (CashOperation, []Entry) {
	if len(rows) != 2 {
		return CashOperation{}, entries(rows, ReasonIncompleteTrade)
	}
	eur, crypto := rows[0], rows[1]
	if crypto.asset == Currency {
		eur, crypto = crypto, eur
	}
	for _, r := range rows {
		if r.typ != "trade" || r.subtype != "tradespot" {
			return CashOperation{}, entries(rows, ReasonIncompleteTrade)
		}
	}
	switch {
	case eur.asset != Currency || crypto.asset == Currency:
		return CashOperation{}, entries(rows, ReasonIncompleteTrade)
	case crypto.amount.IsNegative() || eur.amount.IsPositive():
		return CashOperation{}, entries(rows, ReasonSale)
	case !eur.amount.IsNegative() || !crypto.amount.IsPositive():
		return CashOperation{}, entries(rows, ReasonIncompleteTrade)
	case !eur.fee.IsZero():
		return CashOperation{}, entries(rows, ReasonFee)
	case !crypto.fee.IsZero() && crypto.feeCurrency != "" && crypto.feeCurrency != crypto.asset:
		return CashOperation{}, entries(rows, ReasonFee)
	}
	return CashOperation{
		RefID:  crypto.refID,
		Kind:   KindPurchase,
		Time:   crypto.time,
		Amount: eur.amount,
		Asset:  crypto.asset,
		Volume: crypto.amount.Sub(crypto.fee),
		// The division keeps decimal.DivisionPrecision (16) decimal places.
		Price: eur.amount.Neg().Div(crypto.amount),
		Fee:   crypto.fee,
	}, nil
}

// reward returns the Reward of the row: a positive amount of a crypto asset. The fee must be in that asset.
// The volume is the amount after the fee, like the volume of a purchase.
func reward(r row) (CashOperation, []Entry) {
	switch {
	case r.asset == Currency || !r.amount.IsPositive():
		return CashOperation{}, []Entry{r.entry(ReasonUnknownEntry)}
	case !r.fee.IsZero() && r.feeCurrency != "" && r.feeCurrency != r.asset:
		return CashOperation{}, []Entry{r.entry(ReasonFee)}
	}
	return CashOperation{
		RefID:  r.refID,
		Kind:   KindReward,
		Time:   r.time,
		Amount: decimal.Zero,
		Asset:  r.asset,
		Volume: r.amount.Sub(r.fee),
		Price:  decimal.Zero,
		Fee:    r.fee,
	}, nil
}

// entries returns each row as an unsupported entry with the reason.
func entries(rows []row, reason string) []Entry {
	result := make([]Entry, 0, len(rows))
	for _, r := range rows {
		result = append(result, r.entry(reason))
	}
	return result
}

// assetCode returns the code of the asset that the tool uses, for example "BTC" for "XBT".
func assetCode(code string) string {
	if mapped, ok := assetCodes[code]; ok {
		return mapped
	}
	return code
}

// trimBOM removes the UTF-8 byte order mark at the start of data.
func trimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
}
