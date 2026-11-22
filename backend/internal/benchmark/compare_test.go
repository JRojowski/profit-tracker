package benchmark_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/benchmark"
)

func day(d int) time.Time { return time.Date(2026, time.October, d, 0, 0, 0, 0, time.UTC) }

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// sp500 returns the result of the S&P 500 Benchmark from the comparison.
func sp500(t *testing.T, results []benchmark.Result) benchmark.Result {
	t.Helper()
	require.NotEmpty(t, results)
	require.Equal(t, "sp500", results[0].ID)
	return results[0]
}

// TestCompare_EdgeCases checks the cases of the USD Benchmark S&P 500 that the HTTP tests do not reach.
// The base day is 2026-10-01 and the last day is 2026-10-10. The return of the user is 5 %.
func TestCompare_EdgeCases(t *testing.T) {
	t.Parallel()
	closes := []benchmark.Close{{Date: day(1), Price: dec("100")}, {Date: day(10), Price: dec("110")}}
	cases := []struct {
		name       string
		currency   string
		closes     []benchmark.Close
		rates      map[string][]benchmark.Rate
		wantStatus benchmark.Status
		wantReturn string
		wantDiff   string
	}{
		{
			// Base: 100 × 4.00 / 4.00 = 100 EUR. Last: 110 × 4.40 / 4.00 = 121 EUR. 121 / 100 − 1 = 21 %.
			name: "USD Benchmark on EUR Account uses both rates", currency: "EUR", closes: closes,
			rates: map[string][]benchmark.Rate{
				"USD": {{Date: day(1), Mid: dec("4.00")}, {Date: day(10), Mid: dec("4.40")}},
				"EUR": {{Date: day(1), Mid: dec("4.00")}},
			},
			wantStatus: benchmark.StatusAvailable, wantReturn: "21", wantDiff: "-16",
		},
		{
			name: "same currency needs no rate", currency: "USD", closes: closes,
			wantStatus: benchmark.StatusAvailable, wantReturn: "10", wantDiff: "-5",
		},
		{
			name: "no rate on or before the base day", currency: "PLN", closes: closes,
			rates:      map[string][]benchmark.Rate{"USD": {{Date: day(2), Mid: dec("4.00")}}},
			wantStatus: benchmark.StatusNoFullPeriod,
		},
		{
			name: "a rate of zero is not valid", currency: "PLN", closes: closes,
			rates:      map[string][]benchmark.Rate{"USD": {{Date: day(1), Mid: dec("0")}}},
			wantStatus: benchmark.StatusNoFullPeriod,
		},
		{
			name: "a close of zero on the base day is not valid", currency: "USD",
			closes:     []benchmark.Close{{Date: day(1), Price: dec("0")}, {Date: day(10), Price: dec("110")}},
			wantStatus: benchmark.StatusNoFullPeriod,
		},
		{
			// 2 / 3 − 1 = −33.333…%, rounded to 8 decimal places.
			name: "the return keeps 8 decimal places", currency: "USD",
			closes:     []benchmark.Close{{Date: day(1), Price: dec("3")}, {Date: day(10), Price: dec("2")}},
			wantStatus: benchmark.StatusAvailable, wantReturn: "-33.33333333", wantDiff: "38.33333333",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sp500(t, benchmark.Compare(benchmark.Input{
				Currency: tc.currency, BaseDay: day(1), LastDay: day(10), UserReturn: dec("5"),
				Closes: map[string][]benchmark.Close{"sp500": tc.closes}, Rates: tc.rates,
			}))

			assert.Equal(t, tc.wantStatus, got.Status)
			if tc.wantStatus == benchmark.StatusAvailable {
				assert.Equal(t, tc.wantReturn, got.Return.String())
				assert.Equal(t, tc.wantDiff, got.Difference.String())
			}
		})
	}
}

func TestCompare_EachBenchmarkOfTheListInOrder(t *testing.T) {
	t.Parallel()

	got := benchmark.Compare(benchmark.Input{Currency: "PLN", BaseDay: day(1), LastDay: day(10)})

	ids := make([]string, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.ID)
		assert.Equal(t, benchmark.StatusNoFullPeriod, r.Status)
	}
	assert.Equal(t, []string{"sp500", "wig20tr", "mwig40tr", "world", "gold"}, ids)
}
