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

// allTiersBreakdownResponse is the body of GET /api/all-accounts/tiers-breakdown.
type allTiersBreakdownResponse struct {
	Available bool     `json:"available"`
	Warnings  []string `json:"warnings"`
	tiersBreakdownResponse
}

// allTiersBreakdown returns the Tiers Breakdown of all Accounts. It stops the test if the status is not 200.
func (s *testServer) allTiersBreakdown(t *testing.T) allTiersBreakdownResponse {
	t.Helper()

	var got allTiersBreakdownResponse
	status := s.getJSON(t, "/api/all-accounts/tiers-breakdown", &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// octoberTime returns the time on day n of October 2026, at 9:mm UTC.
func octoberTime(day, minute int) time.Time {
	return time.Date(2026, 10, day, 9, minute, 0, 0, time.UTC)
}

// tierBuy is a purchase of 1 unit in one Fill that is a full Lot.
func tierBuy(id, ticker, amount string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{
		Type: "Stock purchase", Ticker: ticker, Instrument: ticker[:3], Amount: "-" + amount, Time: at, ID: id,
		Comment: "OPEN BUY 1 @ 1", PositionID: id,
	}
}

// tierSell closes 1 unit of the Lot position, which has the Lot volume lotVolume.
func tierSell(id, ticker, amount, lotVolume, position string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{
		Type: "Stock sell", Ticker: ticker, Instrument: ticker[:3], Amount: amount, Time: at, ID: id,
		Comment: "CLOSE BUY 1/" + lotVolume + " @ 1", PositionID: position,
	}
}

// adr0010Export is the USD Account of the ADR 0010 example. AMZN.US has two Lots of 1 unit, each bought
// for 100.00 USD on 2026-10-05. Lot 3001 stays open. Lot 3002 is sold for 110.00 USD on 2026-10-07.
func adr0010Export() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "58100001", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("3000", "1000.00", octoberTime(5, 0)),
			tierBuy("3001", "AMZN.US", "100.00", octoberTime(5, 1)),
			tierBuy("3002", "AMZN.US", "100.00", octoberTime(5, 2)),
			tierSell("3003", "AMZN.US", "110.00", "1", "3002", octoberTime(7, 0)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "AMZN.US", Volume: "1", OpenTime: octoberTime(5, 2), CloseTime: octoberTime(7, 0), ProfitLoss: "10.00", PositionID: "3002"},
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AMZN.US", PositionID: "3001", Volume: "1"}},
	}
}

// TestAllTiersBreakdown_ADR0010Example checks the example of ADR 0010.
//
//	Cost:     100 × 4.00               = 400 PLN
//	Open:     110 × 3.60 − 400         = −4 PLN, Return on Cost −1
//	Sold:     110 × 3.80 − 400         = 18 PLN, Return on Cost 4.5
func TestAllTiersBreakdown_ADR0010Example(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "AMZN", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-07", "3.80")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, adr0010Export(), "XTB USD")
	srv.refreshPrices(t)

	got := srv.allTiersBreakdown(t)

	require.True(t, got.Available, "warnings: %v", got.Warnings)
	assert.Equal(t, "PLN", got.Currency)
	require.NotNil(t, got.NoTier)
	require.Len(t, got.NoTier.Instruments, 1)
	amzn := got.NoTier.Instruments[0]
	assert.Equal(t, &breakdownHoldingDTO{
		Value: ptr("396.00"), Cost: "400.00", UnrealizedProfit: ptr("-4.00"), ReturnOnCost: ptr("-1"), LastPurchaseDate: "2026-10-05",
	}, amzn.Holding)
	require.NotNil(t, amzn.ClosedPositions)
	assert.Equal(t, "400.00", amzn.ClosedPositions.Cost)
	assert.Equal(t, "418.00", amzn.ClosedPositions.SaleAmount)
	assert.Equal(t, "18.00", amzn.ClosedPositions.RealizedProfit)
	assert.Equal(t, ptr("4.5"), amzn.ClosedPositions.ReturnOnCost)
}

// bothAccountsExports are a PLN Account and a USD Account. Each has AAA.US: one open Lot of 1 unit and one sold Lot.
//
//	PLN: Lot 4001 bought for 400.00 PLN. Lot 4002 bought for 300.00 PLN, sold for 330.00 PLN on 2026-10-08.
//	USD: Lot 5001 bought for 100.00 USD on 2026-10-05. Lot 5002 bought for 80.00 USD on 2026-10-05,
//	     sold for 90.00 USD on 2026-10-07.
func bothAccountsExports() (pln, usd xtbtest.Export) {
	pln = xtbtest.Export{
		AccountNumber: "58200001", Currency: "PLN",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("4000", "1000.00", octoberTime(5, 0)),
			tierBuy("4001", "AAA.US", "400.00", octoberTime(5, 1)),
			tierBuy("4002", "AAA.US", "300.00", octoberTime(5, 2)),
			tierSell("4003", "AAA.US", "330.00", "1", "4002", octoberTime(8, 0)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "AAA.US", Volume: "1", OpenTime: octoberTime(5, 2), CloseTime: octoberTime(8, 0), ProfitLoss: "30.00", PositionID: "4002"},
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AAA.US", PositionID: "4001", Volume: "1"}},
	}
	usd = xtbtest.Export{
		AccountNumber: "58200002", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("5000", "1000.00", octoberTime(5, 0)),
			tierBuy("5001", "AAA.US", "100.00", octoberTime(5, 1)),
			tierBuy("5002", "AAA.US", "80.00", octoberTime(5, 2)),
			tierSell("5003", "AAA.US", "90.00", "1", "5002", octoberTime(7, 0)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "AAA.US", Volume: "1", OpenTime: octoberTime(5, 2), CloseTime: octoberTime(7, 0), ProfitLoss: "10.00", PositionID: "5002"},
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AAA.US", PositionID: "5001", Volume: "1"}},
	}
	return pln, usd
}

// TestAllTiersBreakdown_InstrumentOnBothAccounts_GivesOneRowInPLN checks the merge of the two Accounts. The price of
// AAA.US is 110.00 USD. The USD rate is 4.00 on 2026-10-05, 3.80 on 2026-10-07 and 3.60 latest.
//
//	Holding:  value 110 × 3.60 (PLN Account) + 110 × 3.60 (USD Account) = 792, cost 400 + 100 × 4.00 = 800
//	Closed:   cost 300 + 80 × 4.00 = 620, sale amount 330 + 90 × 3.80 = 672, Realized Profit 52
//	Tier:     total profit −8 + 52 = 44, cost 800 + 620 = 1420
func TestAllTiersBreakdown_InstrumentOnBothAccounts_GivesOneRowInPLN(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "AAA", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-07", "3.80")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	plnExport, usdExport := bothAccountsExports()
	plnID := srv.importAccount(t, plnExport, "XTB PLN")
	usdID := srv.importAccount(t, usdExport, "XTB USD")
	srv.refreshPrices(t)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "I. Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
	}})

	got := srv.allTiersBreakdown(t)

	require.True(t, got.Available, "warnings: %v", got.Warnings)
	assert.Nil(t, got.NoTier)
	require.Len(t, got.Tiers, 1)
	require.Len(t, got.Tiers[0].Instruments, 1, "one row for the Instrument on both Accounts")
	aaa := got.Tiers[0].Instruments[0]
	assert.Equal(t, &breakdownHoldingDTO{
		Value: ptr("792.00"), Cost: "800.00", UnrealizedProfit: ptr("-8.00"), ReturnOnCost: ptr("-1"), LastPurchaseDate: "2026-10-05",
	}, aaa.Holding)
	closed := aaa.ClosedPositions
	require.NotNil(t, closed)
	assert.Equal(t, 2, closed.Count)
	assert.Equal(t, "620.00", closed.Cost)
	assert.Equal(t, "672.00", closed.SaleAmount)
	assert.Equal(t, "52.00", closed.RealizedProfit)
	assert.Equal(t, "2026-10-08", closed.LastSaleDate)
	require.Len(t, closed.Positions, 2)
	usdSale, plnSale := closed.Positions[0], closed.Positions[1]
	assert.Equal(t, &breakdownAccountDTO{ID: usdID, Name: "XTB USD", Currency: "USD"}, usdSale.Account)
	assert.Equal(t, []string{"342.00", "320.00", "22.00"}, []string{usdSale.Amount, usdSale.Cost, usdSale.RealizedProfit})
	assert.Equal(t, &breakdownAccountDTO{ID: plnID, Name: "XTB PLN", Currency: "PLN"}, plnSale.Account)
	assert.Equal(t, []string{"330.00", "300.00", "30.00"}, []string{plnSale.Amount, plnSale.Cost, plnSale.RealizedProfit},
		"the amounts of the PLN Account are not converted")
	assert.Equal(t, breakdownSumDTO{
		Value: "792.00", UnrealizedProfit: "-8.00", RealizedProfit: "52.00", TotalProfit: "44.00", Cost: "1420.00",
		ReturnOnCost: ptr("3.0985915492957746"),
	}, got.Total)
}

// TestAllTiersBreakdown_LotWithFillsOnTwoDays_UsesTheAverageRateWeightedByFillAmount checks ADR 0010, rule 4.
// One order of 2 units of AAA.US gives Lot 6001 with two Fills. The price is 110.00 USD.
//
//	Fill 1: 2026-10-05, 50.00 USD, rate 4.00. Fill 2: 2026-10-06, 150.00 USD, rate 4.40.
//	Average rate: (50 × 4.00 + 150 × 4.40) / 200 = 4.30
//	Sold 1 unit for 120.00 USD on 2026-10-07, rate 3.80: cost 100 × 4.30 = 430, sale amount 456, profit 26
//	Open 1 unit: cost 100 × 4.30 = 430, value 110 × 3.60 = 396, profit −34
func TestAllTiersBreakdown_LotWithFillsOnTwoDays_UsesTheAverageRateWeightedByFillAmount(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "AAA", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-06", "4.40")
	srv.rateProvider.set(t, "USD", "2026-10-07", "3.80")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	fill := func(id, amount, volume string, at time.Time) xtbtest.CashOperation {
		return xtbtest.CashOperation{
			Type: "Stock purchase", Ticker: "AAA.US", Instrument: "AAA", Amount: "-" + amount, Time: at, ID: id,
			Comment: "OPEN BUY " + volume + "/2 @ 100", PositionID: "6001",
		}
	}
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "58300001", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("6000", "1000.00", octoberTime(5, 0)),
			fill("6001", "50.00", "0.5", octoberTime(5, 1)),
			fill("6002", "150.00", "1.5", octoberTime(6, 1)),
			tierSell("6003", "AAA.US", "120.00", "2", "6001", octoberTime(7, 0)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "AAA.US", Volume: "1", OpenTime: octoberTime(5, 1), CloseTime: octoberTime(7, 0), ProfitLoss: "20.00", PositionID: "6001"},
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AAA.US", PositionID: "6001", Volume: "1"}},
	}, "XTB USD")
	srv.refreshPrices(t)

	got := srv.allTiersBreakdown(t)

	require.True(t, got.Available, "warnings: %v", got.Warnings)
	require.NotNil(t, got.NoTier)
	require.Len(t, got.NoTier.Instruments, 1)
	aaa := got.NoTier.Instruments[0]
	require.NotNil(t, aaa.Holding)
	assert.Equal(t, "430.00", aaa.Holding.Cost)
	assert.Equal(t, ptr("396.00"), aaa.Holding.Value)
	assert.Equal(t, ptr("-34.00"), aaa.Holding.UnrealizedProfit)
	require.NotNil(t, aaa.ClosedPositions)
	assert.Equal(t, "430.00", aaa.ClosedPositions.Cost)
	assert.Equal(t, "456.00", aaa.ClosedPositions.SaleAmount)
	assert.Equal(t, "26.00", aaa.ClosedPositions.RealizedProfit)
}

// TestAllTiersBreakdown_MissingRate_IsNotAvailable checks the warnings when a rate that the conversion needs is missing.
func TestAllTiersBreakdown_MissingRate_IsNotAvailable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// rates are the USD rates by day. A nil map makes each request of the USD rates fail.
		rates       map[string]string
		wantWarning string
	}{
		{name: "no rate", wantWarning: "No Exchange Rate for USD. Refresh the prices."},
		{
			name: "no rate on the day of the Fill", rates: map[string]string{"2026-10-22": "3.60"},
			wantWarning: "No Exchange Rate for USD on 2026-10-05. Refresh the prices.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := newFakeProvider()
			provider.set(t, "AMZN", "110.00", "USD", "2026-10-22")
			srv := startServerWithPrices(t, provider)
			if tc.rates == nil {
				srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
			}
			for day, rate := range tc.rates {
				srv.rateProvider.set(t, "USD", day, rate)
			}
			srv.importAccount(t, adr0010Export(), "XTB USD")
			srv.refreshPrices(t)

			var body map[string]any
			require.Equal(t, http.StatusOK, srv.getJSON(t, "/api/all-accounts/tiers-breakdown", &body))

			assert.Equal(t, map[string]any{"available": false, "warnings": []any{tc.wantWarning}}, body)
		})
	}
}

// TestAllTiersBreakdown_LeavesOutIKZEAndKraken checks that only the Tier Accounts are in the breakdown.
func TestAllTiersBreakdown_LeavesOutIKZEAndKraken(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importIKZE(t, ikzeDeposit("1", "100.00", octoberTime(5, 0)))
	srv.importKraken(t, krakenExport())

	got := srv.allTiersBreakdown(t)

	assert.True(t, got.Available)
	assert.Equal(t, []string{}, got.Warnings)
	assert.Nil(t, got.NoTier, "the BTC Holding of the Kraken Account is not in the breakdown")
	assert.Empty(t, got.Tiers)
	assert.Equal(t, "0.00", got.Total.Value)
}
