package yahoo_test

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/price/yahoo"
)

// historyNow is the clock of the provider in the history tests: 12:00 UTC on 2026-10-23.
var historyNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// recorded returns the content of the recorded response in testdata.
func recorded(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

// startHistory starts a server that answers each request with the status and the body.
// It returns a provider for the server with the clock at historyNow.
func startHistory(t *testing.T, status int, body string) (*yahoo.Provider, *recorder) {
	t.Helper()
	provider, requests := startYahoo(t, status, body)
	provider.Now = func() time.Time { return historyNow }
	return provider, requests
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// closes returns the closes of the history as "YYYY-MM-DD=close" texts, in order.
func closes(h price.History) []string {
	got := make([]string, 0, len(h.Closes))
	for _, c := range h.Closes {
		got = append(got, c.Date.Format(time.DateOnly)+"="+c.Close.String())
	}
	return got
}

// TestDailyCloses_SplitAfterDay_ReturnsRawClose checks the example of ADR 0005.
// AAPL had a 4:1 split on 2020-08-31. The close of 124.81 on 2020-08-28 gives 124.81 × 4 = 499.24.
func TestDailyCloses_SplitAfterDay_ReturnsRawClose(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusOK, recorded(t, "aapl_split.json"))

	got, err := provider.DailyCloses(t.Context(), "AAPL", day(2020, 8, 27), day(2020, 9, 1))

	require.NoError(t, err)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, yahoo.Source, got.Source)
	assert.Equal(t, []string{
		"2020-08-27=500.04", "2020-08-28=499.24", "2020-08-31=129.04", "2020-09-01=134.18",
	}, closes(got), "a close before the split day is multiplied by 4, a close on or after it is not")
}

func TestDailyCloses_Request_UsesPeriodsDailyIntervalAndSplitEvents(t *testing.T) {
	t.Parallel()
	provider, requests := startHistory(t, http.StatusOK, recorded(t, "aapl_split.json"))

	_, err := provider.DailyCloses(t.Context(), "AAPL", day(2020, 8, 27), day(2020, 9, 1))

	require.NoError(t, err)
	all := requests.all()
	require.Len(t, all, 1)
	assert.Equal(t, "/v8/finance/chart/AAPL", all[0].path)
	query, err := url.ParseQuery(all[0].rawQuery)
	require.NoError(t, err)
	assert.False(t, query.Has("range"), "range=max gives monthly data")
	assert.Equal(t, "1d", query.Get("interval"))
	assert.Equal(t, "split", query.Get("events"))
	period1, err := strconv.ParseInt(query.Get("period1"), 10, 64)
	require.NoError(t, err)
	period2, err := strconv.ParseInt(query.Get("period2"), 10, 64)
	require.NoError(t, err)
	assert.LessOrEqual(t, period1, day(2020, 8, 27).Unix(), "the first day must be in the period")
	assert.Equal(t, historyNow.Unix(), period2, "the period goes to now, so that the response has each later split")
	assert.NotContains(t, all[0].userAgent, "Go-http-client")
}

func TestDailyCloses_LondonPriceInPence_ReturnsPounds(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusOK, recorded(t, "vusa_pence.json"))

	got, err := provider.DailyCloses(t.Context(), "VUSA.L", day(2026, 10, 20), day(2026, 10, 22))

	require.NoError(t, err)
	assert.Equal(t, "GBP", got.Currency)
	assert.Equal(t, []string{"2026-10-20=82.345", "2026-10-21=83"}, closes(got))
}

func TestDailyCloses_NullCloseAndRowOfToday_AreSkipped(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusOK, recorded(t, "null_close_and_today.json"))

	got, err := provider.DailyCloses(t.Context(), "SAP.DE", day(2026, 10, 20), day(2026, 10, 23))

	require.NoError(t, err)
	assert.Equal(t, "EUR", got.Currency)
	assert.Equal(t, []string{"2026-10-20=229.45", "2026-10-22=230.85"}, closes(got),
		"the row of 2026-10-21 has no close, and the row of today is a live price")
}

func TestDailyCloses_RowsOutsideAskedDays_AreSkipped(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusOK, recorded(t, "aapl_split.json"))

	got, err := provider.DailyCloses(t.Context(), "AAPL", day(2020, 8, 28), day(2020, 8, 31))

	require.NoError(t, err)
	assert.Equal(t, []string{"2020-08-28=499.24", "2020-08-31=129.04"}, closes(got))
}

func TestDailyCloses_UnknownSymbol_ReturnsErrorWithDescription(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusNotFound, recorded(t, "not_found.json"))

	_, err := provider.DailyCloses(t.Context(), "NOPE", day(2026, 1, 2), day(2026, 10, 22))

	require.Error(t, err)
	require.NotErrorIs(t, err, price.ErrTooManyRequests)
	assert.Contains(t, err.Error(), "NOPE")
	assert.Contains(t, err.Error(), "No data found, symbol may be delisted")
}

func TestDailyCloses_MonthlyResponse_ReturnsError(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusOK, recorded(t, "monthly.json"))

	_, err := provider.DailyCloses(t.Context(), "AAPL", day(2020, 8, 1), day(2020, 9, 30))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "AAPL")
	assert.Contains(t, err.Error(), "1mo", "the error must name the granularity of the response")
}

func TestDailyCloses_TooManyRequests_ReturnsSentinelError(t *testing.T) {
	t.Parallel()
	provider, _ := startHistory(t, http.StatusTooManyRequests, "Too Many Requests\n")

	_, err := provider.DailyCloses(t.Context(), "AAPL", day(2026, 1, 2), day(2026, 10, 22))

	require.ErrorIs(t, err, price.ErrTooManyRequests)
	assert.Contains(t, err.Error(), "AAPL")
}

func TestLastClose_TooManyRequests_ReturnsSentinelError(t *testing.T) {
	t.Parallel()
	provider, _ := startYahoo(t, http.StatusTooManyRequests, "Too Many Requests\n")

	_, err := provider.LastClose(t.Context(), "AAPL")

	require.ErrorIs(t, err, price.ErrTooManyRequests)
}
