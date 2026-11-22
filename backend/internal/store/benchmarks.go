package store

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// BenchmarkPrice is one stored daily close of a Benchmark.
type BenchmarkPrice struct {
	// BenchmarkID is the identifier of the Benchmark, for example "sp500".
	BenchmarkID string
	// Date is the price date at midnight UTC.
	Date time.Time
	// Close is the split-adjusted close price in Currency (ADR 0008).
	Close    decimal.Decimal
	Currency string
	// Source is the price provider, for example "yahoo".
	Source string
	// FetchedAt is the time when the tool got the close, in UTC.
	FetchedAt time.Time
}

// BenchmarkPriceRange is the range of days that the tool asked the price source for, for one Benchmark.
type BenchmarkPriceRange struct {
	BenchmarkID string
	// FirstDay and LastDay are at midnight UTC. The range includes both days.
	FirstDay, LastDay time.Time
	// FetchedAt is the time of the last fetch of the Benchmark, in UTC.
	FetchedAt time.Time
}

// SaveBenchmarkPrices stores the closes that the tool got for the asked range of one Benchmark, in one transaction.
// It never replaces a stored close. It adds the asked days to the stored range of the Benchmark.
func (s *Store) SaveBenchmarkPrices(ctx context.Context, asked BenchmarkPriceRange, prices []BenchmarkPrice) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin benchmark price save of %s: %w", asked.BenchmarkID, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	for _, p := range prices {
		err := q.InsertBenchmarkPrice(ctx, queries.InsertBenchmarkPriceParams{
			BenchmarkID: p.BenchmarkID,
			Date:        p.Date.Format(dateLayout),
			Close:       p.Close,
			Currency:    p.Currency,
			Source:      p.Source,
			FetchedAt:   p.FetchedAt.UTC().Format(timeLayout),
		})
		if err != nil {
			return fmt.Errorf("save benchmark price of %s on %s: %w", p.BenchmarkID, p.Date.Format(dateLayout), err)
		}
	}
	err = q.UpsertBenchmarkPriceRange(ctx, queries.UpsertBenchmarkPriceRangeParams{
		BenchmarkID: asked.BenchmarkID,
		FirstDay:    asked.FirstDay.Format(dateLayout),
		LastDay:     asked.LastDay.Format(dateLayout),
		FetchedAt:   asked.FetchedAt.UTC().Format(timeLayout),
	})
	if err != nil {
		return fmt.Errorf("save benchmark price range of %s: %w", asked.BenchmarkID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit benchmark price save of %s: %w", asked.BenchmarkID, err)
	}
	return nil
}

// BenchmarkPriceRanges returns the stored range of each Benchmark, by Benchmark identifier.
// A Benchmark that the tool never asked for is not in the map.
func (s *Store) BenchmarkPriceRanges(ctx context.Context) (map[string]BenchmarkPriceRange, error) {
	rows, err := queries.New(s.db).BenchmarkPriceRanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("read benchmark price ranges: %w", err)
	}
	ranges := make(map[string]BenchmarkPriceRange, len(rows))
	for _, row := range rows {
		first, err := time.Parse(dateLayout, row.FirstDay)
		if err != nil {
			return nil, fmt.Errorf("benchmark price range of %s: parse first day %q: %w", row.BenchmarkID, row.FirstDay, err)
		}
		last, err := time.Parse(dateLayout, row.LastDay)
		if err != nil {
			return nil, fmt.Errorf("benchmark price range of %s: parse last day %q: %w", row.BenchmarkID, row.LastDay, err)
		}
		fetchedAt, err := time.Parse(timeLayout, row.FetchedAt)
		if err != nil {
			return nil, fmt.Errorf("benchmark price range of %s: parse fetch time %q: %w", row.BenchmarkID, row.FetchedAt, err)
		}
		ranges[row.BenchmarkID] = BenchmarkPriceRange{
			BenchmarkID: row.BenchmarkID, FirstDay: first, LastDay: last, FetchedAt: fetchedAt.UTC(),
		}
	}
	return ranges, nil
}

// BenchmarkPrices returns the stored closes of each Benchmark in date order, by Benchmark identifier.
// The result has no Source and no FetchedAt.
func (s *Store) BenchmarkPrices(ctx context.Context) (map[string][]BenchmarkPrice, error) {
	rows, err := queries.New(s.db).BenchmarkPrices(ctx)
	if err != nil {
		return nil, fmt.Errorf("read benchmark prices: %w", err)
	}
	prices := map[string][]BenchmarkPrice{}
	for _, row := range rows {
		date, err := time.Parse(dateLayout, row.Date)
		if err != nil {
			return nil, fmt.Errorf("benchmark price of %s: parse date %q: %w", row.BenchmarkID, row.Date, err)
		}
		prices[row.BenchmarkID] = append(prices[row.BenchmarkID], BenchmarkPrice{
			BenchmarkID: row.BenchmarkID, Date: date, Close: row.Close, Currency: row.Currency,
		})
	}
	return prices, nil
}

// FirstCashOperationTime returns the time of the first Cash Operation of all Accounts, in UTC.
// It returns false if no Account has a Cash Operation.
func (s *Store) FirstCashOperationTime(ctx context.Context) (time.Time, bool, error) {
	text, err := queries.New(s.db).FirstCashOperationTime(ctx)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read first cash operation: %w", err)
	}
	if text == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(timeLayout, text)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("first cash operation: parse time %q: %w", text, err)
	}
	return t.UTC(), true, nil
}
