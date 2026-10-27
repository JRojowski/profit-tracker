package httpapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// plnExport is an export of a PLN Account with one Holding quoted in USD, and the extra purchases.
//
//	Cash:    1000.00 - 800.00 = 200.00, minus the extra purchases
//	ABC.US:  volume 2, cost 800.00 PLN
func plnExport(extra ...xtbtest.CashOperation) xtbtest.Export {
	ops := append([]xtbtest.CashOperation(nil), extra...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Amount: "-800.00", ID: "1002", Comment: "OPEN BUY 2 @ 100.00", PositionID: "10000000001"},
		xtbtest.CashOperation{Type: "Deposit", Amount: "1000.00", ID: "1001", Comment: "Blik deposit"},
	)
	return xtbtest.Export{AccountNumber: "56000030", Currency: "PLN", CashOperations: ops}
}

// TestRefreshPrices_USDHoldingOnPLNAccount_ValueAtLatestExchangeRate checks the conversion with the latest rate.
//
//	ABC.US: value 125.50 × 2 × 3.6620 = 919.162 PLN, Unrealized Profit 919.162 - 800.00 = 119.162 PLN,
//	        119.162 × 100 / 800.00 = 14.89525 %
//	Value:  200.00 + 919.162 = 1119.162 PLN
func TestRefreshPrices_USDHoldingOnPLNAccount_ValueAtLatestExchangeRate(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-21", "3.6512")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	id := srv.importAccount(t, plnExport(), "XTB PLN")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 1, Failures: []refreshFailure{}, RateFailures: []rateFailure{}}, refresh)
	assert.Equal(t, []holdingPrices{{
		Ticker: "ABC.US", QuoteCurrency: ptr("USD"), Price: ptr("125.5"), PriceDate: ptr("2026-10-22"),
		ExchangeRate: ptr("3.662"), ExchangeRateCurrency: ptr("USD"), ExchangeRateDate: ptr("2026-10-22"),
		Value: ptr("919.162"), UnrealizedProfit: ptr("119.162"), UnrealizedProfitPercent: ptr("14.89525"),
	}}, srv.holdingPrices(t, id))
	assert.Equal(t, summaryPrices{
		Cash: "200.00", Value: "1119.162", UnrealizedProfit: "119.162", TotalResult: "119.162",
		PriceDate: ptr("2026-10-22"), ExchangeRateDate: ptr("2026-10-22"), StalePrices: false, StaleRates: false,
		UnpricedHoldings: 0,
	}, srv.summaryPrices(t, id))
}

// defPurchase is a purchase of 1 DEF.DE, quoted in EUR, for 200.00 in the Account currency.
var defPurchase = xtbtest.CashOperation{
	Type: "Stock purchase", Ticker: "DEF.DE", Instrument: "DEF", Amount: "-200.00", ID: "1003",
	Comment: "OPEN BUY 1 @ 45.00", PositionID: "10000000002",
}

// TestRefreshPrices_PLNHoldingOnUSDAccount_ValueDividedByExchangeRate checks the conversion from PLN.
// The USD Account holds no Instrument quoted in USD, but it needs the USD rate.
//
//	PKO.PL: value 50.00 × 2 / 3.20 = 31.25 USD, Unrealized Profit 31.25 - 20.00 = 11.25 USD,
//	        11.25 × 100 / 20.00 = 56.25 %
//	Value:  80.00 + 31.25 = 111.25 USD
func TestRefreshPrices_PLNHoldingOnUSDAccount_ValueDividedByExchangeRate(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "PKO.WA", "50.00", "PLN", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.20")
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "56000031", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "PKO.PL", Instrument: "PKO", Amount: "-20.00", ID: "1102", Comment: "OPEN BUY 2 @ 40.00", PositionID: "11000000001"},
		{Type: "Deposit", Amount: "100.00", ID: "1101", Comment: "Blik deposit"},
	}}, "XTB USD")
	srv.refreshPrices(t)

	assert.Equal(t, []holdingPrices{{
		Ticker: "PKO.PL", QuoteCurrency: ptr("PLN"), Price: ptr("50"), PriceDate: ptr("2026-10-22"),
		ExchangeRate: ptr("3.2"), ExchangeRateCurrency: ptr("USD"), ExchangeRateDate: ptr("2026-10-22"),
		Value: ptr("31.25"), UnrealizedProfit: ptr("11.25"), UnrealizedProfitPercent: ptr("56.25"),
	}}, srv.holdingPrices(t, id))
	assert.Equal(t, summaryPrices{
		Cash: "80.00", Value: "111.25", UnrealizedProfit: "11.25", TotalResult: "11.25",
		PriceDate: ptr("2026-10-22"), ExchangeRateDate: ptr("2026-10-22"), UnpricedHoldings: 0,
	}, srv.summaryPrices(t, id))
	// The Deposit has the default time 2026-01-02, so the series of the Account currency starts 7 days before it.
	assert.Equal(t, []rateRequest{{Currency: "USD", From: "2025-12-26", To: "2026-10-23"}}, srv.rateProvider.requestedRanges(),
		"the USD Account holds an Instrument quoted in PLN, so the tool needs the USD rate")
}

// TestRefreshPrices_OneCurrencyFails_OtherCurrenciesGetRates checks a currency with no rate.
// The EUR Holding has a price but no rate, so it has no value, and the rates are stale.
func TestRefreshPrices_OneCurrencyFails_OtherCurrenciesGetRates(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "DEF.DE", "45.00", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	srv.rateProvider.fail("EUR", errors.New("the NBP does not respond"))
	id := srv.importAccount(t, plnExport(defPurchase), "XTB PLN")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 2, Failures: []refreshFailure{}, RateFailures: []rateFailure{
		{Currency: "EUR", Error: "the NBP does not respond"},
	}}, refresh)
	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 2)
	assert.Equal(t, ptr("919.162"), holdings[0].Value)
	assert.Equal(t, noValue("DEF.DE", ptr("EUR"), "There is no Exchange Rate for EUR. Refresh the prices."), holdings[1])
	assert.Equal(t, summaryPrices{
		Cash: "0.00", Value: "919.162", UnrealizedProfit: "119.162", TotalResult: "119.162",
		PriceDate: ptr("2026-10-22"), ExchangeRateDate: ptr("2026-10-22"), StalePrices: false, StaleRates: true,
		UnpricedHoldings: 1,
	}, srv.summaryPrices(t, id))

	// The next day the NBP answers. The tool recorded no days for EUR, so the series starts again 7 days before
	// the first Fill of DEF.DE. The Fills have the default time 2026-01-02.
	// DEF.DE: value 45.00 × 1 × 4.25 = 191.25 PLN.
	srv.clock.set(testToday.AddDate(0, 0, 1))
	srv.rateProvider.fail("EUR", nil)
	srv.rateProvider.set(t, "EUR", "2026-10-23", "4.25")

	refresh = srv.refreshPrices(t)

	assert.Empty(t, refresh.RateFailures)
	assert.Equal(t, []rateRequest{
		{Currency: "EUR", From: "2025-12-26", To: "2026-10-23"},
		{Currency: "USD", From: "2025-12-26", To: "2026-10-23"},
		{Currency: "EUR", From: "2025-12-26", To: "2026-10-24"},
		{Currency: "USD", From: "2026-10-23", To: "2026-10-24"},
	}, srv.rateProvider.requestedRanges())
	assert.Equal(t, ptr("191.25"), srv.holdingPrices(t, id)[1].Value)
	assert.Equal(t, ptr("2026-10-22"), srv.summaryPrices(t, id).ExchangeRateDate, "the oldest of the used rates")
}

func TestRefreshPrices_ExchangeRates_AsksOnlyForMissingDays(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, plnExport(), "XTB PLN")

	// Friday 2026-10-23, 11:30 in Polish time. A Quote Currency starts 7 days before the first Fill of its
	// Instrument. The Fill has the default time 2026-01-02.
	srv.refreshPrices(t)
	assert.Equal(t, []rateRequest{{Currency: "USD", From: "2025-12-26", To: "2026-10-23"}}, srv.rateProvider.requestedRanges())

	// Later on the same day, the refresh asks for nothing.
	srv.clock.set(time.Date(2026, 10, 23, 21, 59, 0, 0, time.UTC))
	srv.refreshPrices(t)
	assert.Len(t, srv.rateProvider.requestedRanges(), 1, "a second refresh on the same day asks for nothing")

	// Monday. The last asked day comes again, because a morning fetch does not get the table of that day.
	// A new Holding in EUR starts a new series 7 days before its first Fill.
	srv.clock.set(time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC))
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "56000032", Currency: "PLN", CashOperations: []xtbtest.CashOperation{defPurchase},
	}, "XTB IKZE")
	srv.refreshPrices(t)
	assert.Equal(t, []rateRequest{
		{Currency: "USD", From: "2025-12-26", To: "2026-10-23"},
		{Currency: "EUR", From: "2025-12-26", To: "2026-10-26"},
		{Currency: "USD", From: "2026-10-23", To: "2026-10-26"},
	}, srv.rateProvider.requestedRanges())
}

func TestRefreshPrices_ExchangeRates_NoHoldingInOtherCurrency_AsksForNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, xtbtest.Export{AccountNumber: "56000033", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "PKO.PL", Instrument: "PKO", Amount: "-20.00", ID: "1202", Comment: "OPEN BUY 2 @ 10.00", PositionID: "12000000001"},
		{Type: "Deposit", Amount: "100.00", ID: "1201", Comment: "Blik deposit"},
	}}, "XTB PLN")

	refresh := srv.refreshPrices(t)
	_, ran, err := srv.rates.RefreshIfStale(t.Context())

	require.NoError(t, err)
	assert.Empty(t, refresh.RateFailures)
	assert.False(t, ran, "the tool needs no rate for a Holding in PLN")
	assert.Empty(t, srv.rateProvider.requestedRanges())
}

func TestRefreshAtStart_ExchangeRates_RunsOnlyWhenTheLastFetchIsBeforeTodayInPolishTime(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	srv.importAccount(t, plnExport(), "XTB PLN")

	_, ran, err := srv.rates.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.True(t, ran, "the tool never asked for USD")

	// 23:59 on 2026-10-23 in Polish time (UTC+2).
	srv.clock.set(time.Date(2026, 10, 23, 21, 59, 0, 0, time.UTC))
	_, ran, err = srv.rates.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.False(t, ran, "the last fetch is today")

	// 00:01 on 2026-10-24 in Polish time, but still 2026-10-23 in UTC.
	srv.clock.set(time.Date(2026, 10, 23, 22, 1, 0, 0, time.UTC))
	result, ran, err := srv.rates.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.True(t, ran, "the last fetch is yesterday in Polish time")
	assert.Empty(t, result.Failures)
	assert.Equal(t, []rateRequest{
		{Currency: "USD", From: "2025-12-26", To: "2026-10-23"},
		{Currency: "USD", From: "2026-10-23", To: "2026-10-24"},
	}, srv.rateProvider.requestedRanges())
}

func TestRefreshPrices_RateProviderFailsNextDay_KeepsLastRateWithStaleWarning(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-21", "3.6512")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	id := srv.importAccount(t, plnExport(), "XTB PLN")
	srv.refreshPrices(t)
	srv.clock.set(time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC))
	srv.rateProvider.set(t, "USD", "2026-10-23", "3.7000")
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 1, Failures: []refreshFailure{}, RateFailures: []rateFailure{
		{Currency: "USD", Error: "the NBP does not respond"},
	}}, refresh)
	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 1)
	assert.Equal(t, ptr("3.662"), holdings[0].ExchangeRate, "the tool uses the last stored rate")
	assert.Equal(t, ptr("2026-10-22"), holdings[0].ExchangeRateDate)
	assert.Equal(t, ptr("919.162"), holdings[0].Value)
	summary := srv.summaryPrices(t, id)
	assert.True(t, summary.StaleRates)
	assert.False(t, summary.StalePrices, "the prices are from today")
	assert.Equal(t, ptr("2026-10-22"), summary.ExchangeRateDate)

	// The failed days are not recorded, so the next refresh asks for them again.
	srv.rateProvider.fail("USD", nil)
	refresh = srv.refreshPrices(t)
	assert.Empty(t, refresh.RateFailures)
	assert.Equal(t, []rateRequest{
		{Currency: "USD", From: "2025-12-26", To: "2026-10-23"},
		{Currency: "USD", From: "2026-10-23", To: "2026-10-26"},
		{Currency: "USD", From: "2026-10-23", To: "2026-10-26"},
	}, srv.rateProvider.requestedRanges())
	assert.Equal(t, ptr("2026-10-23"), srv.summaryPrices(t, id).ExchangeRateDate)
	assert.False(t, srv.summaryPrices(t, id).StaleRates)
}

func TestAccountSummary_USDHoldingOnPLNAccountNextDay_PricesAndRatesAreStale(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	id := srv.importAccount(t, plnExport(), "XTB PLN")
	srv.refreshPrices(t)
	summary := srv.summaryPrices(t, id)
	require.False(t, summary.StalePrices)
	require.False(t, summary.StaleRates)

	srv.clock.set(testToday.AddDate(0, 0, 1))
	summary = srv.summaryPrices(t, id)

	assert.True(t, summary.StalePrices, "the price of the USD Holding was fetched yesterday")
	assert.True(t, summary.StaleRates, "the USD rate was fetched yesterday")
	assert.Equal(t, "1119.162", summary.Value, "the tool keeps the last price and rate")
}

func TestAccountSummary_USDHoldingOnPLNAccountWithNoPrice_PricesAreStale(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.6620")
	id := srv.importAccount(t, plnExport(), "XTB PLN")

	srv.refreshPrices(t)

	assert.Equal(t, summaryPrices{
		Cash: "200.00", Value: "200.00", UnrealizedProfit: "0.00", TotalResult: "0.00",
		StalePrices: true, StaleRates: false, UnpricedHoldings: 1,
	}, srv.summaryPrices(t, id))
	assert.Equal(t, []holdingPrices{noValue("ABC.US", ptr("USD"), noPriceNote)}, srv.holdingPrices(t, id))
}

func TestRefreshExchangeRates_ContextEnded_AsksForNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, plnExport(), "XTB PLN")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := srv.rates.Refresh(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, srv.rateProvider.requestedRanges())
}

func TestRefreshPrices_CurrencyWithNoTable_NamesTheCurrencyInEachRefresh(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, plnExport(), "XTB PLN")
	// The NBP table A has no CHF in the fake, so the NBP answers with no rate and no error.
	srv.setQuoteCurrency(t, "ABC.US", "CHF")
	noRate := []rateFailure{{Currency: "CHF", Error: "the NBP has no Exchange Rate for CHF"}}

	first := srv.refreshPrices(t)
	second := srv.refreshPrices(t)

	assert.Equal(t, noRate, first.RateFailures)
	assert.Equal(t, noRate, second.RateFailures, "a second refresh on the same day asks for nothing, but still names CHF")
	assert.Len(t, srv.rateProvider.requestedRanges(), 1)
	assert.True(t, srv.summaryPrices(t, id).StaleRates)
}
