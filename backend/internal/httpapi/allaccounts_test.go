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

// plnAmounts are the amounts in PLN of the All Accounts totals or of one row. An amount is nil if it is not available.
type plnAmounts struct {
	Value            *string `json:"value"`
	Cash             *string `json:"cash"`
	NetDeposits      *string `json:"netDeposits"`
	TotalResult      *string `json:"totalResult"`
	RealizedProfit   *string `json:"realizedProfit"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	NetDividends     *string `json:"netDividends"`
	NetInterest      *string `json:"netInterest"`
	Fees             *string `json:"fees"`
	CurrencyResult   *string `json:"currencyResult"`
}

// allAccountsResponse is the body of GET /api/all-accounts/summary.
type allAccountsResponse struct {
	Available bool `json:"available"`
	plnAmounts
	PriceDate        *string             `json:"priceDate"`
	ExchangeRateDate *string             `json:"exchangeRateDate"`
	StalePrices      bool                `json:"stalePrices"`
	StaleRates       bool                `json:"staleRates"`
	UnpricedHoldings int                 `json:"unpricedHoldings"`
	Warnings         []string            `json:"warnings"`
	Accounts         []allAccountsRowDTO `json:"accounts"`
}

// allAccountsRowDTO is one row of the All Accounts response.
type allAccountsRowDTO struct {
	AccountID    int64   `json:"accountId"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Currency     string  `json:"currency"`
	Available    bool    `json:"available"`
	ExchangeRate *string `json:"exchangeRate"`
	plnAmounts
	UnpricedHoldings int     `json:"unpricedHoldings"`
	Note             *string `json:"note"`
}

// allAccounts returns the All Accounts summary. It stops the test if the status is not 200.
func (s *testServer) allAccounts(t *testing.T) allAccountsResponse {
	t.Helper()

	var got allAccountsResponse
	status := s.getJSON(t, "/api/all-accounts/summary", &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// cashAmounts returns the amounts in PLN of Accounts that hold only cash, with interest as the only part
// of the Total Result. The value is the cash. The other parts are 0.
func cashAmounts(cash, netDeposits, totalResult, interest, currencyResult string) plnAmounts {
	zero := "0.00"
	return plnAmounts{
		Value: &cash, Cash: &cash, NetDeposits: &netDeposits, TotalResult: &totalResult, RealizedProfit: &zero,
		UnrealizedProfit: &zero, NetDividends: &zero, NetInterest: &interest, Fees: &zero, CurrencyResult: &currencyResult,
	}
}

// usdDeposit is a Deposit of the amount at the time.
func usdDeposit(id, amount string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{Type: "Deposit", Time: at, Amount: amount, ID: id, Comment: "Blik deposit"}
}

// adr0004Export is the USD Account of the ADR 0004 example, with the extra Cash Operations.
// A Deposit of 10000 USD on Monday 2026-10-05 and 1000 USD of interest give a value of 11000 USD.
func adr0004Export(extra ...xtbtest.CashOperation) xtbtest.Export {
	ops := append([]xtbtest.CashOperation(nil), extra...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Free funds interest", Time: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC), Amount: "1000.00", ID: "9002", Comment: "Free-funds Interest"},
		usdDeposit("9001", "10000.00", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)),
	)
	return xtbtest.Export{AccountNumber: "58000001", Currency: "USD", CashOperations: ops}
}

// TestAllAccounts_ADR0004Example checks the example of ADR 0004.
//
//	Value:            11000 × 3.60                  = 39600 PLN
//	Net Deposits:     10000 × 4.00                  = 40000 PLN
//	Parts:            1000 of interest × 3.60       =  3600 PLN
//	Currency Result:  10000 × (3.60 − 4.00)         = −4000 PLN
//	Total Result:     39600 − 40000 = 3600 − 4000   =  −400 PLN
func TestAllAccounts_ADR0004Example(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	id := srv.importAccount(t, adr0004Export(), "XTB USD")

	srv.refreshPrices(t)
	got := srv.allAccounts(t)

	want := cashAmounts("39600.00", "40000.00", "-400.00", "3600.00", "-4000.00")
	assert.Equal(t, allAccountsResponse{
		Available: true, plnAmounts: want, ExchangeRateDate: ptr("2026-10-22"), Warnings: []string{},
		Accounts: []allAccountsRowDTO{{
			AccountID: id, Name: "XTB USD", Type: "regular", Currency: "USD", Available: true,
			ExchangeRate: ptr("3.6"), plnAmounts: want,
		}},
	}, got)
	assert.Equal(t, []rateRequest{{Currency: "USD", From: "2026-09-28", To: "2026-10-23"}}, srv.rateProvider.requestedRanges(),
		"the series of an Account currency starts 7 days before the earliest Net Deposits flow")
}

func TestAllAccounts_FlowOnSaturday_UsesTheRateOfTheFridayBefore(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-09", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-12", "4.50")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	// Saturday 2026-10-10, 12:00 in Polish time.
	srv.importAccount(t, xtbtest.Export{AccountNumber: "58000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		usdDeposit("9011", "1000.00", time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)),
	}}, "XTB USD")

	srv.refreshPrices(t)
	got := srv.allAccounts(t)

	assert.Equal(t, ptr("4000.00"), got.NetDeposits, "1000 × 4.00, the rate of Friday 2026-10-09")
	assert.Equal(t, ptr("-400.00"), got.CurrencyResult, "1000 × (3.60 − 4.00)")
}

func TestAllAccounts_TransfersBetweenPLNAccounts_CancelInTheNetDeposits(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	plnID, _ := srv.importWithWarnings(t, adr0003PLNExport(), "XTB PLN", "regular")
	ikzeID, _ := srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")

	got := srv.allAccounts(t)

	zero := cashAmounts("0.00", "0.00", "0.00", "0.00", "0.00")
	ikze := cashAmounts("10000.00", "10000.00", "0.00", "0.00", "0.00")
	assert.Equal(t, allAccountsResponse{
		Available: true, plnAmounts: ikze, Warnings: []string{},
		Accounts: []allAccountsRowDTO{
			{AccountID: ikzeID, Name: "XTB IKZE", Type: "IKZE", Currency: "PLN", Available: true, plnAmounts: ikze},
			{AccountID: plnID, Name: "XTB PLN", Type: "regular", Currency: "PLN", Available: true, plnAmounts: zero},
		},
	}, got)
	assert.Empty(t, srv.rateProvider.requestedRanges(), "PLN Accounts need no Exchange Rate")
}

// TestAllAccounts_EarlierFlowAfterRefresh_RateMissingUntilTheSeriesExtends checks a flow on a day outside
// the recorded range. A new Import adds a Deposit before the start of the USD series. Until the next refresh,
// the totals and the USD row are not available. The PLN row stays available.
// The refresh asks only for the missing days before the series, and then the totals are available.
func TestAllAccounts_EarlierFlowAfterRefresh_RateMissingUntilTheSeriesExtends(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-06-01", "4.10")
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	plnID := srv.importAccount(t, depositOnlyExport("58000003", "PLN"), "XTB PLN")
	usdID := srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.refreshPrices(t)
	require.True(t, srv.allAccounts(t).Available)

	earlier := usdDeposit("9000", "1000.00", time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC))
	srv.confirm(t, srv.preview(t, adr0004Export(earlier)).ID, nil)
	got := srv.allAccounts(t)

	note := "No Exchange Rate for USD on 2026-06-01. Refresh the prices."
	pln := cashAmounts("250.00", "250.00", "0.00", "0.00", "0.00")
	assert.Equal(t, allAccountsResponse{
		Available: false, ExchangeRateDate: ptr("2026-10-22"), Warnings: []string{note},
		Accounts: []allAccountsRowDTO{
			{AccountID: plnID, Name: "XTB PLN", Type: "regular", Currency: "PLN", Available: true, plnAmounts: pln},
			{AccountID: usdID, Name: "XTB USD", Type: "regular", Currency: "USD", Available: false, ExchangeRate: ptr("3.6"), Note: &note},
		},
	}, got)

	_, ran, err := srv.rates.RefreshIfStale(t.Context())
	require.NoError(t, err)
	got = srv.allAccounts(t)

	assert.True(t, ran, "the series of USD starts after the day that the earliest flow needs")
	assert.Equal(t, []rateRequest{
		{Currency: "USD", From: "2026-09-28", To: "2026-10-23"},
		{Currency: "USD", From: "2026-05-25", To: "2026-09-27"},
	}, srv.rateProvider.requestedRanges(), "the refresh asks only for the missing days")
	// Net Deposits: 250 + 1000 × 4.10 + 10000 × 4.00 = 44350. Value: 250 + 12000 × 3.60 = 43450.
	// Currency Result: 1000 × (3.60 − 4.10) + 10000 × (3.60 − 4.00) = −4500. Parts: 1000 × 3.60 = 3600.
	assert.True(t, got.Available)
	assert.Empty(t, got.Warnings)
	assert.Equal(t, cashAmounts("43450.00", "44350.00", "-900.00", "3600.00", "-4500.00"), got.plnAmounts)
}

func TestAllAccounts_NoLatestRate_RowNotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, adr0004Export(), "XTB USD")

	srv.refreshPrices(t)
	got := srv.allAccounts(t)

	note := "No Exchange Rate for USD. Refresh the prices."
	assert.False(t, got.Available)
	assert.Equal(t, plnAmounts{}, got.plnAmounts)
	assert.Equal(t, []string{note}, got.Warnings)
	require.Len(t, got.Accounts, 1)
	assert.False(t, got.Accounts[0].Available)
	assert.Nil(t, got.Accounts[0].ExchangeRate)
	assert.Equal(t, &note, got.Accounts[0].Note)
}

// TestAllAccounts_RateOfAccountCurrencyFetchedBeforeToday_StaleRates checks the stale-rate flag for the rate of
// an Account currency. The USD Account holds no Instrument, so its own summary has no stale rate.
func TestAllAccounts_RateOfAccountCurrencyFetchedBeforeToday_StaleRates(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.refreshPrices(t)

	// 00:30 on 2026-10-24 in Polish time.
	srv.clock.set(time.Date(2026, 10, 23, 22, 30, 0, 0, time.UTC))
	got := srv.allAccounts(t)

	assert.True(t, got.Available, "the tool keeps the last stored rates")
	assert.True(t, got.StaleRates)
	assert.False(t, got.StalePrices, "no Holding needs a price")
	assert.Empty(t, got.Warnings, "the stale-rate flag tells the user, not a warning")
}

// TestAllAccounts_HoldingsOfAllAccounts_PriceDateAndUnpricedHoldings checks the price status of all Accounts.
// The PLN Account holds ABC.US with a price. The USD Account holds XYZ.US with no price.
//
//	PLN Account: value 200.00 + 125.50 × 2 × 3.6620 = 1119.162 PLN, Unrealized Profit 119.162 PLN
//	USD Account: value 1000.00 − 60.00 = 940.00 USD × 3.6620 = 3442.28 PLN, Net Deposits 1000.00 USD × 3.6620
func TestAllAccounts_HoldingsOfAllAccounts_PriceDateAndUnpricedHoldings(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-21")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2025-12-29", "3.6620")
	srv.importAccount(t, plnExport(), "XTB PLN")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "58000004", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Amount: "-60.00", ID: "9022", Comment: "OPEN BUY 0.5 @ 120.00", PositionID: "9000000002"},
		usdDeposit("9021", "1000.00", time.Time{}),
	}}, "XTB USD")

	srv.refreshPrices(t)
	got := srv.allAccounts(t)

	assert.True(t, got.Available)
	assert.Equal(t, ptr("2026-10-21"), got.PriceDate)
	assert.Equal(t, ptr("2025-12-29"), got.ExchangeRateDate)
	assert.Equal(t, 1, got.UnpricedHoldings)
	require.Len(t, got.Accounts, 2)
	assert.Equal(t, 0, got.Accounts[0].UnpricedHoldings)
	assert.Equal(t, 1, got.Accounts[1].UnpricedHoldings)
	assert.Equal(t, ptr("4561.442"), got.Value, "1119.162 + 3442.28")
	assert.True(t, got.StalePrices, "XYZ.US on the USD Account has no price")
	assert.False(t, got.StaleRates)
}

func TestAllAccounts_NoAccounts_ZeroTotalsAndNoRows(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, _, body := srv.get(t, "/api/all-accounts/summary")

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"available": true, "value": "0.00", "cash": "0.00", "netDeposits": "0.00", "totalResult": "0.00",
		"realizedProfit": "0.00", "unrealizedProfit": "0.00", "netDividends": "0.00", "netInterest": "0.00",
		"fees": "0.00", "currencyResult": "0.00", "priceDate": null, "exchangeRateDate": null,
		"stalePrices": false, "staleRates": false, "unpricedHoldings": 0, "warnings": [], "accounts": []
	}`, body)
}
