package price

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// minGap and maxGap are the limits of the pause before a history request.
// The price source publishes no limits. A gap of 1 to 2 seconds keeps the requests slow and not regular.
const (
	minGap = time.Second
	maxGap = 2 * time.Second
)

// PauseBetweenRequests waits a random time from 1 to 2 seconds. It returns the error of ctx if ctx ends first.
func PauseBetweenRequests(ctx context.Context) error {
	timer := time.NewTimer(gap())
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("pause between price requests: %w", ctx.Err())
	}
}

// gap returns a random time from minGap to maxGap.
func gap() time.Duration {
	//nolint:gosec // The gap needs no secure random numbers.
	return minGap + rand.N(maxGap-minGap+1)
}

// historyRequest is one range of days that an Instrument needs from the price source.
type historyRequest struct {
	// ticker is the broker ticker. priceTicker is the price ticker that the tool uses.
	ticker, priceTicker string
	quoteCurrency       string
	// from and to are at midnight UTC. The range includes both days.
	from, to time.Time
}

// historyPlan returns the history requests of the refresh, in broker ticker order.
// The history of an Instrument goes from the day of its first Fill to yesterday, in UTC.
// The tool asks only for the days that are not in the stored range of the Instrument.
// An Instrument with no Quote Currency gets no history, because the tool cannot check its price.
func (s *Service) historyPlan(ctx context.Context) ([]historyRequest, error) {
	firstFills, err := s.store.FirstFillTimes(ctx)
	if err != nil {
		return nil, err
	}
	instruments, err := s.store.Instruments(ctx)
	if err != nil {
		return nil, err
	}
	ranges, err := s.store.PriceRanges(ctx)
	if err != nil {
		return nil, err
	}
	yesterday := StartOfDay(s.now()).AddDate(0, 0, -1)
	var plan []historyRequest
	for _, ticker := range slices.Sorted(maps.Keys(firstFills)) {
		inst := instruments[ticker]
		if inst.QuoteCurrency == "" {
			continue
		}
		r, asked := ranges[ticker]
		for _, days := range missingDays(StartOfDay(firstFills[ticker]), r, asked, yesterday) {
			plan = append(plan, historyRequest{
				ticker: ticker, priceTicker: EffectiveTicker(ticker, inst.PriceTicker), quoteCurrency: inst.QuoteCurrency,
				from: days[0], to: days[1],
			})
		}
	}
	return plan, nil
}

// missingDays returns the ranges of days from first to yesterday that are not in the stored range r.
// Each range is a pair of the first and the last day. asked is false if the tool never asked for the Instrument.
// The range before the stored first day comes first. The days to yesterday are final closes,
// so the tool never asks for a day of the stored range again.
func missingDays(first time.Time, r store.PriceRange, asked bool, yesterday time.Time) [][2]time.Time {
	if first.After(yesterday) {
		return nil
	}
	if !asked {
		return [][2]time.Time{{first, yesterday}}
	}
	var missing [][2]time.Time
	if first.Before(r.FirstDay) {
		missing = append(missing, [2]time.Time{first, r.FirstDay.AddDate(0, 0, -1)})
	}
	if r.LastDay.Before(yesterday) {
		missing = append(missing, [2]time.Time{r.LastDay.AddDate(0, 0, 1), yesterday})
	}
	return missing
}

// refreshHistory sends the history requests one after the other, with a pause before each request,
// and stores the daily closes. It adds the counts and the failures to result.
// It stops with no error when the price source answers "Too Many Requests". It stops with an error when ctx ends.
//
// It stores a history in a currency that is not the Quote Currency, with the currency that the source gave.
// It reports the Instrument as a failure, so that the user can correct the Quote Currency.
// The Value History uses only the prices in the Quote Currency. Thus a correction uses the stored closes at once.
func (s *Service) refreshHistory(ctx context.Context, plan []historyRequest, result *Result) error {
	s.setProgress(func(p *Progress) { p.Total = len(plan) })
	// failed are the Instruments with a failed request in this refresh. reported are the Instruments in
	// result.HistoryFailures.
	failed, reported := map[string]bool{}, map[string]bool{}
	for i, req := range plan {
		s.setProgress(func(p *Progress) { p.Done, p.Ticker = i, req.ticker })
		// The second range of a failed Instrument would fail too.
		if failed[req.ticker] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("refresh price history: %w", err)
		}
		if s.pause != nil {
			if err := s.pause(ctx); err != nil {
				return fmt.Errorf("refresh price history: %w", err)
			}
		}
		h, err := s.provider.DailyCloses(ctx, req.priceTicker, req.from, req.to)
		if err != nil {
			stop, err := stopOn(ctx, err, result)
			if stop {
				return err
			}
			failed[req.ticker] = true
			if !reported[req.ticker] {
				reported[req.ticker] = true
				result.HistoryFailures = append(result.HistoryFailures, Failure{Ticker: req.ticker, Err: err})
			}
			continue
		}
		if err := s.saveHistory(ctx, req, h); err != nil {
			return err
		}
		result.HistoryStored += len(h.Closes)
		// A response with no closes can have no currency.
		if len(h.Closes) > 0 && h.Currency != req.quoteCurrency && !reported[req.ticker] {
			reported[req.ticker] = true
			result.HistoryFailures = append(result.HistoryFailures, Failure{
				Ticker: req.ticker, Err: &CurrencyError{Got: h.Currency, Want: req.quoteCurrency},
			})
		}
	}
	return nil
}

// saveHistory stores the daily closes of the history request and adds its days to the stored range.
func (s *Service) saveHistory(ctx context.Context, req historyRequest, h History) error {
	now := s.now()
	prices := make([]store.Price, 0, len(h.Closes))
	for _, c := range h.Closes {
		prices = append(prices, store.Price{
			Ticker: req.ticker, Date: c.Date, Close: c.Close, Currency: h.Currency, Source: h.Source, FetchedAt: now,
		})
	}
	asked := store.PriceRange{Ticker: req.ticker, FirstDay: req.from, LastDay: req.to, FetchedAt: now}
	return s.store.SaveDailyPrices(ctx, asked, prices)
}
