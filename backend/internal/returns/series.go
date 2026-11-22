package returns

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// SeriesPoint is the cumulative Time-Weighted Return from the end of the base day to the end of Date.
type SeriesPoint struct {
	// Date is the day at midnight UTC.
	Date time.Time
	// Percent is the cumulative return as a percentage with at most 8 decimal places.
	Percent decimal.Decimal
}

// TimeWeightedSeries returns one point for each day from the base day to the last day of days. days are in date
// order, one for each calendar day, as for TimeWeighted.
//
// The first point is 0. Each next point chains the daily factors with the same formula and precision as
// TimeWeighted, so the last point is equal to the return of the period with this base day. A day with no rate
// keeps the point of the day before. The result is empty if the base day is not a day of days.
func TimeWeightedSeries(days []DayValue, base time.Time) []SeriesPoint {
	first, found := slices.BinarySearchFunc(days, base, func(d DayValue, t time.Time) int { return d.Date.Compare(t) })
	if !found {
		return []SeriesPoint{}
	}
	series := make([]SeriesPoint, 0, len(days)-first)
	product := decimal.NewFromInt(1)
	for t := first; t < len(days); t++ {
		if t > first {
			if factor, ok := dailyFactor(days, t); ok {
				product = product.Mul(factor).Round(twrPrecision)
			}
		}
		series = append(series, SeriesPoint{Date: days[t].Date, Percent: CumulativePercent(product)})
	}
	return series
}
