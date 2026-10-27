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

// SavePrice stores the current price. It replaces a stored current price of the same Instrument and price date.
// ADR 0005: it does not replace a stored close of the daily history. The Instrument must exist.
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

// TickersFetchedSince returns the broker tickers of the Instruments with a current price that the tool fetched
// at the time since or later. A close of the daily history does not count.
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

// PriceRange is the range of days that the tool asked the price source for, for one Instrument.
type PriceRange struct {
	// Ticker is the broker ticker of the Instrument.
	Ticker string
	// FirstDay and LastDay are at midnight UTC. The range includes both days.
	FirstDay, LastDay time.Time
	// FetchedAt is the time of the last fetch of the Instrument, in UTC.
	FetchedAt time.Time
}

// SaveDailyPrices stores the closes of the daily history that the tool got for the asked range, in one transaction.
// ADR 0005: it never replaces a stored close of the daily history.
// It replaces a stored current price of the same day that the tool fetched on or before that day in UTC.
// That price can be a live price.
// It adds the asked days to the stored range of the Instrument and sets the time of the last fetch.
func (s *Store) SaveDailyPrices(ctx context.Context, asked PriceRange, prices []Price) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin daily price save of %s: %w", asked.Ticker, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	for _, p := range prices {
		err := q.InsertDailyPrice(ctx, queries.InsertDailyPriceParams{
			Ticker:    p.Ticker,
			Date:      p.Date.Format(dateLayout),
			Close:     p.Close,
			Currency:  p.Currency,
			Source:    p.Source,
			FetchedAt: p.FetchedAt.UTC().Format(timeLayout),
		})
		if err != nil {
			return fmt.Errorf("save daily price of %s on %s: %w", p.Ticker, p.Date.Format(dateLayout), err)
		}
	}
	err = q.UpsertPriceRange(ctx, queries.UpsertPriceRangeParams{
		Ticker:    asked.Ticker,
		FirstDay:  asked.FirstDay.Format(dateLayout),
		LastDay:   asked.LastDay.Format(dateLayout),
		FetchedAt: asked.FetchedAt.UTC().Format(timeLayout),
	})
	if err != nil {
		return fmt.Errorf("save price range of %s: %w", asked.Ticker, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit daily price save of %s: %w", asked.Ticker, err)
	}
	return nil
}

// PriceRanges returns the stored range of each Instrument, by broker ticker.
// An Instrument that the tool never asked for is not in the map.
func (s *Store) PriceRanges(ctx context.Context) (map[string]PriceRange, error) {
	rows, err := queries.New(s.db).PriceRanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("read price ranges: %w", err)
	}
	ranges := make(map[string]PriceRange, len(rows))
	for _, row := range rows {
		first, err := time.Parse(dateLayout, row.FirstDay)
		if err != nil {
			return nil, fmt.Errorf("price range of %s: parse first day %q: %w", row.Ticker, row.FirstDay, err)
		}
		last, err := time.Parse(dateLayout, row.LastDay)
		if err != nil {
			return nil, fmt.Errorf("price range of %s: parse last day %q: %w", row.Ticker, row.LastDay, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("price range of %s: parse fetch time %q: %w", row.Ticker, row.FetchedAt, err)
		}
		ranges[row.Ticker] = PriceRange{Ticker: row.Ticker, FirstDay: first, LastDay: last, FetchedAt: fetchedAt.UTC()}
	}
	return ranges, nil
}

// PriceHistory returns all stored prices of each Instrument in date order, by broker ticker.
// A close of the daily history keeps the currency that the price source gave.
// Thus a price can be in a currency that is not the Quote Currency of the Instrument. The caller must check it.
func (s *Store) PriceHistory(ctx context.Context) (map[string][]Price, error) {
	rows, err := queries.New(s.db).Prices(ctx)
	if err != nil {
		return nil, fmt.Errorf("read prices: %w", err)
	}
	history := map[string][]Price{}
	for _, row := range rows {
		date, err := time.Parse(dateLayout, row.Date)
		if err != nil {
			return nil, fmt.Errorf("price of %s: parse date %q: %w", row.Ticker, row.Date, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("price of %s: parse fetch time %q: %w", row.Ticker, row.FetchedAt, err)
		}
		history[row.Ticker] = append(history[row.Ticker], Price{
			Ticker: row.Ticker, Date: date, Close: row.Close, Currency: row.Currency, Source: row.Source,
			FetchedAt: fetchedAt.UTC(),
		})
	}
	return history, nil
}

// FirstFillTimes returns the time of the first Fill of each Instrument on any Account, by broker ticker.
// The map has each Instrument that an Account ever held, also an Instrument that the Account sold.
func (s *Store) FirstFillTimes(ctx context.Context) (map[string]time.Time, error) {
	rows, err := queries.New(s.db).FirstFillTimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("read first fills: %w", err)
	}
	times := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		t, err := time.Parse(timeLayout, row.FirstTime)
		if err != nil {
			return nil, fmt.Errorf("first fill of %s: parse time %q: %w", row.Ticker, row.FirstTime, err)
		}
		times[row.Ticker] = t.UTC()
	}
	return times, nil
}
