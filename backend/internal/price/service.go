package price

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// Service gets prices from a Provider and stores them. It runs one refresh at a time.
// It gets the current prices of the Instruments in Holdings.
// It also gets the daily history of each Instrument that an Account ever held.
type Service struct {
	store    *store.Store
	provider Provider
	now      func() time.Time
	// pause runs before each history request. It can be nil.
	pause func(ctx context.Context) error
	// lock serializes the refreshes, because the refresh at start and a refresh by the user can overlap.
	// It is a channel with one slot and not a sync.Mutex, so that a refresh stops waiting when its context ends.
	lock chan struct{}

	// progressMu guards progress.
	progressMu sync.Mutex
	progress   Progress
}

// NewService returns a Service that stores the prices in st. now is the clock that tells "today".
// pause runs before each history request, so that the requests do not come too fast for the price source.
// Give PauseBetweenRequests in production. A nil pause does not wait.
func NewService(st *store.Store, provider Provider, now func() time.Time, pause func(ctx context.Context) error) *Service {
	return &Service{store: st, provider: provider, now: now, pause: pause, lock: make(chan struct{}, 1)}
}

// Result is the result of one refresh.
type Result struct {
	// Refreshed is the number of stored current prices.
	Refreshed int
	// Failures are the Instruments that got no current price, in broker ticker order.
	Failures []Failure
	// HistoryStored is the number of stored daily closes.
	HistoryStored int
	// HistoryFailures are the Instruments that got no history, in broker ticker order.
	HistoryFailures []Failure
	// TooManyRequests tells that the price source answered "Too Many Requests" and the refresh stopped.
	// The prices that the refresh stored before the stop stay.
	TooManyRequests bool
}

// Progress is the state of the running refresh.
type Progress struct {
	// Running tells that a refresh runs.
	Running bool
	// Done is the number of history requests that the refresh completed. Total is the number of history
	// requests of the refresh. Both are 0 before the history step.
	Done, Total int
	// Ticker is the broker ticker of the Instrument of the running history request, or empty.
	Ticker string
}

// Progress returns the state of the running refresh. It is the zero Progress when no refresh runs.
func (s *Service) Progress() Progress {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	return s.progress
}

// setProgress runs change on the progress, with the lock.
func (s *Service) setProgress(change func(p *Progress)) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	change(&s.progress)
}

// Failure is one Instrument that got no price in a refresh.
type Failure struct {
	// Ticker is the broker ticker of the Instrument.
	Ticker string
	Err    error
}

// CurrencyError tells that the provider gave a price in a currency that is not the Quote Currency of the Instrument.
type CurrencyError struct {
	// Got is the currency of the price. Want is the Quote Currency of the Instrument.
	Got, Want string
}

func (e *CurrencyError) Error() string {
	return fmt.Sprintf("the price is in %s, but the Instrument is quoted in %s", e.Got, e.Want)
}

// Refresh gets a current price for each Instrument that is in a Holding of an Account and has a Quote Currency.
// Then it gets the missing daily closes of each Instrument that an Account ever held and that has
// a Quote Currency. A failure of one Instrument does not stop the others. "Too Many Requests" from the price
// source stops the refresh, and the result tells it.
// Refresh returns an error if it cannot read or store data, or if ctx ends. Then the result holds the prices
// that it stored before the error.
func (s *Service) Refresh(ctx context.Context) (Result, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer s.release()

	held, err := HeldInstruments(ctx, s.store)
	if err != nil {
		return Result{}, err
	}
	plan, err := s.historyPlan(ctx)
	if err != nil {
		return Result{}, err
	}
	result, err := s.refresh(ctx, held)
	if err != nil || result.TooManyRequests {
		return result, err
	}
	err = s.refreshHistory(ctx, plan, &result)
	return result, err
}

// RefreshIfStale gets the current prices if a held Instrument with a Quote Currency has no current price
// fetched today, in UTC. Thus a later start of the same day tries again for the Instruments that failed.
// It tells if the refresh ran. It gets no daily history. Only Refresh gets the history.
func (s *Service) RefreshIfStale(ctx context.Context) (Result, bool, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, false, err
	}
	defer s.release()

	held, err := HeldInstruments(ctx, s.store)
	if err != nil {
		return Result{}, false, err
	}
	// A daily close does not tell that the current price is current. See TickersFetchedSince.
	fetched, err := s.store.TickersFetchedSince(ctx, StartOfDay(s.now()))
	if err != nil {
		return Result{}, false, err
	}
	if !slices.ContainsFunc(held, func(h HeldInstrument) bool { return !fetched[h.Ticker] }) {
		return Result{}, false, nil
	}
	result, err := s.refresh(ctx, held)
	return result, true, err
}

// acquire waits until no other refresh runs. It returns the error of ctx if ctx ends first.
// The progress tells that a refresh runs until release.
func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		s.setProgress(func(p *Progress) { *p = Progress{Running: true} })
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for the running price refresh: %w", ctx.Err())
	}
}

// release lets the next refresh run.
func (s *Service) release() {
	s.setProgress(func(p *Progress) { *p = Progress{} })
	<-s.lock
}

// StartOfDay returns the start of the day of t in UTC.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// refresh gets and stores a price for each held Instrument. It stops when ctx ends.
func (s *Service) refresh(ctx context.Context, held []HeldInstrument) (Result, error) {
	var result Result
	for _, h := range held {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("refresh prices: %w", err)
		}
		q, err := s.provider.LastClose(ctx, h.PriceTicker)
		if err == nil {
			// The price check needs the currency of the price source, also when it is not the Quote Currency.
			if err := s.store.SetSourceCurrency(ctx, h.Ticker, q.Currency); err != nil {
				return result, err
			}
			err = checkCurrency(q.Currency, h.QuoteCurrency)
		}
		if err != nil {
			if stop, err := stopOn(ctx, err, &result); stop {
				return result, err
			}
			if err := s.recordNoData(ctx, h.Ticker, err); err != nil {
				return result, err
			}
			result.Failures = append(result.Failures, Failure{Ticker: h.Ticker, Err: err})
			continue
		}
		err = s.store.SavePrice(ctx, store.Price{
			Ticker: h.Ticker, Date: q.Date, Close: q.Close, Currency: q.Currency, Source: q.Source, FetchedAt: s.now(),
		})
		if err != nil {
			return result, err
		}
		result.Refreshed++
	}
	return result, nil
}

// stopOn tells if the provider error err stops the refresh, and returns the error of the stop.
//   - A provider error that comes from the end of ctx is not a failure of the Instrument.
//     The refresh stops with the error of ctx.
//   - "Too Many Requests" stops the refresh with no error and sets result.TooManyRequests,
//     because more requests make the block of the price source longer.
//
// Otherwise it returns false and err, which is a failure of the Instrument.
func stopOn(ctx context.Context, err error, result *Result) (bool, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return true, fmt.Errorf("refresh prices: %w", ctxErr)
	}
	if errors.Is(err, ErrTooManyRequests) {
		result.TooManyRequests = true
		return true, nil
	}
	return false, err
}

// checkCurrency returns a *CurrencyError if the currency of a price is not the Quote Currency.
func checkCurrency(got, quoteCurrency string) error {
	if got != quoteCurrency {
		return &CurrencyError{Got: got, Want: quoteCurrency}
	}
	return nil
}

// recordNoData stores that the price source has no data for the price ticker, if err wraps ErrNoData.
// The price check uses it. Another error tells nothing about the price ticker, so it changes nothing.
func (s *Service) recordNoData(ctx context.Context, ticker string, err error) error {
	if !errors.Is(err, ErrNoData) {
		return nil
	}
	return s.store.SetPriceNoData(ctx, ticker)
}

// HeldInstrument is an Instrument in a Holding of an Account, with a known Quote Currency.
type HeldInstrument struct {
	// Ticker is the broker ticker.
	Ticker string
	// PriceTicker is the manual price ticker, or the default mapping if the Instrument has no manual price ticker.
	PriceTicker   string
	QuoteCurrency string
	// AccountCurrencies are the currencies of the Accounts that hold the Instrument, in order.
	AccountCurrencies []string
}

// HeldInstruments returns the Instruments in a Holding of any Account that have a Quote Currency,
// in broker ticker order. The tool cannot check the price of an Instrument with no Quote Currency.
func HeldInstruments(ctx context.Context, st *store.Store) ([]HeldInstrument, error) {
	tickers, err := heldTickers(ctx, st)
	if err != nil {
		return nil, err
	}
	instruments, err := st.Instruments(ctx)
	if err != nil {
		return nil, err
	}
	held := make([]HeldInstrument, 0, len(tickers))
	for _, ticker := range slices.Sorted(maps.Keys(tickers)) {
		inst := instruments[ticker]
		if inst.QuoteCurrency == "" {
			continue
		}
		held = append(held, HeldInstrument{
			Ticker: ticker, PriceTicker: EffectiveTicker(ticker, inst.PriceTicker), QuoteCurrency: inst.QuoteCurrency,
			AccountCurrencies: slices.Sorted(maps.Keys(tickers[ticker])),
		})
	}
	return held, nil
}

// heldTickers returns the broker tickers of the Instruments in a Holding of any Account,
// each with the set of the currencies of the Accounts that hold it.
func heldTickers(ctx context.Context, st *store.Store) (map[string]map[string]bool, error) {
	accounts, err := st.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	held := map[string]map[string]bool{}
	for _, acc := range accounts {
		l, err := st.Ledger(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		state, err := ledger.Calculate(l)
		if err != nil {
			return nil, fmt.Errorf("calculate the holdings of account %d: %w", acc.ID, err)
		}
		for _, h := range state.Holdings {
			if held[h.Instrument.Ticker] == nil {
				held[h.Instrument.Ticker] = map[string]bool{}
			}
			held[h.Instrument.Ticker][acc.Currency] = true
		}
	}
	return held, nil
}
