package httpapi_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// allBenchmarksResponse is the body of GET /api/all-accounts/benchmarks. It has no accountId.
type allBenchmarksResponse struct {
	Period     string         `json:"period"`
	Currency   string         `json:"currency"`
	Available  bool           `json:"available"`
	Warnings   []string       `json:"warnings"`
	BaseDay    *string        `json:"baseDay"`
	LastDay    *string        `json:"lastDay"`
	User       userReturnDTO  `json:"user"`
	Benchmarks []benchmarkDTO `json:"benchmarks"`
}

// allBenchmarks returns the Benchmarks of all Accounts for the period. It stops the test if the status is not 200.
func (s *testServer) allBenchmarks(t *testing.T, period string) allBenchmarksResponse {
	t.Helper()

	var got allBenchmarksResponse
	status := s.getJSON(t, "/api/all-accounts/benchmarks?period="+period, &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// allBenchmarksServer has a PLN Account and a USD Account. The Benchmark closes are stored.
//
//	PLN Account: Deposit 1000.00 on 2026-10-01, interest 10.00 on 2026-10-20 = 1010 PLN today
//	USD Account: Deposit 250.00 on 2026-10-01, USD rate 4.00 on 2026-10-01, 3.90 on 2026-10-15, 3.80 on 2026-10-23
//	             250 USD × 3.80 = 950 PLN today
//	All Accounts: 2000 PLN on the first day, 1010 + 950 = 1960 PLN today, so All = 1960 / 2000 − 1 = −2 %
//	S&P 500 (USD): 500 on 2026-10-01 × 4.00 = 2000 PLN, last close 540 (2026-10-22) × 3.80 = 2052 PLN
//	               2052 / 2000 − 1 = 2.6 %, difference −2 − 2.6 = −4.6 pp
//	WIG20TR (PLN): 104 on 2026-10-01, last close 117 (2026-10-22), 117 / 104 − 1 = 12.5 %, difference −2 − 12.5 = −14.5 pp
func allBenchmarksServer(t *testing.T) *testServer {
	t.Helper()

	provider := newFakeProvider()
	provider.setHistory(t, "CSPX.L", "USD", "2026-10-01=500", "2026-10-15=520", "2026-10-22=540")
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	return allBenchmarksServerWith(t, provider)
}

// allBenchmarksServerWith is allBenchmarksServer with the given price provider.
func allBenchmarksServerWith(t *testing.T, provider *fakeProvider) *testServer {
	t.Helper()

	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-01", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-15", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-23", "3.80")
	srv.importAccount(t, plnInterestExport(), "XTB PLN")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "61000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("6201", "250.00", octoberAt(1)),
	}}, "XTB USD")
	srv.refreshHistory(t)
	return srv
}

func TestAllBenchmarks_PLNAndUSDAccounts_ReturnInPLNAndDifference(t *testing.T) {
	t.Parallel()
	srv := allBenchmarksServer(t)

	got := srv.allBenchmarks(t, "All")

	assert.Equal(t, allBenchmarksResponse{
		Period: "All", Currency: "PLN", Available: true, Warnings: []string{},
		BaseDay: ptr("2026-10-01"), LastDay: ptr("2026-10-23"), User: userReturnDTO{Return: ptr("-2")},
		Benchmarks: []benchmarkDTO{
			{
				ID: "sp500", Name: "S&P 500", Fund: "iShares Core S&P 500 UCITS ETF USD (Acc)", Ticker: "CSPX.L",
				QuoteCurrency: "USD", Status: "available", Return: ptr("2.6"), Difference: ptr("-4.6"),
			},
			{
				ID: "wig20tr", Name: "WIG20TR", Fund: "Beta ETF WIG20TR Portfelowy FIZ", Ticker: "ETFBW20TR.WA",
				QuoteCurrency: "PLN", Status: "available", Return: ptr("12.5"), Difference: ptr("-14.5"),
			},
			noFullPeriod("mwig40tr", "mWIG40TR", "Beta ETF mWIG40TR Portfelowy FIZ", "ETFBM40TR.WA", "PLN"),
			noFullPeriod("world", "World equities", "iShares Core MSCI World UCITS ETF USD (Acc)", "IWDA.L", "USD"),
			noFullPeriod("gold", "Gold", "iShares Physical Gold ETC", "IGLN.L", "USD"),
		},
	}, got)
}

// TestAllBenchmarks_EachPeriod_UserReturnEqualsTheValueHistory checks that the user return and the base day of
// each period are the values of GET /api/all-accounts/value-history, as the same decimal text.
func TestAllBenchmarks_EachPeriod_UserReturnEqualsTheValueHistory(t *testing.T) {
	t.Parallel()
	srv := allBenchmarksServer(t)
	history := srv.allValueHistory(t)
	cases := []struct {
		period string
		want   *string
		start  *string
	}{
		{"1M", history.TimeWeightedReturn.OneMonth, history.PeriodStarts.OneMonth},
		{"3M", history.TimeWeightedReturn.ThreeMonths, history.PeriodStarts.ThreeMonths},
		{"YTD", history.TimeWeightedReturn.YearToDate, history.PeriodStarts.YearToDate},
		{"1Y", history.TimeWeightedReturn.OneYear, history.PeriodStarts.OneYear},
		{"All", history.TimeWeightedReturn.All, history.PeriodStarts.All},
	}

	for _, tc := range cases {
		t.Run(tc.period, func(t *testing.T) {
			t.Parallel()
			got := srv.allBenchmarks(t, tc.period)

			require.NotNil(t, tc.want)
			assert.Equal(t, tc.want, got.User.Return)
			assert.Equal(t, tc.start, got.BaseDay)
			assert.Equal(t, ptr("2026-10-23"), got.LastDay)
		})
	}
}

func TestAllBenchmarks_MissingOrUnknownPeriod_Returns400(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{
		"/api/all-accounts/benchmarks",
		"/api/all-accounts/benchmarks?period=",
		"/api/all-accounts/benchmarks?period=5Y",
	} {
		status, _, _ := srv.get(t, path)

		assert.Equal(t, http.StatusBadRequest, status, path)
	}
}

// TestAllBenchmarks_ValueHistoryNotAvailable_ReturnsTheWarnings checks a USD Account with no Exchange Rate.
func TestAllBenchmarks_ValueHistoryNotAvailable_ReturnsTheWarnings(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.refreshPrices(t)

	status, _, body := srv.get(t, "/api/all-accounts/benchmarks?period=1Y")

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"period": "1Y", "currency": "PLN", "available": false, "warnings": ["No Exchange Rate for USD. Refresh the prices."],
		"baseDay": null, "lastDay": null, "user": {"return": null}, "benchmarks": []
	}`, body)
}

func TestAllBenchmarks_NoAccounts_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	got := srv.allBenchmarks(t, "All")

	assert.Equal(t, allBenchmarksResponse{
		Period: "All", Currency: "PLN", Warnings: got.Warnings, Benchmarks: []benchmarkDTO{},
	}, got)
	assert.NotNil(t, got.Warnings, "the list is empty, not null")
}

// TestAllBenchmarks_OneBenchmarkFails_OthersAreAvailable checks that a failure of the price source for one Benchmark
// does not hide the others. WIG20TR: 117 / 104 − 1 = 12.5 %, difference −2 − 12.5 = −14.5 pp.
func TestAllBenchmarks_OneBenchmarkFails_OthersAreAvailable(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("CSPX.L", errors.New("connection reset"))
	provider.setHistory(t, "ETFBW20TR.WA", "PLN", "2026-10-01=104", "2026-10-22=117")
	srv := allBenchmarksServerWith(t, provider)

	got := srv.allBenchmarks(t, "All")

	assert.True(t, got.Available)
	assert.Equal(t, sourceFailure("sp500", "S&P 500", "iShares Core S&P 500 UCITS ETF USD (Acc)", "CSPX.L", "USD"),
		benchmarkByID(t, benchmarksResponse{Benchmarks: got.Benchmarks}, "sp500"))
	wig := benchmarkByID(t, benchmarksResponse{Benchmarks: got.Benchmarks}, "wig20tr")
	assert.Equal(t, "available", wig.Status)
	assert.Equal(t, ptr("12.5"), wig.Return)
	assert.Equal(t, ptr("-14.5"), wig.Difference)
}
