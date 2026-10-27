package httpapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// historyRefreshResponse is the body of POST /api/prices/refresh with the fields of the history step.
type historyRefreshResponse struct {
	Refreshed       int              `json:"refreshed"`
	HistoryStored   int              `json:"historyStored"`
	HistoryFailures []refreshFailure `json:"historyFailures"`
	TooManyRequests bool             `json:"tooManyRequests"`
	StopMessage     *string          `json:"stopMessage"`
}

// progressResponse is the body of GET /api/prices/refresh/progress.
type progressResponse struct {
	Running bool    `json:"running"`
	Done    int     `json:"done"`
	Total   int     `json:"total"`
	Ticker  *string `json:"ticker"`
}

// tooManyRequestsMessage is the stop message of a refresh that the price source blocks.
const tooManyRequestsMessage = "Yahoo blocks the requests, try again later"

// historyTime returns the time on day n of October 2026, at 14:00 UTC.
func historyTime(day int) time.Time {
	return time.Date(2026, 10, day, 14, 0, 0, 0, time.UTC)
}

// historyExport is a USD Account that sold ABC.US and holds XYZ.US, with the extra Cash Operations.
// The clock of the test server is at 2026-10-23, so yesterday is 2026-10-22.
//
//	ABC.US: first Fill on 2026-10-19, sold on 2026-10-21
//	XYZ.US: first Fill on 2026-10-20, held
func historyExport(extra ...xtbtest.CashOperation) xtbtest.Export {
	ops := append([]xtbtest.CashOperation(nil), extra...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Stock sell", Ticker: "ABC.US", Instrument: "ABC", Time: historyTime(21), Amount: "210.00", ID: "805", Comment: "CLOSE BUY 2/2 @ 105.00", PositionID: "8000000001"},
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Time: historyTime(20), Amount: "-50.00", ID: "804", Comment: "OPEN BUY 1 @ 50.00", PositionID: "8000000002"},
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Time: historyTime(19), Amount: "-200.00", ID: "803", Comment: "OPEN BUY 2 @ 100.00", PositionID: "8000000001"},
		xtbtest.CashOperation{Type: "Deposit", Time: historyTime(1), Amount: "1000.00", ID: "801", Comment: "Blik deposit"},
	)
	return xtbtest.Export{
		AccountNumber: "58100001", Currency: "USD", CashOperations: ops,
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "ABC.US", Instrument: "ABC", Volume: "2", OpenTime: historyTime(19), CloseTime: historyTime(21), ProfitLoss: "10.00", PositionID: "8000000001"},
		},
	}
}

// historyProvider returns a fake price provider with the daily closes of ABC and XYZ.
func historyProvider(t *testing.T) *fakeProvider {
	t.Helper()
	provider := newFakeProvider()
	provider.setHistory(t, "ABC", "USD", "2026-10-19=101", "2026-10-20=103", "2026-10-21=105", "2026-10-22=104")
	provider.setHistory(t, "XYZ", "USD", "2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52")
	return provider
}

// refreshHistory sends POST /api/prices/refresh and returns the fields of the history step.
func (s *testServer) refreshHistory(t *testing.T) historyRefreshResponse {
	t.Helper()

	status, body := s.postJSON(t, "/api/prices/refresh", nil)
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got historyRefreshResponse
	decode(t, body, &got)
	return got
}

// progress returns the body of GET /api/prices/refresh/progress.
func (s *testServer) progress(t *testing.T) progressResponse {
	t.Helper()

	var got progressResponse
	status := s.getJSON(t, "/api/prices/refresh/progress", &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// storedCloses returns the stored prices of the Instrument with the broker ticker as "YYYY-MM-DD=close", in date order.
func (s *testServer) storedCloses(t *testing.T, ticker string) []string {
	t.Helper()

	all, err := s.db.PriceHistory(t.Context())
	require.NoError(t, err)
	got := []string{}
	for _, p := range all[ticker] {
		got = append(got, p.Date.Format(time.DateOnly)+"="+p.Close.String())
	}
	return got
}

func TestRefreshPrices_SoldInstrument_GetsHistoryFromFirstFillToYesterday(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.Equal(t, historyRefreshResponse{HistoryStored: 7, HistoryFailures: []refreshFailure{}}, refresh)
	assert.Equal(t, []historyRequest{
		{Ticker: "ABC", From: "2026-10-19", To: "2026-10-22"},
		{Ticker: "XYZ", From: "2026-10-20", To: "2026-10-22"},
	}, provider.historyRequests())
	assert.Equal(t, []string{"XYZ"}, provider.requestedTickers(), "only a held Instrument gets a current price")
	assert.Equal(t, []string{"2026-10-19=101", "2026-10-20=103", "2026-10-21=105", "2026-10-22=104"}, srv.storedCloses(t, "ABC.US"))
	assert.Equal(t, []string{"2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52"}, srv.storedCloses(t, "XYZ.US"))
	assert.Equal(t, 2, srv.pauses.count(), "the tool pauses before each history request")
}

func TestRefreshPrices_SecondRefresh_AsksOnlyMissingDays(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)

	srv.refreshHistory(t)
	assert.Len(t, provider.historyRequests(), 2, "a second refresh on the same day asks for no history")

	srv.clock.set(time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC))
	provider.setHistory(t, "ABC", "USD", "2026-10-23=106", "2026-10-26=107")
	refresh := srv.refreshHistory(t)

	assert.Equal(t, 2, refresh.HistoryStored)
	assert.Equal(t, []historyRequest{
		{Ticker: "ABC", From: "2026-10-23", To: "2026-10-26"},
		{Ticker: "XYZ", From: "2026-10-23", To: "2026-10-26"},
	}, provider.historyRequests()[2:])
	assert.Equal(t, []string{
		"2026-10-19=101", "2026-10-20=103", "2026-10-21=105", "2026-10-22=104", "2026-10-23=106", "2026-10-26=107",
	}, srv.storedCloses(t, "ABC.US"), "the tool keeps the stored prices")
}

func TestRefreshPrices_HolidayWithNoRow_IsNotAskedAgain(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	// 2026-10-22 is a holiday: the price source has no row for it.
	provider.setHistory(t, "XYZ", "USD", "2026-10-20=50.5", "2026-10-21=51")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)

	srv.clock.set(testToday.AddDate(0, 0, 1))
	srv.refreshHistory(t)

	assert.Equal(t, []historyRequest{
		{Ticker: "ABC", From: "2026-10-19", To: "2026-10-22"},
		{Ticker: "XYZ", From: "2026-10-20", To: "2026-10-22"},
		{Ticker: "ABC", From: "2026-10-23", To: "2026-10-23"},
		{Ticker: "XYZ", From: "2026-10-23", To: "2026-10-23"},
	}, provider.historyRequests())
}

func TestRefreshPrices_EarlierFillInNewImport_AsksForTheEarlierDays(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)
	// A second Account bought XYZ.US before the first Fill of the first Account.
	srv.importAccount(t, xtbtest.Export{AccountNumber: "58100002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Time: historyTime(15), Amount: "-49.00", ID: "902", Comment: "OPEN BUY 1 @ 49.00", PositionID: "8100000001"},
		{Type: "Deposit", Time: historyTime(14), Amount: "100.00", ID: "901", Comment: "Blik deposit"},
	}}, "XTB USD 2")

	srv.refreshHistory(t)

	assert.Equal(t, []historyRequest{{Ticker: "XYZ", From: "2026-10-15", To: "2026-10-19"}}, provider.historyRequests()[2:])
}

func TestRefreshPrices_TooManyRequests_StopsAndKeepsStoredPrices(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.setHistory(t, "ZZZ", "USD", "2026-10-20=9")
	provider.failHistory("XYZ", fmt.Errorf("yahoo daily prices of XYZ: status 429: %w", price.ErrTooManyRequests))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "ZZZ.US", Instrument: "ZZZ", Time: historyTime(20).Add(time.Minute), Amount: "-9.00", ID: "806", Comment: "OPEN BUY 1 @ 9.00", PositionID: "8000000003"},
	), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.Equal(t, historyRefreshResponse{
		HistoryStored: 4, HistoryFailures: []refreshFailure{}, TooManyRequests: true, StopMessage: ptr(tooManyRequestsMessage),
	}, refresh)
	assert.Equal(t, []string{"ABC", "XYZ"}, historyTickers(provider.historyRequests()), "the refresh must stop at the first HTTP 429")
	assert.Len(t, srv.storedCloses(t, "ABC.US"), 4, "the prices stored before the stop stay")

	// The next refresh continues from that point.
	provider.failHistory("XYZ", nil)
	refresh = srv.refreshHistory(t)

	assert.False(t, refresh.TooManyRequests)
	assert.Nil(t, refresh.StopMessage)
	assert.Equal(t, []historyRequest{
		{Ticker: "XYZ", From: "2026-10-20", To: "2026-10-22"},
		{Ticker: "ZZZ", From: "2026-10-20", To: "2026-10-22"},
	}, provider.historyRequests()[2:])
}

func TestRefreshPrices_TooManyRequestsForCurrentPrice_AsksForNoHistory(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.failWith(fmt.Errorf("yahoo price of XYZ: status 429: %w", price.ErrTooManyRequests))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.True(t, refresh.TooManyRequests)
	assert.Equal(t, ptr(tooManyRequestsMessage), refresh.StopMessage)
	assert.Empty(t, provider.historyRequests())
}

func TestRefreshPrices_HistoryFails_ReportsInstrumentAndAsksAgainNextTime(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.failHistory("ABC", errors.New("yahoo daily prices of ABC: status 404: Not Found: No data found, symbol may be delisted"))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshHistory(t)

	assert.Equal(t, historyRefreshResponse{HistoryStored: 3, HistoryFailures: []refreshFailure{
		{Ticker: "ABC.US", Error: "yahoo daily prices of ABC: status 404: Not Found: No data found, symbol may be delisted"},
	}}, refresh)

	srv.refreshHistory(t)

	assert.Equal(t, []historyRequest{{Ticker: "ABC", From: "2026-10-19", To: "2026-10-22"}}, provider.historyRequests()[2:],
		"a failed request is not a stored range")
}

// TestRefreshPrices_HistoryInOtherCurrency_StoresClosesAndReportsInstrument checks a history in a currency that
// is not the Quote Currency. The tool stores the closes with their currency and records the asked range.
// The Value History uses the closes when the user corrects the Quote Currency.
//
//	EUR Account: Deposit 1000.00 on 2026-10-01, XYZ.US 1 for 50.00 on 2026-10-20.
//	The price source gives the history of XYZ in EUR, but the suffix ".US" gives the Quote Currency USD.
func TestRefreshPrices_HistoryInOtherCurrency_StoresClosesAndReportsInstrument(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "XYZ", "EUR", "2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "58100003", Currency: "EUR", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Time: historyTime(20), Amount: "-50.00", ID: "1002", Comment: "OPEN BUY 1 @ 50.00", PositionID: "8200000001"},
		cashDeposit("1001", "1000.00", historyTime(1)),
	}}, "XTB EUR")

	refresh := srv.refreshHistory(t)

	assert.Equal(t, 3, refresh.HistoryStored)
	assert.Equal(t, []refreshFailure{{Ticker: "XYZ.US", Error: "the price is in EUR, but the Instrument is quoted in USD"}},
		refresh.HistoryFailures, "the user must see the Instrument with the other currency")
	assert.Equal(t, []string{"2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52"}, srv.storedCloses(t, "XYZ.US"))
	assert.Equal(t, []valuedAtCostDTO{{Ticker: "XYZ.US", Name: "XYZ"}}, srv.valueHistory(t, id).ValuedAtCost,
		"the Value History uses no price in another currency")

	srv.clock.set(testToday.AddDate(0, 0, 1))
	srv.refreshHistory(t)

	assert.Equal(t, []historyRequest{
		{Ticker: "XYZ", From: "2026-10-20", To: "2026-10-22"},
		{Ticker: "XYZ", From: "2026-10-23", To: "2026-10-23"},
	}, provider.historyRequests(), "the second refresh asks only for the missing days")

	srv.clock.set(testToday)
	srv.setQuoteCurrency(t, "XYZ.US", "EUR")
	got := srv.valueHistory(t, id)

	require.Len(t, got.Points, 23)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-20", Value: "1000.50", NetDeposits: "1000.00"},
		{Date: "2026-10-21", Value: "1001.00", NetDeposits: "1000.00"},
		{Date: "2026-10-22", Value: "1002.00", NetDeposits: "1000.00"},
	}, got.Points[19:22], "the Value History uses the stored closes of XYZ.US")
	assert.Empty(t, got.ValuedAtCost)
}

func TestRefreshPrices_PriceTickerChange_DeletesPricesAndAskedRange(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.setHistory(t, "ABC.NEW", "USD", "2026-10-19=1", "2026-10-20=2")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)

	srv.setPriceTicker(t, "ABC.US", "ABC.NEW")

	assert.Empty(t, srv.storedCloses(t, "ABC.US"), "the prices of the old price ticker must go")
	assert.Len(t, srv.storedCloses(t, "XYZ.US"), 3, "the prices of the other Instruments stay")
	srv.refreshHistory(t)
	assert.Equal(t, []historyRequest{{Ticker: "ABC.NEW", From: "2026-10-19", To: "2026-10-22"}}, provider.historyRequests()[2:])
	assert.Equal(t, []string{"2026-10-19=1", "2026-10-20=2"}, srv.storedCloses(t, "ABC.US"))
}

func TestRefreshPrices_LivePriceOfToday_IsReplacedByTheCloseNextDay(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	// The current price of today is a live price during the session.
	provider.set(t, "XYZ", "52.50", "USD", "2026-10-23")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)
	require.Equal(t, "2026-10-23=52.5", srv.storedCloses(t, "XYZ.US")[3])

	srv.clock.set(testToday.AddDate(0, 0, 1))
	provider.set(t, "XYZ", "53.00", "USD", "2026-10-24")
	provider.setHistory(t, "XYZ", "USD", "2026-10-23=52.9")
	srv.refreshHistory(t)

	assert.Equal(t, []string{"2026-10-20=50.5", "2026-10-21=51", "2026-10-22=52", "2026-10-23=52.9", "2026-10-24=53"},
		srv.storedCloses(t, "XYZ.US"), "the close of the history replaces the live price")

	// ADR 0005: a stored close of the history never changes.
	srv.clock.set(testToday.AddDate(0, 0, 2))
	provider.set(t, "XYZ", "99.00", "USD", "2026-10-23")
	srv.refreshHistory(t)

	assert.Equal(t, "2026-10-23=52.9", srv.storedCloses(t, "XYZ.US")[3])
}

func TestRefreshProgress_ShowsTheRunningHistoryRequest(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")
	assert.Equal(t, progressResponse{}, srv.progress(t), "no refresh runs")
	started, release := make(chan struct{}), make(chan struct{})
	provider.setOnHistory(func(ticker string) {
		if ticker == "XYZ" {
			close(started)
			<-release
		}
	})
	done := make(chan error, 1)
	go func() {
		_, err := srv.prices.Refresh(context.WithoutCancel(t.Context()))
		done <- err
	}()
	<-started

	got := srv.progress(t)

	close(release)
	require.NoError(t, <-done)
	assert.Equal(t, progressResponse{Running: true, Done: 1, Total: 2, Ticker: ptr("XYZ.US")}, got)
	assert.Equal(t, progressResponse{}, srv.progress(t), "the refresh is done")
}

// TestRefreshAtStart_GetsNoHistory checks that only "Refresh prices" gets the history.
// The refresh at start gets only the current prices.
func TestRefreshAtStart_GetsNoHistory(t *testing.T) {
	t.Parallel()
	provider := historyProvider(t)
	provider.set(t, "XYZ", "52.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, historyExport(), "XTB USD")

	result, ran, err := srv.prices.RefreshIfStale(t.Context())

	require.NoError(t, err)
	assert.True(t, ran, "XYZ.US has no current price fetched today")
	assert.Equal(t, 1, result.Refreshed)
	assert.Zero(t, result.HistoryStored)
	assert.Empty(t, provider.historyRequests(), "the refresh at start asks for no history")
	assert.Zero(t, srv.pauses.count())

	// A refresh by the user stores daily closes. They do not tell that the current prices are current.
	srv.clock.set(testToday.AddDate(0, 0, 1))
	srv.refreshHistory(t)
	srv.clock.set(testToday.AddDate(0, 0, 2))
	_, ran, err = srv.prices.RefreshIfStale(t.Context())

	require.NoError(t, err)
	assert.True(t, ran, "a daily close is not a current price")
	assert.Len(t, provider.historyRequests(), 2, "the refresh at start asks for no history")

	_, ran, err = srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.False(t, ran, "the current prices are current, also with missing history")
}

// historyTickers returns the price ticker of each request, in request order.
func historyTickers(requests []historyRequest) []string {
	tickers := make([]string, 0, len(requests))
	for _, r := range requests {
		tickers = append(tickers, r.Ticker)
	}
	return tickers
}
