package exchange

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// newSeriesDays is the number of days before the refresh day where the series of a new currency starts.
// Thus the series has a table before the first day that a lookup asks for, also after a weekend or a holiday.
const newSeriesDays = 7

// Service gets the Exchange Rates of the needed currencies from a Provider and stores them.
// It runs one refresh at a time.
type Service struct {
	store    *store.Store
	provider Provider
	now      func() time.Time
	// lock serializes the refreshes, because the refresh at start and a refresh by the user can overlap.
	// It is a channel with one slot and not a sync.Mutex, so that a refresh stops waiting when its context ends.
	lock chan struct{}
}

// NewService returns a Service that stores the rates in st. now is the clock that tells "today".
func NewService(st *store.Store, provider Provider, now func() time.Time) *Service {
	return &Service{store: st, provider: provider, now: now, lock: make(chan struct{}, 1)}
}

// Result is the result of one refresh.
type Result struct {
	// Stored is the number of stored rates.
	Stored int
	// Failures are the currencies that got no rate, in currency order.
	Failures []Failure
}

// Failure is one currency that got no rate in a refresh.
type Failure struct {
	// Currency is the ISO code of the currency, for example "USD".
	Currency string
	Err      error
}

// NoRateError tells that the tool has no stored rate of a needed currency after a refresh, although the NBP
// answered with no error. For example, the NBP table A has no rate for the currency.
type NoRateError struct {
	Currency string
}

func (e *NoRateError) Error() string {
	return "the NBP has no Exchange Rate for " + e.Currency
}

// Latest is the latest stored Exchange Rate of one currency.
type Latest struct {
	Rate
	// LastFetch is the time of the last fetch of the currency, in UTC. It can be later than the fetch of the rate,
	// because a fetch for days with no table stores no rate.
	LastFetch time.Time
}

// Refresh gets the missing rates of each needed currency: the days from the last asked day to today.
// The last asked day comes again, because the NBP publishes the table of a day at about 12:00 in Polish time.
// A second refresh on the same day asks for nothing. A failure of one currency does not stop the others.
// The result names each needed currency that has no stored rate after the refresh.
// Refresh returns an error if it cannot read or store data, or if ctx ends. Then the result holds the rates
// that it stored before the error.
func (s *Service) Refresh(ctx context.Context) (Result, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer s.release()

	needed, err := s.neededCurrencies(ctx)
	if err != nil {
		return Result{}, err
	}
	ranges, err := s.store.ExchangeRateRanges(ctx)
	if err != nil {
		return Result{}, err
	}
	return s.refresh(ctx, needed, ranges)
}

// RefreshIfStale runs a refresh if the last fetch of a needed currency is before today, in Polish time,
// or if the tool never asked for a needed currency. It tells if the refresh ran.
func (s *Service) RefreshIfStale(ctx context.Context) (Result, bool, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, false, err
	}
	defer s.release()

	needed, err := s.neededCurrencies(ctx)
	if err != nil {
		return Result{}, false, err
	}
	ranges, err := s.store.ExchangeRateRanges(ctx)
	if err != nil {
		return Result{}, false, err
	}
	today := StartOfDayInWarsaw(s.now())
	stale := slices.ContainsFunc(needed, func(currency string) bool {
		r, ok := ranges[currency]
		return !ok || r.FetchedAt.Before(today)
	})
	if !stale {
		return Result{}, false, nil
	}
	result, err := s.refresh(ctx, needed, ranges)
	return result, true, err
}

// LatestRates returns the latest stored rate of each currency, by currency.
// A currency with no stored rate is not in the map.
func (s *Service) LatestRates(ctx context.Context) (map[string]Latest, error) {
	rates, err := s.store.LatestExchangeRates(ctx)
	if err != nil {
		return nil, err
	}
	ranges, err := s.store.ExchangeRateRanges(ctx)
	if err != nil {
		return nil, err
	}
	latest := make(map[string]Latest, len(rates))
	for currency, r := range rates {
		lastFetch := r.FetchedAt
		if rg, ok := ranges[currency]; ok {
			lastFetch = rg.FetchedAt
		}
		latest[currency] = Latest{Rate: Rate{Date: r.Date, Mid: r.Rate, Table: r.TableNumber}, LastFetch: lastFetch}
	}
	return latest, nil
}

// acquire waits until no other refresh runs. It returns the error of ctx if ctx ends first.
func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for the running exchange rate refresh: %w", ctx.Err())
	}
}

// release lets the next refresh run.
func (s *Service) release() {
	<-s.lock
}

// refresh gets and stores the missing rates of each needed currency. It stops when ctx ends.
func (s *Service) refresh(ctx context.Context, needed []string, ranges map[string]store.ExchangeRateRange) (Result, error) {
	var result Result
	now := s.now()
	today := DayInWarsaw(now)
	for _, currency := range needed {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("refresh exchange rates: %w", err)
		}
		from := today.AddDate(0, 0, -newSeriesDays)
		if r, ok := ranges[currency]; ok {
			if !r.LastDay.Before(today) {
				continue
			}
			from = r.LastDay
		}
		rates, err := s.provider.Rates(ctx, currency, from, today)
		if err != nil {
			// A provider error that comes from the end of ctx is not a failure of the currency.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, fmt.Errorf("refresh exchange rates: %w", ctxErr)
			}
			result.Failures = append(result.Failures, Failure{Currency: currency, Err: err})
			continue
		}
		asked := store.ExchangeRateRange{Currency: currency, FirstDay: from, LastDay: today, FetchedAt: now}
		if err := s.store.SaveExchangeRates(ctx, asked, toStored(currency, rates, now)); err != nil {
			return result, err
		}
		result.Stored += len(rates)
	}
	if err := s.addMissingRates(ctx, needed, &result); err != nil {
		return result, err
	}
	return result, nil
}

// addMissingRates adds a failure for each needed currency that has no stored rate and no failure yet.
// It keeps the failures in currency order.
func (s *Service) addMissingRates(ctx context.Context, needed []string, result *Result) error {
	stored, err := s.store.LatestExchangeRates(ctx)
	if err != nil {
		return err
	}
	for _, currency := range needed {
		_, ok := stored[currency]
		failed := slices.ContainsFunc(result.Failures, func(f Failure) bool { return f.Currency == currency })
		if !ok && !failed {
			result.Failures = append(result.Failures, Failure{Currency: currency, Err: &NoRateError{Currency: currency}})
		}
	}
	slices.SortFunc(result.Failures, func(a, b Failure) int { return cmp.Compare(a.Currency, b.Currency) })
	return nil
}

// toStored returns the rates of the currency in the form of the store.
func toStored(currency string, rates []Rate, fetchedAt time.Time) []store.ExchangeRate {
	stored := make([]store.ExchangeRate, 0, len(rates))
	for _, r := range rates {
		stored = append(stored, store.ExchangeRate{
			Currency: currency, Date: r.Date, Rate: r.Mid, TableNumber: r.Table, FetchedAt: fetchedAt,
		})
	}
	return stored
}

// neededCurrencies returns the currencies that need a rate, in order: the Quote Currency that is not PLN of each
// Instrument in a Holding, and the Account currency that is not PLN of each Account that holds an Instrument
// quoted in PLN.
func (s *Service) neededCurrencies(ctx context.Context) ([]string, error) {
	held, err := price.HeldInstruments(ctx, s.store)
	if err != nil {
		return nil, err
	}
	var needed []string
	for _, h := range held {
		if h.QuoteCurrency != ledger.PLN {
			needed = append(needed, h.QuoteCurrency)
			continue
		}
		for _, account := range h.AccountCurrencies {
			if account != ledger.PLN {
				needed = append(needed, account)
			}
		}
	}
	slices.Sort(needed)
	return slices.Compact(needed), nil
}
