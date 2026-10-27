package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// valueHistoryResponse is the body of GET /api/accounts/{id}/value-history.
type valueHistoryResponse struct {
	AccountID          int64               `json:"accountId"`
	Currency           string              `json:"currency"`
	Points             []valuePoint        `json:"points"`
	TimeWeightedReturn timeWeightedReturns `json:"timeWeightedReturn"`
	PeriodStarts       periodStarts        `json:"periodStarts"`
	ValuedAtCost       []valuedAtCostDTO   `json:"valuedAtCost"`
	PriceWarnings      []map[string]any    `json:"priceWarnings"`
}

// valuePoint is one point of the Value History.
type valuePoint struct {
	Date        string `json:"date"`
	Value       string `json:"value"`
	NetDeposits string `json:"netDeposits"`
}

// timeWeightedReturns are the Time-Weighted Returns of the periods, as percentages in decimal text.
type timeWeightedReturns struct {
	OneMonth    *string `json:"1M"`
	ThreeMonths *string `json:"3M"`
	YearToDate  *string `json:"YTD"`
	OneYear     *string `json:"1Y"`
	All         *string `json:"All"`
}

// periodStarts are the first dates that the chart shows for the periods, as YYYY-MM-DD.
type periodStarts timeWeightedReturns

// valuedAtCostDTO is one Instrument that the tool valued at cost on one or more days.
type valuedAtCostDTO struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
}

// valueHistory returns the Value History of the Account. It stops the test if the status is not 200.
func (s *testServer) valueHistory(t *testing.T, accountID int64) valueHistoryResponse {
	t.Helper()

	var got valueHistoryResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/value-history", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// octoberAt returns the time on day n of October 2026, at 10:00 UTC (12:00 in Polish time).
func octoberAt(day int) time.Time {
	return time.Date(2026, 10, day, 10, 0, 0, 0, time.UTC)
}

func TestValueHistory_CashOnly_GivesOnePointForEachCalendarDay(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// Thursday 2026-10-15 to Friday 2026-10-23 (today) has the weekend of 2026-10-17 and 2026-10-18.
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000001", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Free funds interest", Time: octoberAt(19), Amount: "5.00", ID: "1203", Comment: "Free-funds Interest"},
		{Type: "Withdrawal", Time: octoberAt(17), Amount: "-100.00", ID: "1202", Comment: "Withdrawal"},
		cashDeposit("1201", "1000.00", octoberAt(15)),
	}}, "XTB USD")

	got := srv.valueHistory(t, id)

	assert.Equal(t, id, got.AccountID)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-15", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-16", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-17", Value: "900.00", NetDeposits: "900.00"},
		{Date: "2026-10-18", Value: "900.00", NetDeposits: "900.00"},
		{Date: "2026-10-19", Value: "905.00", NetDeposits: "900.00"},
		{Date: "2026-10-20", Value: "905.00", NetDeposits: "900.00"},
		{Date: "2026-10-21", Value: "905.00", NetDeposits: "900.00"},
		{Date: "2026-10-22", Value: "905.00", NetDeposits: "900.00"},
		{Date: "2026-10-23", Value: "905.00", NetDeposits: "900.00"},
	}, got.Points)
	assert.Empty(t, got.ValuedAtCost)
	assert.NotNil(t, got.ValuedAtCost, "the list is empty, not null")
	assert.NotNil(t, got.PriceWarnings, "the list is empty, not null")
}

func TestValueHistory_UnknownAccount_Gives404(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{"/api/accounts/999/value-history", "/api/accounts/abc/value-history"} {
		status, _, body := srv.get(t, path)

		assert.Equal(t, http.StatusNotFound, status, "%s: body %q", path, body)
	}
}

// pricedHoldingsExport is a USD Account that buys ABC.US and NOP.US. The price source has no price for NOP.US.
//
//	2026-10-14: Deposit 1000.00
//	2026-10-15: ABC.US 2 for 200.00
//	2026-10-16: NOP.US 1 for 30.00
func pricedHoldingsExport() xtbtest.Export {
	return xtbtest.Export{AccountNumber: "57000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "NOP.US", Instrument: "NOP Inc", Time: octoberAt(16), Amount: "-30.00", ID: "1303", Comment: "OPEN BUY 1 @ 30.00", PositionID: "13000000002"},
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC Inc", Time: octoberAt(15), Amount: "-200.00", ID: "1302", Comment: "OPEN BUY 2 @ 100.00", PositionID: "13000000001"},
		cashDeposit("1301", "1000.00", octoberAt(14)),
	}}
}

// TestValueHistory_Holdings_LastKnownPriceOrCost checks the value of the Holdings on each day.
//
//	ABC.US has no price before 2026-10-16 and no row on the weekend and on 2026-10-21.
//	NOP.US has no price at all.
//
//	2026-10-15: 800 + ABC at cost 200                 = 1000
//	2026-10-16: 770 + ABC 2 × 101 + NOP at cost 30    = 1002, also on the weekend
//	2026-10-19: 770 + 2 × 103 + 30                    = 1006
//	2026-10-20: 770 + 2 × 104 + 30                    = 1008, also on 2026-10-21
//	2026-10-22: 770 + 2 × 105 + 30                    = 1010
//	2026-10-23: 770 + 2 × 106 (current price) + 30    = 1012
func TestValueHistory_Holdings_LastKnownPriceOrCost(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ABC", "USD", "2026-10-16=101", "2026-10-19=103", "2026-10-20=104", "2026-10-22=105")
	provider.set(t, "ABC", "106", "USD", "2026-10-23")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedHoldingsExport(), "XTB USD")
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	assert.Equal(t, []valuePoint{
		{Date: "2026-10-14", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-15", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-16", Value: "1002.00", NetDeposits: "1000.00"},
		{Date: "2026-10-17", Value: "1002.00", NetDeposits: "1000.00"},
		{Date: "2026-10-18", Value: "1002.00", NetDeposits: "1000.00"},
		{Date: "2026-10-19", Value: "1006.00", NetDeposits: "1000.00"},
		{Date: "2026-10-20", Value: "1008.00", NetDeposits: "1000.00"},
		{Date: "2026-10-21", Value: "1008.00", NetDeposits: "1000.00"},
		{Date: "2026-10-22", Value: "1010.00", NetDeposits: "1000.00"},
		{Date: "2026-10-23", Value: "1012.00", NetDeposits: "1000.00"},
	}, got.Points)
	assert.Equal(t, []valuedAtCostDTO{{Ticker: "ABC.US", Name: "ABC Inc"}, {Ticker: "NOP.US", Name: "NOP Inc"}}, got.ValuedAtCost)
}

// TestValueHistory_HoldingInOtherQuoteCurrency_UsesExchangeRateOfEachDay checks a USD Holding on a PLN Account.
// The NBP has no table on 2026-10-21, so that day uses the rate of 2026-10-20.
//
//	2026-10-20: 200 + 2 × 100 × 4.10 = 1020
//	2026-10-21: 200 + 2 × 101 × 4.10 = 1028.2
//	2026-10-22: 200 + 2 × 102 × 4.20 = 1056.8
//	2026-10-23: 200 + 2 × 103 × 4.25 = 1075.5, the value in the hero
func TestValueHistory_HoldingInOtherQuoteCurrency_UsesExchangeRateOfEachDay(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ABC", "USD", "2026-10-20=100", "2026-10-21=101", "2026-10-22=102")
	provider.set(t, "ABC", "103", "USD", "2026-10-23")
	srv := startServerWithPrices(t, provider)
	for _, r := range [][2]string{{"2026-10-19", "4.00"}, {"2026-10-20", "4.10"}, {"2026-10-22", "4.20"}, {"2026-10-23", "4.25"}} {
		srv.rateProvider.set(t, "USD", r[0], r[1])
	}
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000003", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Time: octoberAt(20), Amount: "-800.00", ID: "1402", Comment: "OPEN BUY 2 @ 100.00", PositionID: "14000000001"},
		cashDeposit("1401", "1000.00", octoberAt(19)),
	}}, "XTB PLN")
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	assert.Equal(t, "PLN", got.Currency)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-19", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-20", Value: "1020.00", NetDeposits: "1000.00"},
		{Date: "2026-10-21", Value: "1028.20", NetDeposits: "1000.00"},
		{Date: "2026-10-22", Value: "1056.80", NetDeposits: "1000.00"},
		{Date: "2026-10-23", Value: "1075.50", NetDeposits: "1000.00"},
	}, got.Points)
	assert.Equal(t, srv.summaryReturns(t, id).Value, got.Points[len(got.Points)-1].Value, "the point of today is the value in the hero")
	assert.Empty(t, got.ValuedAtCost)
}

// TestValueHistory_QuoteCurrencyOnlyOfAnInstrument_GetsRatesFromTheFirstFill checks the Exchange Rates of a
// currency that is only a Quote Currency. No Account has the currency USD, so no Net Deposits flow tells
// the first day of the series. The series must start before the first Fill of the Instrument.
//
//	2026-09-02: 200 + 2 × 100 × 3.90 = 980
func TestValueHistory_QuoteCurrencyOnlyOfAnInstrument_GetsRatesFromTheFirstFill(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ABC", "USD", "2026-09-01=100", "2026-09-02=100")
	provider.set(t, "ABC", "103", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-09-01", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.20")
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000004", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Time: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), Amount: "-800.00", ID: "1502", Comment: "OPEN BUY 2 @ 100.00", PositionID: "15000000001"},
		cashDeposit("1501", "1000.00", time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)),
	}}, "XTB PLN")
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	require.NotEmpty(t, got.Points)
	assert.Equal(t, valuePoint{Date: "2026-09-02", Value: "980.00", NetDeposits: "1000.00"}, got.Points[1])
	assert.Empty(t, got.ValuedAtCost)
	assert.Equal(t, []rateRequest{{Currency: "USD", From: "2026-08-25", To: "2026-10-23"}}, srv.rateProvider.requestedRanges())
}

// dayAt returns the time on the day at 10:00 UTC (12:00 in Polish time).
func dayAt(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 10, 0, 0, 0, time.UTC)
}

// TestValueHistory_TimeWeightedReturn_SpecExample checks the example of spec #27.
// A value of 1000, then 1100, then a deposit of 500 and a value of 1650 gives 15 %:
// (1100 / 1000) × ((1650 − 500) / 1100) − 1 = 15 %.
// All periods start before the first point, so they start on the first point.
func TestValueHistory_TimeWeightedReturn_SpecExample(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000005", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Free funds interest", Time: octoberAt(23), Amount: "50.00", ID: "1604", Comment: "Free-funds Interest"},
		cashDeposit("1603", "500.00", octoberAt(23).Add(-time.Hour)),
		{Type: "Free funds interest", Time: octoberAt(22), Amount: "100.00", ID: "1602", Comment: "Free-funds Interest"},
		cashDeposit("1601", "1000.00", octoberAt(21)),
	}}, "XTB USD")

	got := srv.valueHistory(t, id)

	assert.Equal(t, []valuePoint{
		{Date: "2026-10-21", Value: "1000.00", NetDeposits: "1000.00"},
		{Date: "2026-10-22", Value: "1100.00", NetDeposits: "1000.00"},
		{Date: "2026-10-23", Value: "1650.00", NetDeposits: "1500.00"},
	}, got.Points)
	fifteen := ptr("15")
	assert.Equal(t, timeWeightedReturns{OneMonth: fifteen, ThreeMonths: fifteen, YearToDate: fifteen, OneYear: fifteen, All: fifteen},
		got.TimeWeightedReturn)
	first := ptr("2026-10-21")
	assert.Equal(t, periodStarts{OneMonth: first, ThreeMonths: first, YearToDate: first, OneYear: first, All: first},
		got.PeriodStarts, "each period starts on the first point")
}

// TestValueHistory_TimeWeightedReturn_EachPeriod checks the periods with deposits, withdrawals and Transfers.
// Today is 2026-10-23. Each result is one day with a change of the value that is not a flow:
//
//	A  2025-10-01  interest   +100 on 1000  × 1.10  All
//	B  2025-11-03  dividend   +110 on 1100  × 1.10  All, 1Y (base 2025-10-23)
//	C  2026-02-03  interest    +50 on 1000  × 1.05  All, 1Y, YTD (base 2025-12-31), after a withdrawal of 210
//	D  2026-08-04  interest   +200 on 2000  × 1.10  All, 1Y, YTD, 3M (base 2026-07-23), after a Transfer In of 950
//	E  2026-10-02  fee        −100 on 2000  × 0.95  All, 1Y, YTD, 3M, 1M (base 2026-09-23), after a Transfer Out of 200
//
//	1M:  0.95                          − 1 = −5 %
//	3M:  1.10 × 0.95                   − 1 = 4.5 %
//	YTD: 1.05 × 1.10 × 0.95            − 1 = 9.725 %
//	1Y:  1.10 × 1.05 × 1.10 × 0.95     − 1 = 20.6975 %
//	All: 1.10 × 1.10 × 1.05 × 1.10 × 0.95 − 1 = 32.76725 %
func TestValueHistory_TimeWeightedReturn_EachPeriod(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importIKZE(t,
		xtbtest.CashOperation{Type: "SEC fee", Time: dayAt(2026, 10, 2), Amount: "-100.00", ID: "1709", Comment: "SEC fee"},
		xtbtest.CashOperation{Type: "IKZE deposit", Time: dayAt(2026, 10, 1), Amount: "-200.00", ID: "1708", Comment: "Transfer out operation on account with id 57000001"},
		xtbtest.CashOperation{Type: "Free funds interest", Time: dayAt(2026, 8, 4), Amount: "200.00", ID: "1707", Comment: "Free-funds Interest"},
		transferIn("1706", "950.00", dayAt(2026, 8, 3)),
		xtbtest.CashOperation{Type: "Free funds interest", Time: dayAt(2026, 2, 3), Amount: "50.00", ID: "1705", Comment: "Free-funds Interest"},
		xtbtest.CashOperation{Type: "Withdrawal", Time: dayAt(2026, 2, 2), Amount: "-210.00", ID: "1704", Comment: "Withdrawal"},
		xtbtest.CashOperation{Type: "Dividend", Ticker: "ABC.US", Instrument: "ABC", Time: dayAt(2025, 11, 3), Amount: "110.00", ID: "1703", Comment: "ABC.US USD 0.5000/ SHR"},
		xtbtest.CashOperation{Type: "Free funds interest", Time: dayAt(2025, 10, 1), Amount: "100.00", ID: "1702", Comment: "Free-funds Interest"},
		ikzeDeposit("1701", "1000.00", dayAt(2025, 9, 1)),
	)

	got := srv.valueHistory(t, id)

	require.NotEmpty(t, got.Points)
	assert.Equal(t, valuePoint{Date: "2025-09-01", Value: "1000.00", NetDeposits: "1000.00"}, got.Points[0])
	assert.Equal(t, valuePoint{Date: "2026-10-23", Value: "1900.00", NetDeposits: "1540.00"}, got.Points[len(got.Points)-1])
	assert.Len(t, got.Points, 418, "one point for each calendar day from 2025-09-01 to 2026-10-23")
	assert.Equal(t, timeWeightedReturns{
		OneMonth: ptr("-5"), ThreeMonths: ptr("4.5"), YearToDate: ptr("9.725"), OneYear: ptr("20.6975"), All: ptr("32.76725"),
	}, got.TimeWeightedReturn)
	assert.Equal(t, periodStarts{
		OneMonth: ptr("2026-09-23"), ThreeMonths: ptr("2026-07-23"), YearToDate: ptr("2025-12-31"), OneYear: ptr("2025-10-23"),
		All: ptr("2025-09-01"),
	}, got.PeriodStarts, "each period starts on its base day")
}

// TestValueHistory_TimeWeightedReturn_SkipsTheDaysAfterAnEmptyAccount checks a day after a value of 0.
//
//	2026-10-16: interest +100 on 1000 × 1.10
//	2026-10-17: withdrawal of all 1100, the value is 0
//	2026-10-18 to 2026-10-20: no rate, the day before has a value of 0. A Deposit of 500 on 2026-10-20.
//	2026-10-21: interest +50 on 500 × 1.10
//
//	All: 1.10 × 1.10 − 1 = 21 %
func TestValueHistory_TimeWeightedReturn_SkipsTheDaysAfterAnEmptyAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000006", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Free funds interest", Time: octoberAt(21), Amount: "50.00", ID: "1805", Comment: "Free-funds Interest"},
		cashDeposit("1804", "500.00", octoberAt(20)),
		{Type: "Withdrawal", Time: octoberAt(17), Amount: "-1100.00", ID: "1803", Comment: "Withdrawal"},
		{Type: "Free funds interest", Time: octoberAt(16), Amount: "100.00", ID: "1802", Comment: "Free-funds Interest"},
		cashDeposit("1801", "1000.00", octoberAt(15)),
	}}, "XTB USD")

	got := srv.valueHistory(t, id)

	assert.Equal(t, ptr("21"), got.TimeWeightedReturn.All)
	assert.Equal(t, ptr("21"), got.TimeWeightedReturn.OneMonth)
}

func TestValueHistory_FirstCashOperationToday_HasNoTimeWeightedReturn(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000007", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("1901", "1000.00", octoberAt(23)),
	}}, "XTB USD")

	got := srv.valueHistory(t, id)

	assert.Equal(t, []valuePoint{{Date: "2026-10-23", Value: "1000.00", NetDeposits: "1000.00"}}, got.Points)
	assert.Equal(t, timeWeightedReturns{}, got.TimeWeightedReturn, "each period is null")
	today := ptr("2026-10-23")
	assert.Equal(t, periodStarts{OneMonth: today, ThreeMonths: today, YearToDate: today, OneYear: today, All: today},
		got.PeriodStarts)
}

// TestValueHistory_SoldInstrument_LeavesTheValueOnTheDayOfTheSale checks a sale with the history of a sold Instrument.
//
//	2026-10-19: 800 + ABC 2 × 101                  = 1002
//	2026-10-20: 750 + ABC 2 × 103 + XYZ 1 × 50.5   = 1006.5
//	2026-10-21: 960 (ABC sold for 210) + XYZ 51    = 1011
//	2026-10-22: 960 + XYZ 52                       = 1012, also today with the last stored price
func TestValueHistory_SoldInstrument_LeavesTheValueOnTheDayOfTheSale(t *testing.T) {
	t.Parallel()
	srv := startServerWithPrices(t, historyProvider(t))
	id := srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	require.Len(t, got.Points, 23, "one point for each day from 2026-10-01 to 2026-10-23")
	assert.Equal(t, valuePoint{Date: "2026-10-18", Value: "1000.00", NetDeposits: "1000.00"}, got.Points[17])
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-19", Value: "1002.00", NetDeposits: "1000.00"},
		{Date: "2026-10-20", Value: "1006.50", NetDeposits: "1000.00"},
		{Date: "2026-10-21", Value: "1011.00", NetDeposits: "1000.00"},
		{Date: "2026-10-22", Value: "1012.00", NetDeposits: "1000.00"},
		{Date: "2026-10-23", Value: "1012.00", NetDeposits: "1000.00"},
	}, got.Points[18:])
	assert.Equal(t, srv.summaryReturns(t, id).Value, got.Points[22].Value, "the point of today is the value in the hero")
	assert.Empty(t, got.ValuedAtCost)
}

// TestValueHistory_TimeWeightedReturn_MonthEnd checks a base day in a shorter month.
// Today is 2026-03-31. February 2026 has no 31st day, so the base day of 1M is 2026-02-28.
//
//	2026-02-28: interest +100 on 1000, not in 1M
//	2026-03-01: interest +110 on 1100 × 1.10, in 1M
func TestValueHistory_TimeWeightedReturn_MonthEnd(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.clock.set(time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC))
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000008", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Free funds interest", Time: dayAt(2026, 3, 1), Amount: "110.00", ID: "2003", Comment: "Free-funds Interest"},
		{Type: "Free funds interest", Time: dayAt(2026, 2, 28), Amount: "100.00", ID: "2002", Comment: "Free-funds Interest"},
		cashDeposit("2001", "1000.00", dayAt(2026, 2, 1)),
	}}, "XTB USD")

	got := srv.valueHistory(t, id)

	assert.Equal(t, ptr("10"), got.TimeWeightedReturn.OneMonth)
	assert.Equal(t, ptr("21"), got.TimeWeightedReturn.All)
	first := ptr("2026-02-01")
	assert.Equal(t, periodStarts{OneMonth: ptr("2026-02-28"), ThreeMonths: first, YearToDate: first, OneYear: first, All: first},
		got.PeriodStarts, "a base day before the first point is the first point")
}
