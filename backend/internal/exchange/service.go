package exchange

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// newSeriesDays is the number of days before the first needed day where a series starts.
// The first needed day is the earlier of two days. One is the day of the earliest Net Deposits flow
// of an Account in the currency. The other is the day of the first Fill of an Instrument with the currency
// as its Quote Currency.
// For a currency with neither day, the series starts 7 days before the day of the first refresh.
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
// When a new Import added an earlier Net Deposits flow, the first day of a series can be after the first day
// that the tool needs. Then Refresh also gets the days from the needed first day to the stored first day.
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

// RefreshIfStale runs a refresh in these cases, and it tells if the refresh ran:
//   - The last fetch of a needed currency is before today, in Polish time.
//   - The tool never asked for a needed currency.
//   - The first day of a series is after the first day that the tool needs.
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
	stale := slices.ContainsFunc(needed, func(n neededCurrency) bool {
		r, ok := ranges[n.currency]
		return !ok || r.FetchedAt.Before(today) || n.hasFirstDay && n.firstDay.Before(r.FirstDay)
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
func (s *Service) refresh(ctx context.Context, needed []neededCurrency, ranges map[string]store.ExchangeRateRange) (Result, error) {
	var result Result
	now := s.now()
	today := DayInWarsaw(now)
	currencies := make([]string, 0, len(needed))
	for _, n := range needed {
		currencies = append(currencies, n.currency)
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("refresh exchange rates: %w", err)
		}
		for _, asked := range missingDays(n, ranges, today, now) {
			rates, err := s.provider.Rates(ctx, n.currency, asked.FirstDay, asked.LastDay)
			if err != nil {
				// A provider error that comes from the end of ctx is not a failure of the currency.
				if ctxErr := ctx.Err(); ctxErr != nil {
					return result, fmt.Errorf("refresh exchange rates: %w", ctxErr)
				}
				result.Failures = append(result.Failures, Failure{Currency: n.currency, Err: err})
				break
			}
			if err := s.store.SaveExchangeRates(ctx, asked, toStored(n.currency, rates, now)); err != nil {
				return result, err
			}
			result.Stored += len(rates)
		}
	}
	if err := s.addMissingRates(ctx, currencies, &result); err != nil {
		return result, err
	}
	return result, nil
}

// missingDays returns the ranges of days that the series of the currency needs and that the tool did not ask for.
//
//   - A new series goes from the needed first day, or from 7 days before today, to today.
//   - When the stored first day is after the needed first day, the tool asks for the days between them.
//     This range keeps the time of the last fetch, because it does not make the end of the series current.
//   - When the stored last day is before today, the tool asks again for the last day and the days after it.
//
// The range before the stored first day comes first. Thus the range to today sets the time of the last fetch.
func missingDays(n neededCurrency, ranges map[string]store.ExchangeRateRange, today, now time.Time) []store.ExchangeRateRange {
	firstDay := today.AddDate(0, 0, -newSeriesDays)
	if n.hasFirstDay {
		firstDay = n.firstDay
	}
	// The data can have a flow that is later than the clock of the tool, for example in a test.
	// The tool does not ask for days after today.
	if firstDay.After(today) {
		firstDay = today
	}
	r, ok := ranges[n.currency]
	if !ok {
		return []store.ExchangeRateRange{{Currency: n.currency, FirstDay: firstDay, LastDay: today, FetchedAt: now}}
	}
	var missing []store.ExchangeRateRange
	if n.hasFirstDay && firstDay.Before(r.FirstDay) {
		missing = append(missing, store.ExchangeRateRange{
			Currency: n.currency, FirstDay: firstDay, LastDay: r.FirstDay.AddDate(0, 0, -1), FetchedAt: r.FetchedAt,
		})
	}
	if r.LastDay.Before(today) {
		missing = append(missing, store.ExchangeRateRange{Currency: n.currency, FirstDay: r.LastDay, LastDay: today, FetchedAt: now})
	}
	return missing
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

// neededCurrency is one currency that needs a rate, with the first day that its series needs.
type neededCurrency struct {
	currency string
	// firstDay is 7 days before the first needed day of the currency, in Polish time at midnight UTC.
	// See newSeriesDays. hasFirstDay is false if the currency has no needed day.
	firstDay    time.Time
	hasFirstDay bool
}

// neededCurrencies returns the currencies that need a rate, in currency order:
//   - the Account currency that is not PLN of each Account, for the view of all Accounts (ADR 0004)
//   - the Quote Currency that is not PLN of each Instrument that an Account ever held.
//     The value of its Holding today and in the Value History needs the rate.
func (s *Service) neededCurrencies(ctx context.Context) ([]neededCurrency, error) {
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	firstFills, err := s.store.FirstFillTimes(ctx)
	if err != nil {
		return nil, err
	}
	instruments, err := s.store.Instruments(ctx)
	if err != nil {
		return nil, err
	}
	firstFlows, err := s.store.EarliestFlowTimes(ctx)
	if err != nil {
		return nil, err
	}
	// first is the earliest needed day of each currency. A currency with no known day has the zero time.
	first := map[string]time.Time{}
	add := func(currency string) bool {
		if currency == "" || currency == ledger.PLN {
			return false
		}
		if _, ok := first[currency]; !ok {
			first[currency] = time.Time{}
		}
		return true
	}
	addDay := func(currency string, t time.Time) {
		if !add(currency) {
			return
		}
		if day := DayInWarsaw(t); first[currency].IsZero() || day.Before(first[currency]) {
			first[currency] = day
		}
	}
	for _, acc := range accounts {
		if t, ok := firstFlows[acc.Currency]; ok {
			addDay(acc.Currency, t)
		} else {
			add(acc.Currency)
		}
	}
	for ticker, t := range firstFills {
		addDay(instruments[ticker].QuoteCurrency, t)
	}

	needed := make([]neededCurrency, 0, len(first))
	for _, currency := range slices.Sorted(maps.Keys(first)) {
		n := neededCurrency{currency: currency}
		if day := first[currency]; !day.IsZero() {
			n.firstDay, n.hasFirstDay = day.AddDate(0, 0, -newSeriesDays), true
		}
		needed = append(needed, n)
	}
	return needed, nil
}
