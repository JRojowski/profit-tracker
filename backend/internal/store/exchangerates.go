package store

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// ExchangeRate is one stored Exchange Rate of NBP table A.
type ExchangeRate struct {
	// Currency is the ISO code, for example "USD".
	Currency string
	// Date is the table date in Polish time, at midnight UTC.
	Date time.Time
	// Rate is the PLN amount for 1 unit of the currency.
	Rate decimal.Decimal
	// TableNumber is the NBP table number, for example "204/A/NBP/2026".
	TableNumber string
	// FetchedAt is the time when the tool got the rate, in UTC.
	FetchedAt time.Time
}

// ExchangeRateRange is the range of days that the tool asked the NBP for, for one currency.
type ExchangeRateRange struct {
	Currency string
	// FirstDay and LastDay are days in Polish time, at midnight UTC. The range includes both days.
	FirstDay, LastDay time.Time
	// FetchedAt is the time of the last fetch of the currency, in UTC.
	FetchedAt time.Time
}

// SaveExchangeRates stores the rates that the tool got for the asked range, in one transaction.
// It replaces a stored rate of the same currency and table date.
// It adds the asked days to the stored range of the currency and sets the time of the last fetch.
func (s *Store) SaveExchangeRates(ctx context.Context, asked ExchangeRateRange, rates []ExchangeRate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin exchange rate save of %s: %w", asked.Currency, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	for _, r := range rates {
		err := q.UpsertExchangeRate(ctx, queries.UpsertExchangeRateParams{
			Currency:    r.Currency,
			Date:        r.Date.Format(dateLayout),
			Rate:        r.Rate,
			TableNumber: r.TableNumber,
			FetchedAt:   r.FetchedAt.UTC().Format(timeLayout),
		})
		if err != nil {
			return fmt.Errorf("save exchange rate of %s on %s: %w", r.Currency, r.Date.Format(dateLayout), err)
		}
	}
	err = q.UpsertExchangeRateRange(ctx, queries.UpsertExchangeRateRangeParams{
		Currency:  asked.Currency,
		FirstDay:  asked.FirstDay.Format(dateLayout),
		LastDay:   asked.LastDay.Format(dateLayout),
		FetchedAt: asked.FetchedAt.UTC().Format(timeLayout),
	})
	if err != nil {
		return fmt.Errorf("save exchange rate range of %s: %w", asked.Currency, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit exchange rate save of %s: %w", asked.Currency, err)
	}
	return nil
}

// ExchangeRateRanges returns the stored range of each currency, by currency.
// A currency that the tool never asked for is not in the map.
func (s *Store) ExchangeRateRanges(ctx context.Context) (map[string]ExchangeRateRange, error) {
	rows, err := queries.New(s.db).ExchangeRateRanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("read exchange rate ranges: %w", err)
	}
	ranges := make(map[string]ExchangeRateRange, len(rows))
	for _, row := range rows {
		first, err := time.Parse(dateLayout, row.FirstDay)
		if err != nil {
			return nil, fmt.Errorf("exchange rate range of %s: parse first day %q: %w", row.Currency, row.FirstDay, err)
		}
		last, err := time.Parse(dateLayout, row.LastDay)
		if err != nil {
			return nil, fmt.Errorf("exchange rate range of %s: parse last day %q: %w", row.Currency, row.LastDay, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("exchange rate range of %s: parse fetch time %q: %w", row.Currency, row.FetchedAt, err)
		}
		ranges[row.Currency] = ExchangeRateRange{
			Currency: row.Currency, FirstDay: first, LastDay: last, FetchedAt: fetchedAt.UTC(),
		}
	}
	return ranges, nil
}

// LatestExchangeRates returns the stored rate with the newest table date of each currency, by currency.
// A currency with no stored rate is not in the map.
func (s *Store) LatestExchangeRates(ctx context.Context) (map[string]ExchangeRate, error) {
	rows, err := queries.New(s.db).LatestExchangeRates(ctx)
	if err != nil {
		return nil, fmt.Errorf("read latest exchange rates: %w", err)
	}
	rates := make(map[string]ExchangeRate, len(rows))
	for _, row := range rows {
		date, err := time.Parse(dateLayout, row.Date)
		if err != nil {
			return nil, fmt.Errorf("exchange rate of %s: parse date %q: %w", row.Currency, row.Date, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("exchange rate of %s: parse fetch time %q: %w", row.Currency, row.FetchedAt, err)
		}
		rates[row.Currency] = ExchangeRate{
			Currency: row.Currency, Date: date, Rate: row.Rate, TableNumber: row.TableNumber, FetchedAt: fetchedAt.UTC(),
		}
	}
	return rates, nil
}
