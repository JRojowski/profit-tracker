package store

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// dateLayout is the text form of a price date in the database.
const dateLayout = time.DateOnly

// Price is one stored close price of an Instrument.
type Price struct {
	// Ticker is the broker ticker of the Instrument.
	Ticker string
	// Date is the price date at midnight UTC.
	Date time.Time
	// Close is the close price in Currency.
	Close    decimal.Decimal
	Currency string
	// Source is the price provider, for example "yahoo".
	Source string
	// FetchedAt is the time when the tool got the price, in UTC.
	FetchedAt time.Time
}

// QuoteCurrencies returns the quote currency of each Instrument, by broker ticker.
// The value is empty for an Instrument with no known quote currency.
func (s *Store) QuoteCurrencies(ctx context.Context) (map[string]string, error) {
	rows, err := queries.New(s.db).InstrumentQuoteCurrencies(ctx)
	if err != nil {
		return nil, fmt.Errorf("read quote currencies: %w", err)
	}
	currencies := make(map[string]string, len(rows))
	for _, row := range rows {
		currencies[row.Ticker] = row.QuoteCurrency.String
	}
	return currencies, nil
}

// SavePrice stores the price. It replaces a stored price of the same Instrument and price date.
// The Instrument must exist.
func (s *Store) SavePrice(ctx context.Context, p Price) error {
	err := queries.New(s.db).UpsertPrice(ctx, queries.UpsertPriceParams{
		Ticker:    p.Ticker,
		Date:      p.Date.Format(dateLayout),
		Close:     p.Close,
		Currency:  p.Currency,
		Source:    p.Source,
		FetchedAt: p.FetchedAt.UTC().Format(timeLayout),
	})
	if err != nil {
		return fmt.Errorf("save price of %s on %s: %w", p.Ticker, p.Date.Format(dateLayout), err)
	}
	return nil
}

// LastPrices returns the stored price with the newest price date of each Instrument, by broker ticker.
// An Instrument with no stored price is not in the map.
func (s *Store) LastPrices(ctx context.Context) (map[string]Price, error) {
	rows, err := queries.New(s.db).LastPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("read last prices: %w", err)
	}
	prices := make(map[string]Price, len(rows))
	for _, row := range rows {
		date, err := time.Parse(dateLayout, row.Date)
		if err != nil {
			return nil, fmt.Errorf("price of %s: parse date %q: %w", row.Ticker, row.Date, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("price of %s: parse fetch time %q: %w", row.Ticker, row.FetchedAt, err)
		}
		prices[row.Ticker] = Price{
			Ticker: row.Ticker, Date: date, Close: row.Close, Currency: row.Currency, Source: row.Source,
			FetchedAt: fetchedAt.UTC(),
		}
	}
	return prices, nil
}

// TickersFetchedSince returns the broker tickers of the Instruments with a price that the tool fetched
// at the time since or later.
func (s *Store) TickersFetchedSince(ctx context.Context, since time.Time) (map[string]bool, error) {
	rows, err := queries.New(s.db).TickersFetchedSince(ctx, since.UTC().Format(timeLayout))
	if err != nil {
		return nil, fmt.Errorf("read prices fetched since %s: %w", since.UTC().Format(timeLayout), err)
	}
	tickers := make(map[string]bool, len(rows))
	for _, ticker := range rows {
		tickers[ticker] = true
	}
	return tickers, nil
}
