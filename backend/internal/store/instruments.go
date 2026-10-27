package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Instrument is the price source of one Instrument: its Quote Currency and its manual price ticker.
// It also holds the state of the price check of the Instrument.
type Instrument struct {
	// Ticker is the broker ticker.
	Ticker string
	// Name is the Instrument name from the broker export.
	Name string
	// QuoteCurrency is the currency of the price at the exchange, or empty if it is not known.
	QuoteCurrency string
	// QuoteCurrencyManual tells that the user set QuoteCurrency. Otherwise it comes from the ticker suffix.
	QuoteCurrencyManual bool
	// PriceTicker is the manual price ticker, or empty if the tool uses the default mapping.
	PriceTicker string
	// SourceCurrency is the currency that the price source gave in its last answer with a price,
	// or empty if it is not known.
	SourceCurrency string
	// NoData tells that the last answer of the price source was "no data" for the price ticker.
	NoData bool
	// PriceConfirmed tells that the user confirmed that the price ticker and the Quote Currency are correct.
	PriceConfirmed bool
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
// If deletePrices is true, it also deletes the stored prices and the asked range of the Instrument,
// and clears the state of the price check, in the same transaction.
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
		if row, err = q.ResetPriceCheck(ctx, ticker); err != nil {
			return Instrument{}, fmt.Errorf("reset price check of %s: %w", ticker, err)
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
// the prices in the Quote Currency. A change of the Quote Currency removes the price confirmation.
func (s *Store) SetQuoteCurrency(ctx context.Context, ticker, quoteCurrency string, manual bool) (Instrument, error) {
	row, err := queries.New(s.db).SetQuoteCurrency(ctx, queries.SetQuoteCurrencyParams{
		QuoteCurrency:       sql.NullString{String: quoteCurrency, Valid: quoteCurrency != ""},
		QuoteCurrencyManual: boolFlag(manual),
		Ticker:              ticker,
	})
	if err != nil {
		return Instrument{}, instrumentError(ticker, "set quote currency of", err)
	}
	return toInstrument(row), nil
}

// SetPriceConfirmed sets or removes the confirmation of the user that the price ticker and the Quote Currency
// of the Instrument are correct. It returns the Instrument, or ErrNotFound.
func (s *Store) SetPriceConfirmed(ctx context.Context, ticker string, confirmed bool) (Instrument, error) {
	row, err := queries.New(s.db).SetPriceConfirmed(ctx, queries.SetPriceConfirmedParams{
		PriceConfirmed: boolFlag(confirmed),
		Ticker:         ticker,
	})
	if err != nil {
		return Instrument{}, instrumentError(ticker, "set price confirmation of", err)
	}
	return toInstrument(row), nil
}

// SetSourceCurrency records an answer of the price source with a price in the currency.
// It also clears "no data", because the answer tells that the price source has data for the price ticker.
func (s *Store) SetSourceCurrency(ctx context.Context, ticker, currency string) error {
	err := queries.New(s.db).SetSourceCurrency(ctx, queries.SetSourceCurrencyParams{
		SourceCurrency: sql.NullString{String: currency, Valid: currency != ""},
		Ticker:         ticker,
	})
	if err != nil {
		return fmt.Errorf("set source currency of %s: %w", ticker, err)
	}
	return nil
}

// SetPriceNoData records that the price source answered "no data" for the price ticker of the Instrument.
func (s *Store) SetPriceNoData(ctx context.Context, ticker string) error {
	if err := queries.New(s.db).SetPriceNoData(ctx, ticker); err != nil {
		return fmt.Errorf("set no data of %s: %w", ticker, err)
	}
	return nil
}

// boolFlag returns 1 for true and 0 for false, the form of a flag column.
func boolFlag(b bool) int64 {
	if b {
		return 1
	}
	return 0
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
		Name:                row.Name,
		QuoteCurrency:       row.QuoteCurrency.String,
		QuoteCurrencyManual: row.QuoteCurrencyManual == 1,
		PriceTicker:         row.PriceTicker.String,
		SourceCurrency:      row.SourceCurrency.String,
		NoData:              row.PriceNoData == 1,
		PriceConfirmed:      row.PriceConfirmed == 1,
	}
}
