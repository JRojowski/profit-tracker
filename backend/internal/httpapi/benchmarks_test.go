package httpapi_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// benchmarksResponse is the body of GET /api/accounts/{id}/benchmarks.
type benchmarksResponse struct {
	AccountID  int64          `json:"accountId"`
	Period     string         `json:"period"`
	Currency   string         `json:"currency"`
	Available  bool           `json:"available"`
	Warnings   []string       `json:"warnings"`
	BaseDay    *string        `json:"baseDay"`
	LastDay    *string        `json:"lastDay"`
	User       userReturnDTO  `json:"user"`
	Benchmarks []benchmarkDTO `json:"benchmarks"`
}

// userReturnDTO is the Time-Weighted Return of the user in the period.
type userReturnDTO struct {
	Return *string          `json:"return"`
	Series []seriesPointDTO `json:"series"`
}

// seriesPointDTO is the return from the base day to the date, as a percentage.
type seriesPointDTO struct {
	Date   string `json:"date"`
	Return string `json:"return"`
}

// benchmarkDTO is one Benchmark with its Benchmark Return and the difference in percentage points.
type benchmarkDTO struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Fund          string           `json:"fund"`
	Ticker        string           `json:"ticker"`
	QuoteCurrency string           `json:"quoteCurrency"`
	Status        string           `json:"status"`
	Return        *string          `json:"return"`
	Difference    *string          `json:"difference"`
	Series        []seriesPointDTO `json:"series"`
}

// octoberSeries returns one point for each day from 2026-10-01 to today, 2026-10-23, inclusive.
// Each point has the return of the last change on or before its day. changes maps the day of the month to the return.
func octoberSeries(changes map[int]string) []seriesPointDTO {
	series := make([]seriesPointDTO, 0, 23)
	current := "0"
	for d := 1; d <= 23; d++ {
		if r, ok := changes[d]; ok {
			current = r
		}
		series = append(series, seriesPointDTO{Date: fmt.Sprintf("2026-10-%02d", d), Return: current})
	}
	return series
}

// benchmarks returns the Benchmarks of the Account for the period. It stops the test if the status is not 200.
func (s *testServer) benchmarks(t *testing.T, accountID int64, period string) benchmarksResponse {
	t.Helper()

	var got benchmarksResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/benchmarks?period=%s", accountID, period), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// benchmarkByID returns the Benchmark with the identifier from the response. It stops the test if it is not there.
func benchmarkByID(t *testing.T, got benchmarksResponse, id string) benchmarkDTO {
	t.Helper()
	for _, b := range got.Benchmarks {
		if b.ID == id {
			return b
		}
	}
	require.Failf(t, "no benchmark", "the response has no benchmark %q", id)
	return benchmarkDTO{}
}

// noFullPeriod is a Benchmark of the list with no Benchmark Return, because it has no close on or before the base day.
func noFullPeriod(id, name, fund, ticker, quoteCurrency string) benchmarkDTO {
	return benchmarkDTO{
		ID: id, Name: name, Fund: fund, Ticker: ticker, QuoteCurrency: quoteCurrency, Status: "noFullPeriod",
		Series: []seriesPointDTO{},
	}
}

// sourceFailure is a Benchmark of the list with no Benchmark Return, because the last refresh of its closes failed.
func sourceFailure(id, name, fund, ticker, quoteCurrency string) benchmarkDTO {
	return benchmarkDTO{
		ID: id, Name: name, Fund: fund, Ticker: ticker, QuoteCurrency: quoteCurrency, Status: "sourceFailure",
		Series: []seriesPointDTO{},
	}
}

// plnInterestExport is a PLN Account with a Deposit of 1000.00 on 2026-10-01 and 10.00 of interest on 2026-10-20.
// The clock is at 2026-10-23, so the Time-Weighted Return of each period is 1010 / 1000 − 1 = 1 %.
// Each period starts at the first day, 2026-10-01, and ends today, 2026-10-23.
func plnInterestExport() xtbtest.Export {
	return xtbtest.Export{AccountNumber: "61000001", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		interest("6102", "10.00", octoberAt(20)),
		cashDeposit("6101", "1000.00", octoberAt(1)),
	}}
}

// TestBenchmarks_BenchmarkInAccountCurrency_ReturnAndDifference checks a PLN Benchmark on a PLN Account.
//
//	User:     1 % (see plnInterestExport), base day 2026-10-01, last day 2026-10-23
//	          series: 0 % until 2026-10-19, 1010 / 1000 − 1 = 1 % from the interest on 2026-10-20
//	WIG20TR:  close on 2026-10-01 = 104, close on 2026-10-15 = 110.24, last close before today (2026-10-22) = 117
//	          117 / 104 − 1 = 12.5 %, difference 1 − 12.5 = −11.5 pp
//	          series: 0 % until 2026-10-14, 110.24 / 104 − 1 = 6 % from 2026-10-15, 12.5 % from 2026-10-22
//	Others:   no close, so no data for the full period
func TestBenchmarks_BenchmarkInAccountCurrency_ReturnAndDifference(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-09-30=100", "2026-10-01=104", "2026-10-15=110.24", "2026-10-22=117")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)

	got := srv.benchmarks(t, id, "All")

	assert.Equal(t, benchmarksResponse{
		AccountID: id, Period: "All", Currency: "PLN", Available: true, Warnings: []string{},
		BaseDay: ptr("2026-10-01"), LastDay: ptr("2026-10-23"),
		User: userReturnDTO{Return: ptr("1"), Series: octoberSeries(map[int]string{20: "1"})},
		Benchmarks: []benchmarkDTO{
			noFullPeriod("sp500", "S&P 500", "iShares Core S&P 500 UCITS ETF USD (Acc)", "CSPX.L", "USD"),
			{
				ID: "wig20tr", Name: "WIG20TR", Fund: "Beta ETF WIG20TR Portfelowy FIZ", Ticker: "ETFBW20TR.WA",
				QuoteCurrency: "PLN", Status: "available", Return: ptr("12.5"), Difference: ptr("-11.5"),
				Series: octoberSeries(map[int]string{15: "6", 22: "12.5"}),
			},
			noFullPeriod("mwig40tr", "mWIG40TR", "Beta ETF mWIG40TR Portfelowy FIZ", "ETFBM40TR.WA", "PLN"),
			noFullPeriod("world", "World equities", "iShares Core MSCI World UCITS ETF USD (Acc)", "IWDA.L", "USD"),
			noFullPeriod("gold", "Gold", "iShares Physical Gold ETC", "IGLN.L", "USD"),
		},
	}, got)
}

// TestBenchmarks_USDBenchmarkOnPLNAccount_UsesTheExchangeRateOfEachDay checks the conversion with a rate that
// changes in the period.
//
//	User:     1 %, base day 2026-10-01, last day 2026-10-23
//	S&P 500:  base day 2026-10-01: 500 USD × 4.00 = 2000 PLN
//	          last day 2026-10-23: last close 540 USD (2026-10-22) × 3.80 (table of 2026-10-23) = 2052 PLN
//	          2052 / 2000 − 1 = 2.6 %, difference 1 − 2.6 = −1.6 pp
func TestBenchmarks_USDBenchmarkOnPLNAccount_UsesTheExchangeRateOfEachDay(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "CSPX.L", "USD", "2026-10-01=500", "2026-10-15=520", "2026-10-22=540")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-01", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-15", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.75")
	srv.rateProvider.set(t, "USD", "2026-10-23", "3.80")
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)

	got := benchmarkByID(t, srv.benchmarks(t, id, "All"), "sp500")

	assert.Equal(t, "available", got.Status)
	assert.Equal(t, ptr("2.6"), got.Return)
	assert.Equal(t, ptr("-1.6"), got.Difference)
	// 10-15: 520 × 3.90 = 2028 → 1.4 %. 10-22: 540 × 3.75 = 2025 → 1.25 %. 10-23: 2052 → 2.6 %.
	assert.Equal(t, octoberSeries(map[int]string{15: "1.4", 22: "1.25", 23: "2.6"}), got.Series)
}

// TestBenchmarks_PLNBenchmarkOnUSDAccount_DividesByTheExchangeRate checks the conversion of a PLN Benchmark to USD.
//
//	User:     USD Account, 1000 USD on 2026-10-01 and 10 USD of interest on 2026-10-20: 1 %
//	WIG20TR:  base day 2026-10-01: 104 PLN / 4.00 = 26 USD
//	          last day 2026-10-23: 117 PLN (2026-10-22) / 3.60 (table of 2026-10-23) = 32.5 USD
//	          32.5 / 26 − 1 = 25 %, difference 1 − 25 = −24 pp
func TestBenchmarks_PLNBenchmarkOnUSDAccount_DividesByTheExchangeRate(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-01", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-23", "3.60")
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		interest("6202", "10.00", octoberAt(20)),
		cashDeposit("6201", "1000.00", octoberAt(1)),
	}}, "XTB USD")
	srv.refreshHistory(t)

	got := srv.benchmarks(t, id, "All")

	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, ptr("1"), got.User.Return)
	wig := benchmarkByID(t, got, "wig20tr")
	assert.Equal(t, "available", wig.Status)
	assert.Equal(t, ptr("25"), wig.Return)
	assert.Equal(t, ptr("-24"), wig.Difference)
}

// TestBenchmarks_WeekendAndHoliday_UseTheLastEarlierClose checks the days with no close.
//
//	User:     1000 PLN on Saturday 2026-10-03, 10 PLN of interest on 2026-10-20: 1 %
//	WIG20TR:  base day Saturday 2026-10-03: close of Friday 2026-10-02 = 100
//	          last day 2026-10-23: 2026-10-22 is a holiday with no close, so the close of 2026-10-21 = 110
//	          110 / 100 − 1 = 10 %, difference 1 − 10 = −9 pp
func TestBenchmarks_WeekendAndHoliday_UseTheLastEarlierClose(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=90", "2026-10-02=100", "2026-10-05=104", "2026-10-21=110")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000003", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		interest("6302", "10.00", octoberAt(20)),
		cashDeposit("6301", "1000.00", octoberAt(3)),
	}}, "XTB PLN")
	srv.refreshHistory(t)

	got := srv.benchmarks(t, id, "All")

	assert.Equal(t, ptr("2026-10-03"), got.BaseDay)
	wig := benchmarkByID(t, got, "wig20tr")
	assert.Equal(t, ptr("10"), wig.Return)
	assert.Equal(t, ptr("-9"), wig.Difference)
}

// TestBenchmarks_EachPeriod_UsesTheDaysAndTheReturnOfTheTimeWeightedReturn checks the five periods.
// A PLN Account gets 1000 PLN on 2025-06-10. Each interest adds 10 % to the value:
//
//	2025-09-01: +100     → 1100       (All)
//	2025-11-15: +110     → 1210       (All, 1Y)
//	2026-01-15: +121     → 1331       (All, 1Y, YTD)
//	2026-08-01: +133.10  → 1464.10    (All, 1Y, YTD, 3M)
//	2026-10-10: +146.41  → 1610.51    (All, 1Y, YTD, 3M, 1M)
//
// The last day is today, 2026-10-23. WIG20TR has a close on each base day and its last close on 2026-10-22 is 200.
//
//	Period  base day    user      WIG20TR close  Benchmark Return    difference
//	1M      2026-09-23  10        160            200 / 160 − 1 = 25  −15
//	3M      2026-07-23  21        125            200 / 125 − 1 = 60  −39
//	YTD     2025-12-31  33.1      100            200 / 100 − 1 = 100 −66.9
//	1Y      2025-10-23  46.41     80             200 / 80 − 1 = 150  −103.59
//	All     2025-06-10  61.051    50             200 / 50 − 1 = 300  −238.949
func TestBenchmarks_EachPeriod_UsesTheDaysAndTheReturnOfTheTimeWeightedReturn(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ETFBW20TR.WA", "PLN",
		"2025-06-10=50", "2025-10-23=80", "2025-12-31=100", "2026-07-23=125", "2026-09-23=160", "2026-10-22=200")
	srv := startServerWithPrices(t, provider)
	at := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 10, 0, 0, 0, time.UTC) }
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000004", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		interest("6406", "146.41", at(2026, time.October, 10)),
		interest("6405", "133.10", at(2026, time.August, 1)),
		interest("6404", "121.00", at(2026, time.January, 15)),
		interest("6403", "110.00", at(2025, time.November, 15)),
		interest("6402", "100.00", at(2025, time.September, 1)),
		cashDeposit("6401", "1000.00", at(2025, time.June, 10)),
	}}, "XTB PLN")
	srv.refreshHistory(t)
	history := srv.valueHistory(t, id)

	// days is the number of calendar days from the base day to 2026-10-23, inclusive.
	cases := []struct {
		period, baseDay, user, benchmarkReturn, difference string
		days                                               int
		valueHistoryReturn, valueHistoryStart              *string
	}{
		{"1M", "2026-09-23", "10", "25", "-15", 31, history.TimeWeightedReturn.OneMonth, history.PeriodStarts.OneMonth},
		{"3M", "2026-07-23", "21", "60", "-39", 93, history.TimeWeightedReturn.ThreeMonths, history.PeriodStarts.ThreeMonths},
		{"YTD", "2025-12-31", "33.1", "100", "-66.9", 297, history.TimeWeightedReturn.YearToDate, history.PeriodStarts.YearToDate},
		{"1Y", "2025-10-23", "46.41", "150", "-103.59", 366, history.TimeWeightedReturn.OneYear, history.PeriodStarts.OneYear},
		{"All", "2025-06-10", "61.051", "300", "-238.949", 501, history.TimeWeightedReturn.All, history.PeriodStarts.All},
	}
	for _, tc := range cases {
		t.Run(tc.period, func(t *testing.T) {
			t.Parallel()
			got := srv.benchmarks(t, id, tc.period)

			assert.Equal(t, tc.period, got.Period)
			assert.True(t, got.Available)
			assert.Equal(t, ptr(tc.baseDay), got.BaseDay)
			assert.Equal(t, ptr("2026-10-23"), got.LastDay)
			assert.Equal(t, ptr(tc.user), got.User.Return)
			assert.Equal(t, tc.valueHistoryReturn, got.User.Return, "the return of the user is the value of the Value History")
			assert.Equal(t, tc.valueHistoryStart, got.BaseDay, "the base day is the period start of the Value History")
			wig := benchmarkByID(t, got, "wig20tr")
			assert.Equal(t, ptr(tc.benchmarkReturn), wig.Return)
			assert.Equal(t, ptr(tc.difference), wig.Difference)
			for name, series := range map[string][]seriesPointDTO{"user": got.User.Series, "wig20tr": wig.Series} {
				require.Len(t, series, tc.days, name)
				assert.Equal(t, seriesPointDTO{Date: tc.baseDay, Return: "0"}, series[0], name)
			}
			assert.Equal(t, seriesPointDTO{Date: "2026-10-23", Return: tc.user}, got.User.Series[tc.days-1])
			assert.Equal(t, seriesPointDTO{Date: "2026-10-23", Return: tc.benchmarkReturn}, wig.Series[tc.days-1])
		})
	}
}

// TestBenchmarks_Series_UserChainsTheDailyFactors checks the user series of an Account with a deposit and a loss.
// The deposit is a flow, so it does not change the return.
//
//	2026-10-01: deposit 1000                      value 1000
//	2026-10-10: interest 100   1100 / 1000 = 1.1  value 1100   return 10 %
//	2026-10-15: deposit 900    (2000 − 900) / 1100 = 1        value 2000   return 10 %
//	2026-10-20: fee −200       1800 / 2000 = 0.9  value 1800   return 1.1 × 0.9 − 1 = −1 %
//
// The 1M period starts on 2026-09-23, before the first day, so its base day is the first day.
func TestBenchmarks_Series_UserChainsTheDailyFactors(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000007", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{ID: "6704", Type: "Withholding tax", Time: octoberAt(20), Amount: "-200.00"},
		cashDeposit("6703", "900.00", octoberAt(15)),
		interest("6702", "100.00", octoberAt(10)),
		cashDeposit("6701", "1000.00", octoberAt(1)),
	}}, "XTB PLN")

	got := srv.benchmarks(t, id, "1M")

	assert.Equal(t, ptr("-1"), got.User.Return)
	assert.Equal(t, octoberSeries(map[int]string{10: "10", 20: "-1"}), got.User.Series)
}

func TestBenchmarks_UnknownOrMissingPeriod_Gives400(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")

	for _, query := range []string{"", "?period=", "?period=1W", "?period=all"} {
		status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/benchmarks%s", id, query))

		assert.Equal(t, http.StatusBadRequest, status, "%q: body %q", query, body)
		assert.Contains(t, body, "period", "%q", query)
	}
}

func TestBenchmarks_UnknownAccount_Gives404(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{"/api/accounts/999/benchmarks?period=All", "/api/accounts/abc/benchmarks?period=All"} {
		status, _, body := srv.get(t, path)

		assert.Equal(t, http.StatusNotFound, status, "%s: body %q", path, body)
	}
}

// TestBenchmarks_NoTimeWeightedReturn_NotAvailable checks the empty comparison. An Account with no Cash Operations
// has no Value History. An Account whose first Cash Operation is today has no day with a value before it.
func TestBenchmarks_NoTimeWeightedReturn_NotAvailable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ops  []xtbtest.CashOperation
	}{
		{name: "no cash operations", ops: nil},
		{name: "first cash operation today", ops: []xtbtest.CashOperation{cashDeposit("6501", "1000.00", octoberAt(23))}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := newFakeProvider()
			provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
			srv := startServerWithPrices(t, provider)
			id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000005", Currency: "PLN", CashOperations: tc.ops}, "XTB PLN")
			srv.refreshHistory(t)

			got := srv.benchmarks(t, id, "1M")

			assert.Equal(t, benchmarksResponse{
				AccountID: id, Period: "1M", Currency: "PLN", Available: false, Warnings: []string{},
				User: userReturnDTO{Series: []seriesPointDTO{}}, Benchmarks: []benchmarkDTO{},
			}, got)
		})
	}
}

// TestBenchmarks_JSONShape_NullsAndEmptyLists checks the exact body of a comparison that is not available.
func TestBenchmarks_JSONShape_NullsAndEmptyLists(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "61000006", Currency: "USD"}, "XTB USD")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/benchmarks?period=YTD", id))

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, fmt.Sprintf(`{
		"accountId": %d, "period": "YTD", "currency": "USD", "available": false, "warnings": [],
		"baseDay": null, "lastDay": null, "user": {"return": null, "series": []}, "benchmarks": []
	}`, id), body)
}

// TestBenchmarks_FirstCloseAfterBaseDay_NoFullPeriod checks that the tool does not shorten the period.
// The base day is 2026-10-01 and the first close of mWIG40TR is on 2026-10-02.
func TestBenchmarks_FirstCloseAfterBaseDay_NoFullPeriod(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ETFBM40TR.WA", "PLN", "2026-10-02=50", "2026-10-22=55")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)

	got := benchmarkByID(t, srv.benchmarks(t, id, "All"), "mwig40tr")

	assert.Equal(t, noFullPeriod("mwig40tr", "mWIG40TR", "Beta ETF mWIG40TR Portfelowy FIZ", "ETFBM40TR.WA", "PLN"), got)
}

// TestBenchmarks_OneBenchmarkFails_OthersAreAvailable checks that a failure of the price source for one Benchmark
// does not hide the others. WIG20TR: 117 / 104 − 1 = 12.5 %.
func TestBenchmarks_OneBenchmarkFails_OthersAreAvailable(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("CSPX.L", errors.New("connection reset"))
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)

	got := srv.benchmarks(t, id, "All")

	assert.Equal(t, sourceFailure("sp500", "S&P 500", "iShares Core S&P 500 UCITS ETF USD (Acc)", "CSPX.L", "USD"),
		benchmarkByID(t, got, "sp500"))
	assert.Equal(t, ptr("12.5"), benchmarkByID(t, got, "wig20tr").Return)
	assert.Equal(t, "noFullPeriod", benchmarkByID(t, got, "mwig40tr").Status, "no failure, only no closes")
}

// TestBenchmarks_BenchmarkFailsWithInstrument_InstrumentClosesAreStored checks that the failure of one Benchmark
// does not stop the refresh of the Instruments or of the other Benchmarks. The response is a success.
func TestBenchmarks_BenchmarkFailsWithInstrument_InstrumentClosesAreStored(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.failHistory("CSPX.L", errors.New("connection reset"))
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)

	got := srv.benchmarks(t, id, "All")

	assert.Equal(t, "sourceFailure", benchmarkByID(t, got, "sp500").Status)
	assert.Equal(t, benchmarkRequests("2026-09-24", "2026-10-22"), provider.adjustedRequests(), "each Benchmark is asked")
	assert.Equal(t, []string{"2026-10-19=101", "2026-10-20=103", "2026-10-21=105", "2026-10-22=104"}, srv.storedCloses(t, "ABC.US"))
	assert.Equal(t, []string{"2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52"}, srv.storedCloses(t, "XYZ.US"))
}

// TestBenchmarks_FailureWinsOverNoFullPeriod_AndLaterSuccessMakesItAvailable checks that a failure of the last
// refresh wins, and that the next successful refresh clears it. WIG20TR: 117 / 104 − 1 = 12.5 %.
func TestBenchmarks_FailureWinsOverNoFullPeriod_AndLaterSuccessMakesItAvailable(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("ETFBW20TR.WA", errors.New("connection reset"))
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)
	failed := benchmarkByID(t, srv.benchmarks(t, id, "All"), "wig20tr")

	provider.failHistory("ETFBW20TR.WA", nil)
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	srv.refreshHistory(t)
	recovered := benchmarkByID(t, srv.benchmarks(t, id, "All"), "wig20tr")

	assert.Equal(t, "sourceFailure", failed.Status)
	assert.Nil(t, failed.Return)
	assert.Nil(t, failed.Difference)
	assert.Equal(t, "available", recovered.Status)
	assert.Equal(t, ptr("12.5"), recovered.Return)
	assert.Equal(t, ptr("-11.5"), recovered.Difference)
}

// TestBenchmarks_CloseInOtherCurrency_NotStoredAndAskedAgain checks a price source that gives GBP for a USD
// Benchmark. The tool does not guess, so the Benchmark has the status "sourceFailure" and no return. It does not record the days, so the next
// refresh asks for them again.
func TestBenchmarks_CloseInOtherCurrency_NotStoredAndAskedAgain(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "CSPX.L", "GBP", "2026-10-01=400", "2026-10-22=410")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.refreshHistory(t)

	got := benchmarkByID(t, srv.benchmarks(t, id, "All"), "sp500")
	srv.refreshHistory(t)

	assert.Equal(t, "sourceFailure", got.Status)
	assert.Nil(t, got.Return)
	assert.Nil(t, got.Difference)
	var cspx []historyRequest
	for _, r := range provider.adjustedRequests() {
		if r.Ticker == "CSPX.L" {
			cspx = append(cspx, r)
		}
	}
	assert.Equal(t, []historyRequest{
		{Ticker: "CSPX.L", From: "2026-09-24", To: "2026-10-22"},
		{Ticker: "CSPX.L", From: "2026-09-24", To: "2026-10-22"},
	}, cspx)
}

// benchmarkTickers are the price tickers of the Benchmarks, in the order of the list.
var benchmarkTickers = []string{"CSPX.L", "ETFBW20TR.WA", "ETFBM40TR.WA", "IWDA.L", "IGLN.L"}

// benchmarkRequests returns the request of each Benchmark from the day from to the day to, in the order of the list.
func benchmarkRequests(from, to string) []historyRequest {
	requests := make([]historyRequest, 0, len(benchmarkTickers))
	for _, ticker := range benchmarkTickers {
		requests = append(requests, historyRequest{Ticker: ticker, From: from, To: to})
	}
	return requests
}

// TestRefreshPrices_Benchmarks_RequestsEachBenchmarkFromSevenDaysBeforeFirstCashOperation checks the range of
// the Benchmark closes. The first Cash Operation is on 2026-10-01, so the range starts on 2026-09-24.
// The clock is at 2026-10-23, so the range ends yesterday, 2026-10-22.
func TestRefreshPrices_Benchmarks_RequestsEachBenchmarkFromSevenDaysBeforeFirstCashOperation(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	srv.refreshHistory(t)

	assert.Equal(t, benchmarkRequests("2026-09-24", "2026-10-22"), provider.adjustedRequests())
	assert.Len(t, provider.historyRequests(), 2, "the Instruments get their history as before")
	assert.Equal(t, 7, srv.pauses.count(), "the tool pauses before each history request, also of a Benchmark")
}

func TestRefreshPrices_SecondRefresh_AsksNoBenchmarkDaysAgain(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)
	srv.clock.set(testToday.AddDate(0, 0, 2))

	srv.refreshHistory(t)

	want := append(benchmarkRequests("2026-09-24", "2026-10-22"), benchmarkRequests("2026-10-23", "2026-10-24")...)
	assert.Equal(t, want, provider.adjustedRequests(), "the second refresh asks only for the days after the stored range")
}

func TestRefreshPrices_NoAccounts_AsksForNoBenchmark(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	srv := startServerWithPrices(t, provider)

	srv.refreshHistory(t)

	assert.Empty(t, provider.adjustedRequests())
}

func TestRefreshPrices_OneBenchmarkFails_OthersAndInstrumentsAreAsked(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.failHistory("CSPX.L", errors.New("connection reset"))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.Equal(t, benchmarkRequests("2026-09-24", "2026-10-22"), provider.adjustedRequests())
	assert.Equal(t, historyRefreshResponse{HistoryStored: 7, HistoryFailures: []refreshFailure{}}, refresh,
		"a Benchmark is not an Instrument, so the Instrument failures do not name it")
}

func TestRefreshPrices_BenchmarkTooManyRequests_StopsTheRefresh(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.failHistory("ETFBW20TR.WA", fmt.Errorf("yahoo: %w", price.ErrTooManyRequests))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.True(t, refresh.TooManyRequests)
	assert.Equal(t, benchmarkRequests("2026-09-24", "2026-10-22")[:2], provider.adjustedRequests(),
		"the refresh sends no request after the block")
}
