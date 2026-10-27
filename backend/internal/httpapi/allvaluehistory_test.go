package httpapi_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// allValueHistoryResponse is the body of GET /api/all-accounts/value-history.
type allValueHistoryResponse struct {
	Available          bool                `json:"available"`
	Warnings           []string            `json:"warnings"`
	Currency           string              `json:"currency"`
	Points             []valuePoint        `json:"points"`
	TimeWeightedReturn timeWeightedReturns `json:"timeWeightedReturn"`
	PeriodStarts       periodStarts        `json:"periodStarts"`
	ValuedAtCost       []valuedAtCostDTO   `json:"valuedAtCost"`
	PriceWarnings      []map[string]any    `json:"priceWarnings"`
}

// allValueHistory returns the Value History of all Accounts. It stops the test if the status is not 200.
func (s *testServer) allValueHistory(t *testing.T) allValueHistoryResponse {
	t.Helper()

	var got allValueHistoryResponse
	status := s.getJSON(t, "/api/all-accounts/value-history", &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// interest is the interest of the amount at the time, in the Account currency.
func interest(id, amount string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{Type: "Free funds interest", Time: at, Amount: amount, ID: id, Comment: "Free-funds Interest"}
}

// TestAllValueHistory_ValueAndFlows_UseTheExchangeRateOfEachDay checks the conversion of each day to PLN.
// The PLN Account starts on 2026-10-22 and adds nothing before that day.
//
//	2026-10-20: USD 1000 × 4.00                    = 4000, Net Deposits 1000 × 4.00            = 4000
//	2026-10-21: USD 1100 × 4.10                    = 4510, interest is not a flow
//	2026-10-22: USD 1600 × 4.20 + PLN 250          = 6970, Net Deposits 4000 + 500 × 4.20 + 250 = 6350
//	2026-10-23: USD 1600 × 4.30 (latest) + PLN 250 = 7130, the value in the hero
func TestAllValueHistory_ValueAndFlows_UseTheExchangeRateOfEachDay(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	for _, r := range [][2]string{{"2026-10-20", "4.00"}, {"2026-10-21", "4.10"}, {"2026-10-22", "4.20"}, {"2026-10-23", "4.30"}} {
		srv.rateProvider.set(t, "USD", r[0], r[1])
	}
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000001", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("5103", "500.00", octoberAt(22)),
		interest("5102", "100.00", octoberAt(21)),
		cashDeposit("5101", "1000.00", octoberAt(20)),
	}}, "XTB USD")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000002", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		cashDeposit("5201", "250.00", octoberAt(22)),
	}}, "XTB PLN")
	srv.refreshPrices(t)

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Empty(t, got.Warnings)
	assert.NotNil(t, got.Warnings, "the list is empty, not null")
	assert.Equal(t, "PLN", got.Currency)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-20", Value: "4000.00", NetDeposits: "4000.00"},
		{Date: "2026-10-21", Value: "4510.00", NetDeposits: "4000.00"},
		{Date: "2026-10-22", Value: "6970.00", NetDeposits: "6350.00"},
		{Date: "2026-10-23", Value: "7130.00", NetDeposits: "6350.00"},
	}, got.Points)
	hero := srv.allAccounts(t)
	assert.Equal(t, hero.Value, &got.Points[len(got.Points)-1].Value, "the point of today is the value in the hero")
	assert.Equal(t, hero.NetDeposits, &got.Points[len(got.Points)-1].NetDeposits, "the Net Deposits of today are the Net Deposits in the hero")
	assert.Empty(t, got.ValuedAtCost)
	assert.NotNil(t, got.ValuedAtCost, "the list is empty, not null")
	assert.NotNil(t, got.PriceWarnings, "the list is empty, not null")
}

// TestAllValueHistory_WeekendWithNoTable_UsesTheFridayRate checks the days with no NBP table.
// The Deposit on Saturday uses the rate of Friday too (ADR 0004).
//
//	Fri 2026-10-16: 1000 × 4.00 = 4000, Net Deposits 4000
//	Sat 2026-10-17: 1100 × 4.00 = 4400, Net Deposits 4000 + 100 × 4.00 = 4400
//	Sun 2026-10-18: 1100 × 4.00 = 4400
//	Mon 2026-10-19: 1100 × 4.20 = 4620, also to 2026-10-22
//	Fri 2026-10-23: 1100 × 4.30 = 4730
func TestAllValueHistory_WeekendWithNoTable_UsesTheFridayRate(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	for _, r := range [][2]string{{"2026-10-16", "4.00"}, {"2026-10-19", "4.20"}, {"2026-10-23", "4.30"}} {
		srv.rateProvider.set(t, "USD", r[0], r[1])
	}
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000003", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("5302", "100.00", octoberAt(17)),
		cashDeposit("5301", "1000.00", octoberAt(16)),
	}}, "XTB USD")
	srv.refreshPrices(t)

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-16", Value: "4000.00", NetDeposits: "4000.00"},
		{Date: "2026-10-17", Value: "4400.00", NetDeposits: "4400.00"},
		{Date: "2026-10-18", Value: "4400.00", NetDeposits: "4400.00"},
		{Date: "2026-10-19", Value: "4620.00", NetDeposits: "4400.00"},
		{Date: "2026-10-20", Value: "4620.00", NetDeposits: "4400.00"},
		{Date: "2026-10-21", Value: "4620.00", NetDeposits: "4400.00"},
		{Date: "2026-10-22", Value: "4620.00", NetDeposits: "4400.00"},
		{Date: "2026-10-23", Value: "4730.00", NetDeposits: "4400.00"},
	}, got.Points)
}

// TestAllValueHistory_DaysAfterTheLastRefresh_UseTheLastStoredRate checks the days after the end of the stored series.
// The refresh runs on 2026-10-23. On Sunday 2026-10-25, the tool has no newer rate.
//
//	2026-10-23: 1000 × 4.30 = 4300, also on 2026-10-24
//	2026-10-25: 1000 × 4.30 (latest) = 4300
func TestAllValueHistory_DaysAfterTheLastRefresh_UseTheLastStoredRate(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-23", "4.30")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000004", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("5401", "1000.00", octoberAt(22)),
	}}, "XTB USD")
	srv.refreshPrices(t)
	srv.clock.set(octoberAt(25))

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-22", Value: "4000.00", NetDeposits: "4000.00"},
		{Date: "2026-10-23", Value: "4300.00", NetDeposits: "4000.00"},
		{Date: "2026-10-24", Value: "4300.00", NetDeposits: "4000.00"},
		{Date: "2026-10-25", Value: "4300.00", NetDeposits: "4000.00"},
	}, got.Points)
}

// TestAllValueHistory_TimeWeightedReturn_IncludesTheExchangeRate checks a USD Account that holds only cash.
// In USD, its Time-Weighted Return is 0 %. In PLN, the rate goes from 4.00 to 4.40.
//
//	All: (4200 / 4000) × (4400 / 4200) − 1 = 10 %
func TestAllValueHistory_TimeWeightedReturn_IncludesTheExchangeRate(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	for _, r := range [][2]string{{"2026-10-21", "4.00"}, {"2026-10-22", "4.20"}, {"2026-10-23", "4.40"}} {
		srv.rateProvider.set(t, "USD", r[0], r[1])
	}
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "59000005", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("5501", "1000.00", octoberAt(21)),
	}}, "XTB USD")
	srv.refreshPrices(t)

	got := srv.allValueHistory(t)

	assert.Equal(t, []valuePoint{
		{Date: "2026-10-21", Value: "4000.00", NetDeposits: "4000.00"},
		{Date: "2026-10-22", Value: "4200.00", NetDeposits: "4000.00"},
		{Date: "2026-10-23", Value: "4400.00", NetDeposits: "4000.00"},
	}, got.Points)
	assert.Equal(t, ptr("10"), got.TimeWeightedReturn.All)
	assert.Equal(t, ptr("0"), srv.valueHistory(t, id).TimeWeightedReturn.All, "in USD, the return is 0 %")
	first := ptr("2026-10-21")
	assert.Equal(t, periodStarts{OneMonth: first, ThreeMonths: first, YearToDate: first, OneYear: first, All: first}, got.PeriodStarts)
}

// TestAllValueHistory_TransferBetweenAccounts_IsNotAFlow checks ADR 0003 for all Accounts.
// The PLN Account sends 5000 to the IKZE Account on 2026-10-21. The tool does not link the two sides.
// Their flows cancel on that day, so the Net Deposits count the money once.
//
//	2026-10-19: PLN 10000                       = 10000
//	2026-10-20: PLN 10100 (interest)            = 10100
//	2026-10-21: PLN 5100 + IKZE 5000            = 10100, flow −5000 + 5000 = 0
//	2026-10-22: PLN 5100 + IKZE 5050 (interest) = 10150, also today
//
//	All: (10100 / 10000) × (10100 / 10100) × (10150 / 10100) × 1 − 1 = 1.5 %
//
// If the Transfer In is a flow, the rate of 2026-10-21 is (10100 − 5000) / 10100 − 1, and the return is negative.
func TestAllValueHistory_TransferBetweenAccounts_IsNotAFlow(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	transferAt := octoberAt(21)
	srv.importWithWarnings(t, xtbtest.Export{AccountNumber: plnAccountNumber, Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "IKZE deposit", Time: transferAt, Amount: "-5000.00", ID: "5603", Comment: "Transfer out operation on account with id " + plnAccountNumber},
		interest("5602", "100.00", octoberAt(20)),
		cashDeposit("5601", "10000.00", octoberAt(19)),
	}}, "XTB PLN", "regular")
	srv.importWithWarnings(t, xtbtest.Export{AccountNumber: ikzeAccountNumber, Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		interest("5702", "50.00", octoberAt(22)),
		{Type: "IKZE deposit", Time: transferAt.Add(15 * time.Millisecond), Amount: "5000.00", ID: "5701", Comment: "Transfer in operation on account with id " + ikzeAccountNumber},
	}}, "XTB IKZE", "IKZE")

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-19", Value: "10000.00", NetDeposits: "10000.00"},
		{Date: "2026-10-20", Value: "10100.00", NetDeposits: "10000.00"},
		{Date: "2026-10-21", Value: "10100.00", NetDeposits: "10000.00"},
		{Date: "2026-10-22", Value: "10150.00", NetDeposits: "10000.00"},
		{Date: "2026-10-23", Value: "10150.00", NetDeposits: "10000.00"},
	}, got.Points)
	assert.Equal(t, ptr("1.5"), got.TimeWeightedReturn.All)
	assert.Equal(t, srv.allAccounts(t).NetDeposits, ptr("10000.00"), "the hero counts the money once too")
}

// TestAllValueHistory_Holdings_TodayIsTheValueInTheHero checks the point of today with Holdings on two Accounts.
// The USD Account holds ABC.US. The PLN Account holds ABC.US with the Quote Currency USD.
//
//	2026-10-22: USD (800 + 2 × 102) × 4.20 + PLN 200 + 1 × 102 × 4.20 = 4216.8 + 628.4 = 4845.2
//	2026-10-23: USD (800 + 2 × 103) × 4.25 + PLN 200 + 1 × 103 × 4.25 = 4275.5 + 637.75 = 4913.25
func TestAllValueHistory_Holdings_TodayIsTheValueInTheHero(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "ABC", "USD", "2026-10-22=102")
	provider.set(t, "ABC", "103", "USD", "2026-10-23")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.20")
	srv.rateProvider.set(t, "USD", "2026-10-23", "4.25")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000006", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC Inc", Time: octoberAt(22), Amount: "-200.00", ID: "5802", Comment: "OPEN BUY 2 @ 100.00", PositionID: "58000000001"},
		cashDeposit("5801", "1000.00", octoberAt(22).Add(-time.Hour)),
	}}, "XTB USD")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000007", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC Inc", Time: octoberAt(22), Amount: "-400.00", ID: "5902", Comment: "OPEN BUY 1 @ 100.00", PositionID: "59000000001"},
		cashDeposit("5901", "600.00", octoberAt(22).Add(-time.Hour)),
	}}, "XTB PLN")
	srv.refreshHistory(t)

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Equal(t, []valuePoint{
		{Date: "2026-10-22", Value: "4845.20", NetDeposits: "4800.00"},
		{Date: "2026-10-23", Value: "4913.25", NetDeposits: "4800.00"},
	}, got.Points)
	hero := srv.allAccounts(t)
	require.Equal(t, 0, hero.UnpricedHoldings)
	assert.Equal(t, hero.Value, &got.Points[1].Value, "the point of today is the value in the hero")
	assert.Empty(t, got.ValuedAtCost)
}

// TestAllValueHistory_ValuedAtCost_ListsTheInstrumentsOfAllAccounts checks the Instruments with no price.
// NOP.US is on both Accounts. The list has it once, in ticker order.
func TestAllValueHistory_ValuedAtCost_ListsTheInstrumentsOfAllAccounts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-21", "4.00")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000008", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ZZZ.US", Instrument: "ZZZ Inc", Time: octoberAt(22), Amount: "-50.00", ID: "6003", Comment: "OPEN BUY 1 @ 50.00", PositionID: "60000000002"},
		{Type: "Stock purchase", Ticker: "NOP.US", Instrument: "NOP Inc", Time: octoberAt(22), Amount: "-30.00", ID: "6002", Comment: "OPEN BUY 1 @ 30.00", PositionID: "60000000001"},
		cashDeposit("6001", "1000.00", octoberAt(21)),
	}}, "XTB USD")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000009", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "NOP.US", Instrument: "NOP Inc", Time: octoberAt(22), Amount: "-120.00", ID: "6102", Comment: "OPEN BUY 1 @ 30.00", PositionID: "61000000001"},
		cashDeposit("6101", "1000.00", octoberAt(21)),
	}}, "XTB PLN")
	srv.refreshPrices(t)

	got := srv.allValueHistory(t)

	assert.True(t, got.Available)
	assert.Equal(t, []valuedAtCostDTO{{Ticker: "NOP.US", Name: "NOP Inc"}, {Ticker: "ZZZ.US", Name: "ZZZ Inc"}}, got.ValuedAtCost)
}

// TestAllValueHistory_NoLatestRate_NotAvailable checks the case when the hero of all Accounts is not available.
func TestAllValueHistory_NoLatestRate_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.refreshPrices(t)

	status, _, body := srv.get(t, "/api/all-accounts/value-history")

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"available": false, "warnings": ["No Exchange Rate for USD. Refresh the prices."], "currency": "PLN", "points": [],
		"timeWeightedReturn": {"1M": null, "3M": null, "YTD": null, "1Y": null, "All": null},
		"periodStarts": {"1M": null, "3M": null, "YTD": null, "1Y": null, "All": null},
		"valuedAtCost": [], "priceWarnings": []
	}`, body)
}

// TestAllValueHistory_FlowWithNoRateOnItsDay_NotAvailable checks a flow before the start of the stored series.
// The warning is the warning of the hero.
func TestAllValueHistory_FlowWithNoRateOnItsDay_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-06-01", "4.10")
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.refreshPrices(t)
	earlier := cashDeposit("9000", "1000.00", time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	srv.confirm(t, srv.preview(t, adr0004Export(earlier)).ID, nil)

	got := srv.allValueHistory(t)

	assert.False(t, got.Available)
	assert.Equal(t, []string{"No Exchange Rate for USD on 2026-06-01. Refresh the prices."}, got.Warnings)
	assert.Empty(t, got.Points)
	assert.NotNil(t, got.Points, "the list is empty, not null")
}

// TestAllValueHistory_DayBeforeTheFirstStoredRate_NotAvailable checks a past day with no stored rate on that day
// or before it. The first Cash Operation of the USD Account is interest, which is not a flow. The series of USD
// starts 7 days before the first flow, so the days before 2026-10-13 have no rate.
func TestAllValueHistory_DayBeforeTheFirstStoredRate_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-01", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-20", "4.00")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000010", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		cashDeposit("6202", "1000.00", octoberAt(20)),
		interest("6201", "5.00", octoberAt(1)),
	}}, "XTB USD")
	srv.refreshPrices(t)
	require.True(t, srv.allAccounts(t).Available, "the hero needs only the rates of the flows")

	got := srv.allValueHistory(t)

	assert.False(t, got.Available)
	assert.Equal(t, []string{"No Exchange Rate for USD on 2026-10-01 or before it."}, got.Warnings)
	assert.Empty(t, got.Points)
}

// TestAllValueHistory_NoAccounts_NoPoints checks the body with no Accounts. The history is available and has no points.
func TestAllValueHistory_NoAccounts_NoPoints(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, _, body := srv.get(t, "/api/all-accounts/value-history")

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"available": true, "warnings": [], "currency": "PLN", "points": [],
		"timeWeightedReturn": {"1M": null, "3M": null, "YTD": null, "1Y": null, "All": null},
		"periodStarts": {"1M": null, "3M": null, "YTD": null, "1Y": null, "All": null},
		"valuedAtCost": [], "priceWarnings": []
	}`, body)
}
