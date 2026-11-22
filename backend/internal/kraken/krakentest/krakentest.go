// Package krakentest builds small CSV files in the layout of a Kraken ledger export, for tests.
//
// A test gives only the fields that it needs. Build fills the other fields with defaults.
// Amounts and fees are decimal text, for example "-100.00". Build writes them with the same text.
package krakentest

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Header is the header row of a Kraken ledger export.
var Header = []string{
	"txid", "refid", "time", "type", "subtype", "aclass", "subclass", "asset", "wallet",
	"amount", "fee", "balance", "amountusd", "feeusd", "balanceusd", "feecurrency",
}

// Defaults for the fields that a test does not give.
const (
	// DefaultWallet is the wallet of the spot account.
	DefaultWallet = "spot / main"
	// DefaultFileName is the file name of a real export.
	DefaultFileName = "kraken_spot_ledgers_2026-01-01-2026-11-01.csv"
	// TimeLayout is the layout of the time column. The times are in UTC.
	TimeLayout = "2006-01-02 15:04:05"
)

// defaultTime is the time of the first entry with no time.
var defaultTime = time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

// Export is the content of one Kraken ledger export file.
type Export struct {
	// FileName is the file name. The default is DefaultFileName.
	FileName string
	// Entries are the rows of the file, in the given order. A real export puts the oldest entry first.
	Entries []Entry
}

// Entry is one row of the ledger.
type Entry struct {
	// TxID is the ledger ID. The default is a different ID for each row.
	TxID string
	// RefID is the reference ID. The two rows of one trade have the same RefID. The default is a different ID for each row.
	RefID string
	// Time is the time in UTC. The default is a different minute for each row.
	Time time.Time
	// Type is for example "deposit", "trade" or "earn".
	Type string
	// Subtype is for example "tradespot", "reward" or "".
	Subtype string
	// Aclass is the asset class. The default is "currency".
	Aclass string
	// Subclass is the default "fiat" for EUR and "crypto" for each other asset.
	Subclass string
	// Asset is for example "EUR" or "BTC".
	Asset string
	// Wallet is the default DefaultWallet.
	Wallet string
	// Amount is the decimal amount in the Asset, with its sign.
	Amount string
	// Fee is the decimal fee in the Asset. The default is "0".
	Fee string
	// Balance is the balance of the Asset after the entry. The default is the running sum of the amounts minus the fees.
	Balance string
	// FeeCurrency is the currency of the fee. The default is the Asset.
	FeeCurrency string
}

// Deposit returns a deposit of the EUR amount.
func Deposit(refID string, at time.Time, amount string) Entry {
	return Entry{RefID: refID, Time: at, Type: "deposit", Asset: "EUR", Amount: amount}
}

// Purchase returns the two rows of a spot purchase: the EUR row with the negative amount eur,
// and the row of the crypto asset with the positive amount and the fee in that asset.
func Purchase(refID string, at time.Time, eur, asset, amount, fee string) []Entry {
	return []Entry{
		{RefID: refID, Time: at, Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: eur},
		{RefID: refID, Time: at, Type: "trade", Subtype: "tradespot", Asset: asset, Amount: amount, Fee: fee},
	}
}

// Build returns the bytes of a CSV file in the layout of a Kraken ledger export.
// It stops the test if an amount or a fee is not a decimal.
func Build(tb testing.TB, e Export) []byte {
	tb.Helper()

	data, err := BuildFile(e)
	require.NoError(tb, err, "build kraken export fixture")
	return data
}

// Name returns the file name of the export.
func (e Export) Name() string {
	if e.FileName == "" {
		return DefaultFileName
	}
	return e.FileName
}

// BuildFile builds the CSV file of the export, with defaults for the fields that e leaves empty.
// Code outside tests, for example the demo data command, calls it.
func BuildFile(e Export) ([]byte, error) {
	var buf bytes.Buffer
	writeQuoted(&buf, Header)
	balances := map[string]decimal.Decimal{}
	for i, entry := range e.Entries {
		row, err := entry.row(i, balances)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}
		writeQuoted(&buf, row)
	}
	return buf.Bytes(), nil
}

// row returns the fields of the entry with the defaults. balances are the running balances by asset.
func (entry Entry) row(i int, balances map[string]decimal.Decimal) ([]string, error) {
	at := entry.Time
	if at.IsZero() {
		at = defaultTime.Add(time.Duration(i) * time.Minute)
	}
	fee := or(entry.Fee, "0")
	balance := entry.Balance
	if balance == "" {
		amount, err := decimal.NewFromString(entry.Amount)
		if err != nil {
			return nil, fmt.Errorf("amount %q: %w", entry.Amount, err)
		}
		feeValue, err := decimal.NewFromString(fee)
		if err != nil {
			return nil, fmt.Errorf("fee %q: %w", fee, err)
		}
		balances[entry.Asset] = balances[entry.Asset].Add(amount).Sub(feeValue)
		balance = balances[entry.Asset].String()
	}
	subclass := entry.Subclass
	if subclass == "" {
		subclass = "crypto"
		if entry.Asset == "EUR" {
			subclass = "fiat"
		}
	}
	return []string{
		or(entry.TxID, fmt.Sprintf("L%05d-TEST-%06d", i+1, i+1)),
		or(entry.RefID, fmt.Sprintf("R%05d-TEST-%06d", i+1, i+1)),
		at.UTC().Format(TimeLayout),
		entry.Type,
		entry.Subtype,
		or(entry.Aclass, "currency"),
		subclass,
		entry.Asset,
		or(entry.Wallet, DefaultWallet),
		entry.Amount,
		fee,
		balance,
		"", "", "",
		or(entry.FeeCurrency, entry.Asset),
	}, nil
}

// writeQuoted writes one CSV row with each field in double quotes, like a real export.
func writeQuoted(buf *bytes.Buffer, fields []string) {
	for i, f := range fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		for _, r := range f {
			if r == '"' {
				buf.WriteByte('"')
			}
			buf.WriteRune(r)
		}
		buf.WriteByte('"')
	}
	buf.WriteByte('\n')
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
