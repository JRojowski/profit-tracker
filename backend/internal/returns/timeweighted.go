package returns

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// DayValue is the value of an Account at the end of one calendar day, with the Net Deposits flows of that day.
type DayValue struct {
	// Date is the day at midnight UTC.
	Date  time.Time
	Value decimal.Decimal
	// Flow is the sum of the Net Deposits flows of the day: deposits, withdrawals, Transfers In and Transfers Out.
	// Dividends, interest, taxes and fees are not flows.
	Flow decimal.Decimal
}

// TimeWeightedReturns are the cumulative Time-Weighted Returns of the periods. See GLOSSARY.md.
// Each period starts at the end of its base day in Starts. Its return covers the days after the base day.
type TimeWeightedReturns struct {
	OneMonth, ThreeMonths, YearToDate, OneYear, All Rate
	// Starts are the base days of the periods. They are the zero time when there are no days.
	Starts PeriodStarts
}

// PeriodStarts are the base days of the periods. Each period ends on the last day. The chart of a period shows
// the days from its base day to the last day.
//   - OneMonth: the same day of the month, one month before the last day.
//   - ThreeMonths: the same day of the month, three months before the last day.
//   - YearToDate: 31 December of the year before the year of the last day.
//   - OneYear: the same day, one year before the last day.
//   - All: the first day.
//
// When the month of the base day has no such day, the base day is the last day of that month.
// For example, one month before 31 March is 28 February or 29 February.
// A base day before the first day is the first day.
type PeriodStarts struct {
	OneMonth, ThreeMonths, YearToDate, OneYear, All time.Time
}

// twrPrecision is the number of decimal places of the product of the daily factors.
// Without a limit, each multiplication adds decimal places.
const twrPrecision = 20

// TimeWeighted returns the cumulative Time-Weighted Return of each period. days are in date order,
// one for each calendar day.
//
// A flow comes in at the start of its day. Thus the rate of day t is (V_t − F_t) / V_(t−1) − 1.
// The return of a period is the product of (1 + rate) of its days, minus 1. A day after a value of 0 or less has no rate.
// This is the first day, or a day after the Account was empty.
//
// The return of a period is not available (ReasonNoValueHistory) if no day of the period has a rate.
// The result keeps 8 decimal places of the percentage, as the Annual Return does.
func TimeWeighted(days []DayValue) TimeWeightedReturns {
	if len(days) == 0 {
		none := Rate{Reason: ReasonNoValueHistory}
		return TimeWeightedReturns{OneMonth: none, ThreeMonths: none, YearToDate: none, OneYear: none, All: none}
	}
	starts := periodStarts(days[0].Date, days[len(days)-1].Date)
	return TimeWeightedReturns{
		OneMonth:    periodReturn(days, starts.OneMonth),
		ThreeMonths: periodReturn(days, starts.ThreeMonths),
		YearToDate:  periodReturn(days, starts.YearToDate),
		OneYear:     periodReturn(days, starts.OneYear),
		All:         periodReturn(days, starts.All),
		Starts:      starts,
	}
}

// periodStarts returns the base days of the periods from the first day to the last day. See PeriodStarts.
func periodStarts(first, last time.Time) PeriodStarts {
	clamp := func(base time.Time) time.Time {
		if base.Before(first) {
			return first
		}
		return base
	}
	return PeriodStarts{
		OneMonth:    clamp(monthsBefore(last, 1)),
		ThreeMonths: clamp(monthsBefore(last, 3)),
		YearToDate:  clamp(time.Date(last.Year()-1, time.December, 31, 0, 0, 0, 0, time.UTC)),
		OneYear:     clamp(monthsBefore(last, 12)),
		All:         first,
	}
}

// monthsBefore returns the same day of the month, n months before day. If that month is shorter,
// it returns the last day of that month.
func monthsBefore(day time.Time, n int) time.Time {
	firstOfMonth := time.Date(day.Year(), day.Month()-time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1).Day()
	return firstOfMonth.AddDate(0, 0, min(day.Day(), lastOfMonth)-1)
}

// periodReturn returns the Time-Weighted Return of the days after the base day.
func periodReturn(days []DayValue, base time.Time) Rate {
	// start is the index of the first day after the base day.
	start, found := slices.BinarySearchFunc(days, base, func(d DayValue, t time.Time) int { return d.Date.Compare(t) })
	if found {
		start++
	}
	// The first day has no day before it, so it has no rate.
	start = max(start, 1)

	one := decimal.NewFromInt(1)
	product, rated := one, false
	for t := start; t < len(days); t++ {
		previous := days[t-1].Value
		if !previous.IsPositive() {
			continue
		}
		factor := days[t].Value.Sub(days[t].Flow).DivRound(previous, twrPrecision)
		product = product.Mul(factor).Round(twrPrecision)
		rated = true
	}
	if !rated {
		return Rate{Reason: ReasonNoValueHistory}
	}
	return Rate{Percent: product.Sub(one).Mul(hundred).Round(8)}
}
