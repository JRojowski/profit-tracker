package valuehistory_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/returns"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

func day(d int) time.Time {
	return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC)
}

// point returns the point of a day of October 2026 with the value and the Net Deposits.
func point(d int, value, netDeposits string, atCost ...string) valuehistory.Point {
	return valuehistory.Point{
		DayValue:     returns.DayValue{Date: day(d), Value: decimal.RequireFromString(value)},
		NetDeposits:  decimal.RequireFromString(netDeposits),
		ValuedAtCost: atCost,
	}
}

// history has the days 10 to 15. Today is the day 15.
//
//	day 10: value 1000, Net Deposits 1000
//	day 11: value 1100, Net Deposits 1000
//	day 12: value 1700, Net Deposits 1500, ABC valued at cost
//	day 13: value 1750, Net Deposits 1500
//	day 14: value 1650, Net Deposits 1400, XYZ and ABC valued at cost
//	day 15: value 1800, Net Deposits 1400
func history() []valuehistory.Point {
	return []valuehistory.Point{
		point(10, "1000", "1000"), point(11, "1100", "1000"), point(12, "1700", "1500", "ABC"),
		point(13, "1750", "1500"), point(14, "1650", "1400", "ABC", "XYZ"), point(15, "1800", "1400"),
	}
}

func TestTotalResultOfPeriod_UsesTheValuesAtTheEndOfTheDayBeforeAndOfTheLastDay(t *testing.T) {
	t.Parallel()

	// The value at the start is the value of day 11. The Net Deposits of the period are 1500 - 1000.
	// Total Result = 1750 - 1100 - 500.
	got, err := valuehistory.TotalResultOfPeriod(history(), day(12), day(13), day(15))

	require.NoError(t, err)
	assert.Equal(t, day(11), got.StartDay)
	assert.Equal(t, day(13), got.EndDay)
	assert.Equal(t, "1100", got.StartValue.String())
	assert.Equal(t, "1750", got.EndValue.String())
	assert.Equal(t, "500", got.NetDeposits.String())
	assert.Equal(t, "150", got.TotalResult.String())
	assert.Empty(t, got.ValuedAtCost)
}

func TestTotalResultOfPeriod_ADayThatStartsBeforeTheFirstPointStartsAtZero(t *testing.T) {
	t.Parallel()

	// The day before the period is day 9: no point. Value and Net Deposits are 0.
	got, err := valuehistory.TotalResultOfPeriod(history(), day(1), day(11), day(15))

	require.NoError(t, err)
	assert.Equal(t, day(0), got.StartDay)
	assert.Equal(t, "0", got.StartValue.String())
	assert.Equal(t, "1100", got.EndValue.String())
	assert.Equal(t, "1000", got.NetDeposits.String())
	assert.Equal(t, "100", got.TotalResult.String())
}

func TestTotalResultOfPeriod_APeriodThatEndsBeforeTheFirstPointHasNoResult(t *testing.T) {
	t.Parallel()

	got, err := valuehistory.TotalResultOfPeriod(history(), day(1), day(5), day(15))

	require.NoError(t, err)
	assert.Equal(t, "0", got.EndValue.String())
	assert.Equal(t, "0", got.NetDeposits.String())
	assert.Equal(t, "0", got.TotalResult.String())
}

func TestTotalResultOfPeriod_APeriodThatEndsAfterTodayEndsToday(t *testing.T) {
	t.Parallel()

	got, err := valuehistory.TotalResultOfPeriod(history(), day(14), day(30), day(15))

	require.NoError(t, err)
	assert.Equal(t, day(15), got.EndDay)
	// 1800 - 1650 - (1400 - 1400)
	assert.Equal(t, "150", got.TotalResult.String())
	same, err := valuehistory.TotalResultOfPeriod(history(), day(14), day(15), day(15))
	require.NoError(t, err)
	assert.Equal(t, same, got)
}

func TestTotalResultOfPeriod_TwoPeriodsThatFollowEachOtherAddToTheJoinedPeriod(t *testing.T) {
	t.Parallel()

	first, err := valuehistory.TotalResultOfPeriod(history(), day(10), day(12), day(15))
	require.NoError(t, err)
	second, err := valuehistory.TotalResultOfPeriod(history(), day(13), day(15), day(15))
	require.NoError(t, err)
	joined, err := valuehistory.TotalResultOfPeriod(history(), day(10), day(15), day(15))
	require.NoError(t, err)

	assert.Equal(t, joined.TotalResult.String(), first.TotalResult.Add(second.TotalResult).String())
	assert.Equal(t, "400", joined.TotalResult.String(), "1800 - 0 - 1400")
}

func TestTotalResultOfPeriod_NamesTheInstrumentsValuedAtCostOnTheTwoDaysOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from, to int
		want     []string
	}{
		{"on the start day", 13, 15, []string{"ABC"}},
		{"on the end day", 10, 14, []string{"ABC", "XYZ"}},
		{"on both days, each once", 13, 14, []string{"ABC", "XYZ"}},
		{"on a day between the two days only", 11, 13, nil},
		{"on no day", 11, 11, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := valuehistory.TotalResultOfPeriod(history(), day(tc.from), day(tc.to), day(15))

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.ValuedAtCost)
		})
	}
}

func TestTotalResultOfPeriod_WithNoPoints_IsZero(t *testing.T) {
	t.Parallel()

	got, err := valuehistory.TotalResultOfPeriod(nil, day(1), day(30), day(15))

	require.NoError(t, err)
	assert.Equal(t, day(15), got.EndDay)
	assert.Equal(t, "0", got.TotalResult.String())
}

func TestTotalResultOfPeriod_InvalidPeriods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from, to int
		wantErr  error
	}{
		{"the first day is after the last day", 14, 12, valuehistory.ErrPeriodReversed},
		{"the period lies after today", 16, 20, valuehistory.ErrPeriodAfterToday},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := valuehistory.TotalResultOfPeriod(history(), day(tc.from), day(tc.to), day(15))

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}
