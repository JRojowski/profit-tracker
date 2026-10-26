package exchange

import (
	"context"
	"slices"
	"time"
)

// Series is the stored series of the Exchange Rates of one currency, with the range of days that the tool asked for.
type Series struct {
	Currency string
	// Rates are the stored rates in date order.
	Rates []Rate
	// FirstDay and LastDay are the range of days that the tool asked the NBP for, in Polish time at midnight UTC.
	// Inside the range, a day with no rate is a day with no table.
	FirstDay, LastDay time.Time
}

// On returns the rate of the day of t in Polish time: the last table on that day or before it, inside the range.
// Thus a Saturday gets the rate of the Friday before. On returns false if the day is outside the range,
// or if the range has no table on the day or before it.
func (se Series) On(t time.Time) (Rate, bool) {
	day := DayInWarsaw(t)
	if day.Before(se.FirstDay) || day.After(se.LastDay) {
		return Rate{}, false
	}
	i, found := slices.BinarySearchFunc(se.Rates, day, func(r Rate, d time.Time) int { return r.Date.Compare(d) })
	if found {
		return se.Rates[i], true
	}
	// Rates[i-1] is the last table before the day. A table before the first day is outside the range.
	if i == 0 || se.Rates[i-1].Date.Before(se.FirstDay) {
		return Rate{}, false
	}
	return se.Rates[i-1], true
}

// Series returns the stored series of each currency that has a recorded range, by currency.
func (s *Service) Series(ctx context.Context) (map[string]Series, error) {
	stored, err := s.store.ExchangeRateSeries(ctx)
	if err != nil {
		return nil, err
	}
	ranges, err := s.store.ExchangeRateRanges(ctx)
	if err != nil {
		return nil, err
	}
	series := make(map[string]Series, len(ranges))
	for currency, r := range ranges {
		rates := make([]Rate, 0, len(stored[currency]))
		for _, sr := range stored[currency] {
			rates = append(rates, Rate{Date: sr.Date, Mid: sr.Rate, Table: sr.TableNumber})
		}
		series[currency] = Series{Currency: currency, Rates: rates, FirstDay: r.FirstDay, LastDay: r.LastDay}
	}
	return series, nil
}
