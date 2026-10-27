package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Instrument is the price source of one Instrument: its Quote Currency and its manual price ticker.
type Instrument struct {
	// Ticker is the broker ticker.
	Ticker string
	// QuoteCurrency is the currency of the price at the exchange, or empty if it is not known.
	QuoteCurrency string
	// QuoteCurrencyManual tells that the user set QuoteCurrency. Otherwise it comes from the ticker suffix.
	QuoteCurrencyManual bool
	// PriceTicker is the manual price ticker, or empty if the tool uses the default mapping.
	PriceTicker string
}

// Instruments returns the price source of each Instrument, by broker ticker.
func (s *Store) Instruments(ctx context.Context) (map[string]Instrument, error) {
	rows, err := queries.New(s.db).InstrumentPriceSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("read instruments: %w", err)
	}
	instruments := make(map[string]Instrument, len(rows))
	for _, row := range rows {
		instruments[row.Ticker] = toInstrument(row)
	}
	return instruments, nil
}

// Instrument returns the price source of the Instrument with the broker ticker, or ErrNotFound.
func (s *Store) Instrument(ctx context.Context, ticker string) (Instrument, error) {
	row, err := queries.New(s.db).InstrumentPriceSource(ctx, ticker)
	if err != nil {
		return Instrument{}, instrumentError(ticker, "read instrument", err)
	}
	return toInstrument(row), nil
}

// SetPriceTicker sets the manual price ticker of the Instrument and returns the Instrument, or ErrNotFound.
// An empty priceTicker removes the manual price ticker.
// If deletePrices is true, it also deletes the stored prices and the asked range of the Instrument
// in the same transaction.
func (s *Store) SetPriceTicker(ctx context.Context, ticker, priceTicker string, deletePrices bool) (Instrument, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Instrument{}, fmt.Errorf("begin price ticker change of %s: %w", ticker, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	row, err := q.SetPriceTicker(ctx, queries.SetPriceTickerParams{
		PriceTicker: sql.NullString{String: priceTicker, Valid: priceTicker != ""},
		Ticker:      ticker,
	})
	if err != nil {
		return Instrument{}, instrumentError(ticker, "set price ticker of", err)
	}
	if deletePrices {
		if err := q.DeleteInstrumentPrices(ctx, ticker); err != nil {
			return Instrument{}, fmt.Errorf("delete prices of %s: %w", ticker, err)
		}
		if err := q.DeleteInstrumentPriceRange(ctx, ticker); err != nil {
			return Instrument{}, fmt.Errorf("delete price range of %s: %w", ticker, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Instrument{}, fmt.Errorf("commit price ticker change of %s: %w", ticker, err)
	}
	return toInstrument(row), nil
}

// SetQuoteCurrency sets the Quote Currency of the Instrument and returns the Instrument, or ErrNotFound.
// manual tells that the user set the value. An empty quoteCurrency tells that the Quote Currency is not known.
// It keeps the stored prices. A price in another currency gives no value, because the Holdings use only
// the prices in the Quote Currency.
func (s *Store) SetQuoteCurrency(ctx context.Context, ticker, quoteCurrency string, manual bool) (Instrument, error) {
	var flag int64
	if manual {
		flag = 1
	}
	row, err := queries.New(s.db).SetQuoteCurrency(ctx, queries.SetQuoteCurrencyParams{
		QuoteCurrency:       sql.NullString{String: quoteCurrency, Valid: quoteCurrency != ""},
		QuoteCurrencyManual: flag,
		Ticker:              ticker,
	})
	if err != nil {
		return Instrument{}, instrumentError(ticker, "set quote currency of", err)
	}
	return toInstrument(row), nil
}

// instrumentError wraps err of the action on the Instrument with the broker ticker.
// It gives ErrNotFound if the Instrument does not exist.
func instrumentError(ticker, action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("instrument %s: %w", ticker, ErrNotFound)
	}
	return fmt.Errorf("%s %s: %w", action, ticker, err)
}

func toInstrument(row queries.Instrument) Instrument {
	return Instrument{
		Ticker:              row.Ticker,
		QuoteCurrency:       row.QuoteCurrency.String,
		QuoteCurrencyManual: row.QuoteCurrencyManual == 1,
		PriceTicker:         row.PriceTicker.String,
	}
}
