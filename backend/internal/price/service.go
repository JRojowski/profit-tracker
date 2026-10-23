package price

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// Service gets the prices of the Instruments in Holdings from a Provider and stores them.
// It runs one refresh at a time.
type Service struct {
	store    *store.Store
	provider Provider
	now      func() time.Time
	// lock serializes the refreshes, because the refresh at start and a refresh by the user can overlap.
	// It is a channel with one slot and not a sync.Mutex, so that a refresh stops waiting when its context ends.
	lock chan struct{}
}

// NewService returns a Service that stores the prices in st. now is the clock that tells "today".
func NewService(st *store.Store, provider Provider, now func() time.Time) *Service {
	return &Service{store: st, provider: provider, now: now, lock: make(chan struct{}, 1)}
}

// Result is the result of one refresh.
type Result struct {
	// Refreshed is the number of stored prices.
	Refreshed int
	// Failures are the Instruments that got no price, in broker ticker order.
	Failures []Failure
}

// Failure is one Instrument that got no price in a refresh.
type Failure struct {
	// Ticker is the broker ticker of the Instrument.
	Ticker string
	Err    error
}

// CurrencyError tells that the provider gave a price in a currency that is not the quote currency of the Instrument.
type CurrencyError struct {
	// Got is the currency of the price. Want is the quote currency of the Instrument.
	Got, Want string
}

func (e *CurrencyError) Error() string {
	return fmt.Sprintf("the price is in %s, but the Instrument is quoted in %s", e.Got, e.Want)
}

// Refresh gets a price for each Instrument that is in a Holding of an Account and has a quote currency.
// A failure of one Instrument does not stop the others.
// Refresh returns an error if it cannot read or store data, or if ctx ends. Then the result holds the prices
// that it stored before the error.
func (s *Service) Refresh(ctx context.Context) (Result, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer s.release()

	held, err := s.heldInstruments(ctx)
	if err != nil {
		return Result{}, err
	}
	return s.refresh(ctx, held)
}

// RefreshIfStale runs a refresh if a held Instrument with a quote currency has no price fetched today, in UTC.
// Thus a later start of the same day tries again for the Instruments that failed. It tells if the refresh ran.
func (s *Service) RefreshIfStale(ctx context.Context) (Result, bool, error) {
	if err := s.acquire(ctx); err != nil {
		return Result{}, false, err
	}
	defer s.release()

	held, err := s.heldInstruments(ctx)
	if err != nil {
		return Result{}, false, err
	}
	fetched, err := s.store.TickersFetchedSince(ctx, StartOfDay(s.now()))
	if err != nil {
		return Result{}, false, err
	}
	if !slices.ContainsFunc(held, func(h heldInstrument) bool { return !fetched[h.ticker] }) {
		return Result{}, false, nil
	}
	result, err := s.refresh(ctx, held)
	return result, true, err
}

// acquire waits until no other refresh runs. It returns the error of ctx if ctx ends first.
func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for the running price refresh: %w", ctx.Err())
	}
}

// release lets the next refresh run.
func (s *Service) release() {
	<-s.lock
}

// StartOfDay returns the start of the day of t in UTC.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// refresh gets and stores a price for each held Instrument. It stops when ctx ends.
func (s *Service) refresh(ctx context.Context, held []heldInstrument) (Result, error) {
	var result Result
	for _, h := range held {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("refresh prices: %w", err)
		}
		q, err := s.quote(ctx, h.ticker, h.quoteCurrency)
		if err != nil {
			// A provider error that comes from the end of ctx is not a failure of the Instrument.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, fmt.Errorf("refresh prices: %w", ctxErr)
			}
			result.Failures = append(result.Failures, Failure{Ticker: h.ticker, Err: err})
			continue
		}
		err = s.store.SavePrice(ctx, store.Price{
			Ticker: h.ticker, Date: q.Date, Close: q.Close, Currency: q.Currency, Source: q.Source, FetchedAt: s.now(),
		})
		if err != nil {
			return result, err
		}
		result.Refreshed++
	}
	return result, nil
}

// quote gets the price of the Instrument with the broker ticker and checks its currency.
func (s *Service) quote(ctx context.Context, ticker, quoteCurrency string) (Quote, error) {
	q, err := s.provider.LastClose(ctx, Ticker(ticker))
	if err != nil {
		return Quote{}, err
	}
	if q.Currency != quoteCurrency {
		return Quote{}, &CurrencyError{Got: q.Currency, Want: quoteCurrency}
	}
	return q, nil
}

// heldInstrument is an Instrument in a Holding of an Account, with a known quote currency.
type heldInstrument struct {
	ticker        string
	quoteCurrency string
}

// heldInstruments returns the Instruments in a Holding of any Account that have a quote currency,
// in broker ticker order. The tool cannot check the price of an Instrument with no quote currency.
func (s *Service) heldInstruments(ctx context.Context) ([]heldInstrument, error) {
	tickers, err := s.heldTickers(ctx)
	if err != nil {
		return nil, err
	}
	currencies, err := s.store.QuoteCurrencies(ctx)
	if err != nil {
		return nil, err
	}
	held := make([]heldInstrument, 0, len(tickers))
	for _, ticker := range tickers {
		if qc := currencies[ticker]; qc != "" {
			held = append(held, heldInstrument{ticker: ticker, quoteCurrency: qc})
		}
	}
	return held, nil
}

// heldTickers returns the broker tickers of the Instruments in a Holding of any Account, in order.
func (s *Service) heldTickers(ctx context.Context) ([]string, error) {
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	for _, acc := range accounts {
		l, err := s.store.Ledger(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		state, err := ledger.Calculate(l)
		if err != nil {
			return nil, fmt.Errorf("calculate the holdings of account %d: %w", acc.ID, err)
		}
		for _, h := range state.Holdings {
			held[h.Instrument.Ticker] = true
		}
	}
	return slices.Sorted(maps.Keys(held)), nil
}
