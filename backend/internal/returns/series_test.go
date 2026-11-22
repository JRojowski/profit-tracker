package returns_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

// octoberDay returns the day of October 2026 at midnight UTC.
func octoberDay(d int) time.Time { return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC) }

// dayValues returns one DayValue for each "value/flow" pair, from 2026-10-01, one calendar day each.
func dayValues(pairs ...[2]string) []returns.DayValue {
	days := make([]returns.DayValue, 0, len(pairs))
	for i, p := range pairs {
		days = append(days, returns.DayValue{
			Date: octoberDay(i + 1), Value: decimal.RequireFromString(p[0]), Flow: decimal.RequireFromString(p[1]),
		})
	}
	return days
}

// TestTimeWeightedSeries checks the cumulative Time-Weighted Return of each day after the base day.
func TestTimeWeightedSeries(t *testing.T) {
	t.Parallel()
	// Day 2: 1100 / 1000 = 1.1. Day 3: (1710 − 500) / 1100 = 1.1. Day 4: 1539 / 1710 = 0.9.
	withDeposit := dayValues([2]string{"1000", "1000"}, [2]string{"1100", "0"}, [2]string{"1710", "500"}, [2]string{"1539", "0"})
	tests := []struct {
		name  string
		days  []returns.DayValue
		base  time.Time
		want  []string
		dates []time.Time
	}{
		{
			// 1.1 → 10 %, 1.1 × 1.1 = 1.21 → 21 %, 1.21 × 0.9 = 1.089 → 8.9 %.
			name: "base day is the first day", days: withDeposit, base: octoberDay(1),
			want:  []string{"0", "10", "21", "8.9"},
			dates: []time.Time{octoberDay(1), octoberDay(2), octoberDay(3), octoberDay(4)},
		},
		{
			// 1.1 → 10 %, 1.1 × 0.9 = 0.99 → −1 %.
			name: "base day in the middle", days: withDeposit, base: octoberDay(2),
			want:  []string{"0", "10", "-1"},
			dates: []time.Time{octoberDay(2), octoberDay(3), octoberDay(4)},
		},
		{
			name: "base day is the last day", days: withDeposit, base: octoberDay(4),
			want: []string{"0"}, dates: []time.Time{octoberDay(4)},
		},
		{
			// Day 2: (0 + 100) / 100 = 1. Day 3 comes after a value of 0, so it has no rate. Day 4: 220 / 200 = 1.1.
			name: "a day after an empty Account keeps the return",
			days: dayValues([2]string{"100", "100"}, [2]string{"0", "-100"}, [2]string{"200", "200"}, [2]string{"220", "0"}),
			base: octoberDay(1), want: []string{"0", "0", "0", "10"},
			dates: []time.Time{octoberDay(1), octoberDay(2), octoberDay(3), octoberDay(4)},
		},
		{
			// 2 / 3 − 1 = −33.333…%, rounded to 8 decimal places.
			name: "each point keeps 8 decimal places",
			days: dayValues([2]string{"3", "3"}, [2]string{"2", "0"}), base: octoberDay(1),
			want: []string{"0", "-33.33333333"}, dates: []time.Time{octoberDay(1), octoberDay(2)},
		},
		{name: "no days", days: nil, base: octoberDay(1), want: []string{}, dates: []time.Time{}},
		{name: "base day is not a day of the list", days: withDeposit, base: octoberDay(9), want: []string{}, dates: []time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := returns.TimeWeightedSeries(tc.days, tc.base)

			percents := make([]string, 0, len(got))
			dates := make([]time.Time, 0, len(got))
			for _, p := range got {
				percents = append(percents, p.Percent.String())
				dates = append(dates, p.Date)
			}
			assert.Equal(t, tc.want, percents)
			assert.Equal(t, tc.dates, dates)
		})
	}
}

// TestTimeWeightedSeries_LastPointIsTheReturnOfThePeriod checks that the last point has the same decimal text as
// the return of each period, also when the product needs the 20 decimal places.
func TestTimeWeightedSeries_LastPointIsTheReturnOfThePeriod(t *testing.T) {
	t.Parallel()
	days := make([]returns.DayValue, 0, 400)
	value := decimal.NewFromInt(1000)
	start := time.Date(2025, time.September, 15, 0, 0, 0, 0, time.UTC)
	for i := range 400 {
		flow := decimal.Zero
		if i == 0 {
			flow = value
		}
		if i%7 == 3 {
			flow = decimal.NewFromInt(37)
			value = value.Add(flow)
		}
		// Each day changes the value by a factor with many decimal places.
		value = value.Mul(decimal.RequireFromString("1.0013")).Sub(decimal.NewFromInt(int64(i % 5))).Round(2)
		days = append(days, returns.DayValue{Date: start.AddDate(0, 0, i), Value: value, Flow: flow})
	}
	twr := returns.TimeWeighted(days)

	for name, period := range map[string]struct {
		rate returns.Rate
		base time.Time
	}{
		"1M": {twr.OneMonth, twr.Starts.OneMonth}, "3M": {twr.ThreeMonths, twr.Starts.ThreeMonths},
		"YTD": {twr.YearToDate, twr.Starts.YearToDate}, "1Y": {twr.OneYear, twr.Starts.OneYear},
		"All": {twr.All, twr.Starts.All},
	} {
		got := returns.TimeWeightedSeries(days, period.base)

		require.NotEmpty(t, got, name)
		assert.Equal(t, period.rate.Percent.String(), got[len(got)-1].Percent.String(), name)
		assert.Equal(t, days[len(days)-1].Date, got[len(got)-1].Date, name)
	}
}
