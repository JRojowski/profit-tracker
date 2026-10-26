package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// summaryReturns are the return fields of GET /api/accounts/{id}/summary, with the values that they come from.
type summaryReturns struct {
	NetDeposits      string  `json:"netDeposits"`
	TotalResult      string  `json:"totalResult"`
	Value            string  `json:"value"`
	SimpleReturn     *string `json:"simpleReturn"`
	SimpleReturnNote *string `json:"simpleReturnNote"`
	AnnualReturn     *string `json:"annualReturn"`
	AnnualReturnNote *string `json:"annualReturnNote"`
}

// summaryReturns returns the return fields of the summary of the Account.
func (s *testServer) summaryReturns(t *testing.T, accountID int64) summaryReturns {
	t.Helper()

	var got summaryReturns
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// assertPercent checks that got is a percentage as decimal text, equal to want to 4 decimal places.
func assertPercent(t *testing.T, want string, got *string) {
	t.Helper()

	require.NotNil(t, got, "want %s %%, got null", want)
	d, err := decimal.NewFromString(*got)
	require.NoError(t, err, "the percentage %q is not decimal text", *got)
	assert.True(t, decimal.RequireFromString(want).Equal(d.Round(4)), "want %s %%, got %s %%", want, *got)
}

// daysBeforeToday returns the time of the test clock at the start of a test, moved back by the days.
func daysBeforeToday(days int) time.Time {
	return testToday.AddDate(0, 0, -days)
}

// interestExport is an export of a USD Account with a Deposit of 1000.00 at the time and 100.00 of interest later.
// It holds no Instrument, so the value is the cash: 1100.00.
//
//	Net Deposits:  1000.00
//	Total Result:  100.00
//	Simple Return: 100.00 × 100 / 1000.00 = 10 %
func interestExport(deposit time.Time) xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000040",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Free funds interest", Time: deposit.Add(24 * time.Hour), Amount: "100.00", ID: "702", Comment: "Free-funds Interest"},
			{Type: "Deposit", Time: deposit, Amount: "1000.00", ID: "701", Comment: "Blik deposit"},
		},
	}
}

// TestAccountSummary_DepositOneYearAgo_GivesSimpleAndAnnualReturn checks the example of spec #13:
// a deposit of 1000 and a value of 1100 exactly 365 days later give an Annual Return of 10 %.
//
//	Annual Return: 1000.00 × (1 + r)^(365 / 365) = 1100.00, so r = 10 %
func TestAccountSummary_DepositOneYearAgo_GivesSimpleAndAnnualReturn(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, interestExport(daysBeforeToday(365)), "XTB USD")

	got := srv.summaryReturns(t, id)

	assert.Equal(t, "1000.00", got.NetDeposits)
	assert.Equal(t, "100.00", got.TotalResult)
	assert.Equal(t, "1100.00", got.Value)
	assertPercent(t, "10", got.SimpleReturn)
	assert.Nil(t, got.SimpleReturnNote)
	assertPercent(t, "10", got.AnnualReturn)
	assert.Nil(t, got.AnnualReturnNote)
}

func TestAccountSummary_FirstDepositLessThanOneYearAgo_AnnualReturnNotAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		deposit time.Time
	}{
		{name: "364 days", deposit: daysBeforeToday(364)},
		// 2025-10-23 22:30 UTC is 2025-10-24 00:30 in Poland, 364 days before 2026-10-23 in Poland.
		{name: "365 days in UTC, 364 days in Polish time", deposit: time.Date(2025, 10, 23, 22, 30, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			id := srv.importAccount(t, interestExport(tc.deposit), "XTB USD")

			got := srv.summaryReturns(t, id)

			assertPercent(t, "10", got.SimpleReturn)
			assert.Nil(t, got.SimpleReturnNote)
			assert.Nil(t, got.AnnualReturn)
			assert.Equal(t, ptr("The first deposit is less than one year old."), got.AnnualReturnNote)
		})
	}
}

func TestAccountSummary_ClockMovesToOneYearAfterFirstDeposit_AnnualReturnAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, interestExport(daysBeforeToday(364)), "XTB USD")
	require.Nil(t, srv.summaryReturns(t, id).AnnualReturn)

	srv.clock.set(testToday.AddDate(0, 0, 1))

	assertPercent(t, "10", srv.summaryReturns(t, id).AnnualReturn)
}

func TestAccountSummary_NetDepositsZeroOrLess_SimpleReturnNotAvailable(t *testing.T) {
	t.Parallel()
	deposit := daysBeforeToday(400)
	tests := []struct {
		name        string
		ops         []xtbtest.CashOperation
		netDeposits string
	}{
		{
			name: "zero",
			ops: []xtbtest.CashOperation{
				{Type: "Withdrawal", Time: deposit.Add(48 * time.Hour), Amount: "-1000.00", ID: "713", Comment: "Withdrawal to bank account"},
				{Type: "Deposit", Time: deposit, Amount: "1000.00", ID: "711", Comment: "Blik deposit"},
			},
			netDeposits: "0.00",
		},
		{
			name: "negative",
			ops: []xtbtest.CashOperation{
				{Type: "Withdrawal", Time: deposit.Add(48 * time.Hour), Amount: "-1200.00", ID: "713", Comment: "Withdrawal to bank account"},
				{Type: "Free funds interest", Time: deposit.Add(24 * time.Hour), Amount: "500.00", ID: "712", Comment: "Free-funds Interest"},
				{Type: "Deposit", Time: deposit, Amount: "1000.00", ID: "711", Comment: "Blik deposit"},
			},
			netDeposits: "-200.00",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			id := srv.importAccount(t, xtbtest.Export{AccountNumber: "56000041", Currency: "USD", CashOperations: tc.ops}, "XTB USD")

			got := srv.summaryReturns(t, id)

			assert.Equal(t, tc.netDeposits, got.NetDeposits)
			assert.Nil(t, got.SimpleReturn)
			assert.Equal(t, ptr("The Net Deposits are 0 or less."), got.SimpleReturnNote)
		})
	}
}

// unpricedExport is an export of a USD Account with a Deposit of 1000.00 at the time.
// The deposit buys 6 ABC.US for 600.00. The Holding has no value until a price refresh.
//
//	Cash:          1000.00 - 600.00 = 400.00
//	Simple Return: Total Result 0.00 × 100 / 1000.00 = 0 %
func unpricedExport(deposit time.Time) xtbtest.Export {
	return xtbtest.Export{AccountNumber: "56000042", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Time: deposit.Add(time.Hour), Amount: "-600.00", ID: "722", Comment: "OPEN BUY 6 @ 100.00", PositionID: "7000000001"},
		{Type: "Deposit", Time: deposit, Amount: "1000.00", ID: "721", Comment: "Blik deposit"},
	}}
}

// TestAccountSummary_HoldingWithNoValue_AnnualReturnNotAvailable checks that a Holding with no value stops the
// Annual Return. The value leaves the Holding out, so a rate from it is wrong. Spec #13, user story 44.
// After the price refresh, the Holding has a value and the Annual Return is available.
//
//	Value after the refresh: 400.00 + 6 × 110.00 = 1060.00
//	Annual Return:           1000.00 × (1 + r)^(730 / 365) = 1060.00, so r = √1.06 − 1 = 2.9563 %
func TestAccountSummary_HoldingWithNoValue_AnnualReturnNotAvailable(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, unpricedExport(daysBeforeToday(730)), "XTB USD")

	got := srv.summaryReturns(t, id)

	assert.Equal(t, "400.00", got.Value)
	assertPercent(t, "0", got.SimpleReturn)
	assert.Nil(t, got.AnnualReturn)
	assert.Equal(t, ptr("A Holding has no value."), got.AnnualReturnNote)

	srv.refreshPrices(t)
	got = srv.summaryReturns(t, id)

	assert.Equal(t, "1060.00", got.Value)
	assertPercent(t, "2.9563", got.AnnualReturn)
	assert.Nil(t, got.AnnualReturnNote)
}

// TestAccountSummary_HoldingWithNoValueAndShortHistory_ShortHistoryNote checks the order of the reasons.
// A price refresh does not fix a short history, so its note comes first.
func TestAccountSummary_HoldingWithNoValueAndShortHistory_ShortHistoryNote(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, unpricedExport(daysBeforeToday(100)), "XTB USD")

	got := srv.summaryReturns(t, id)

	assert.Nil(t, got.AnnualReturn)
	assert.Equal(t, ptr("The first deposit is less than one year old."), got.AnnualReturnNote)
}
