package valuehistory

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// Errors of TotalResultOfPeriod.
var (
	// ErrPeriodReversed means that the first day of the period is after its last day.
	ErrPeriodReversed = errors.New("the first day of the period is after its last day")
	// ErrPeriodAfterToday means that the period starts after today.
	ErrPeriodAfterToday = errors.New("the period lies after today")
)

// PeriodResult is the Total Result of a period and the values that give it.
type PeriodResult struct {
	// StartDay is the day whose end gives the start values. It is the day before the first day of the period.
	StartDay time.Time
	// EndDay is the day whose end gives the end values. It is the last day of the period, or today if the period
	// ends after today.
	EndDay time.Time
	// StartValue and EndValue are the values at the end of StartDay and of EndDay.
	StartValue, EndValue decimal.Decimal
	// NetDeposits are the Net Deposits of the period: the flows after StartDay up to EndDay.
	NetDeposits decimal.Decimal
	// TotalResult is EndValue minus StartValue minus NetDeposits.
	TotalResult decimal.Decimal
	// ValuedAtCost are the tickers of the Instruments that the Value History valued at cost on StartDay or on
	// EndDay, in ticker order. It is nil if there is none.
	ValuedAtCost []string
}

// TotalResultOfPeriod returns the Total Result of the period from the first day to the last day, both included.
// It is the change of the Total Result from the end of the day before the period to the end of its last day.
// See GLOSSARY.md, "Total Result".
//
// points are the points of the Value History in date order, one for each calendar day. today is the day of today
// at midnight UTC. The value and the Net Deposits of a day before the first point are 0. A period that ends after
// today ends today. The period is not valid if first is after last (ErrPeriodReversed) or after today
// (ErrPeriodAfterToday).
func TotalResultOfPeriod(points []Point, first, last, today time.Time) (PeriodResult, error) {
	if first.After(last) {
		return PeriodResult{}, fmt.Errorf("%s to %s: %w", first.Format(time.DateOnly), last.Format(time.DateOnly), ErrPeriodReversed)
	}
	if first.After(today) {
		return PeriodResult{}, fmt.Errorf("%s to %s, today is %s: %w",
			first.Format(time.DateOnly), last.Format(time.DateOnly), today.Format(time.DateOnly), ErrPeriodAfterToday)
	}
	end := last
	if end.After(today) {
		end = today
	}
	start := first.AddDate(0, 0, -1)

	startPoint, endPoint := pointOf(points, start), pointOf(points, end)
	netDeposits := endPoint.NetDeposits.Sub(startPoint.NetDeposits)
	atCost := slices.Concat(startPoint.ValuedAtCost, endPoint.ValuedAtCost)
	slices.Sort(atCost)
	return PeriodResult{
		StartDay: start, EndDay: end,
		StartValue: startPoint.Value, EndValue: endPoint.Value, NetDeposits: netDeposits,
		TotalResult:  endPoint.Value.Sub(startPoint.Value).Sub(netDeposits),
		ValuedAtCost: slices.Compact(atCost),
	}, nil
}

// pointOf returns the point of the day. A day before the first point has the value 0 and no Net Deposits.
// A day after the last point has the state of the last point. The points have one point for each day.
func pointOf(points []Point, day time.Time) Point {
	if len(points) == 0 || day.Before(points[0].Date) {
		return Point{}
	}
	i := min(int(day.Sub(points[0].Date)/(24*time.Hour)), len(points)-1)
	return points[i]
}
