package httpapi_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// TestTiersBreakdown_LastPurchaseDate_IsThePolishDayOfTheLatestFillOfTheOpenLots checks that the Holding part shows
// the latest Fill of the open Lots. Lot 7001 is bought on 2026-10-06. Lot 7002 is bought at 23:30 UTC on 2026-10-09,
// which is 2026-10-10 in Poland (UTC+2). Lot 7003 is bought on 2026-10-12 and sold on 2026-10-13, so it is not open.
func TestTiersBreakdown_LastPurchaseDate_IsThePolishDayOfTheLatestFillOfTheOpenLots(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 10, 9, 23, 30, 0, 0, time.UTC)
	closedBuy := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	sold := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	id := srv.importAccount(t, xtbtest.Export{
		AccountNumber: "57200004", Currency: "PLN",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("2", "1000.00", first.Add(-time.Hour)),
			tierBuy("7001", "OPEN.PL", "100.00", first),
			tierBuy("7002", "OPEN.PL", "100.00", late),
			tierBuy("7003", "OPEN.PL", "100.00", closedBuy),
			tierSell("7004", "OPEN.PL", "120.00", "1", "7003", sold),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "OPEN.PL", Volume: "1", OpenTime: closedBuy, CloseTime: sold, ProfitLoss: "20.00", PositionID: "7003"},
		},
		OpenLots: []xtbtest.OpenLot{
			{Ticker: "OPEN.PL", PositionID: "7001", Volume: "1"},
			{Ticker: "OPEN.PL", PositionID: "7002", Volume: "1"},
		},
	}, "PLN Account")
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "OPEN.PL"}},
	}}})

	got := srv.tiersBreakdown(t, id)

	require.Len(t, got.Tiers, 1)
	require.Len(t, got.Tiers[0].Instruments, 1)
	holding := got.Tiers[0].Instruments[0].Holding
	require.NotNil(t, holding)
	assert.Equal(t, "2026-10-10", holding.LastPurchaseDate)
}

// TestAllTiersBreakdown_LastPurchaseDate_TakesTheLatestDateOfBothAccounts checks that an Instrument on two Accounts
// shows the latest purchase of both. The open Lot of the PLN Account is bought on 2026-10-05, the one of the USD
// Account on 2026-10-14.
func TestAllTiersBreakdown_LastPurchaseDate_TakesTheLatestDateOfBothAccounts(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "AAA", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-14", "3.90")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "58200011", Currency: "PLN",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("4000", "1000.00", octoberTime(5, 0)),
			tierBuy("4001", "AAA.US", "400.00", octoberTime(5, 1)),
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AAA.US", PositionID: "4001", Volume: "1"}},
	}, "XTB PLN")
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "58200012", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("5000", "1000.00", octoberTime(5, 0)),
			tierBuy("5001", "AAA.US", "100.00", octoberTime(14, 1)),
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "AAA.US", PositionID: "5001", Volume: "1"}},
	}, "XTB USD")
	srv.refreshPrices(t)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "I. Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
	}})

	got := srv.allTiersBreakdown(t)

	require.True(t, got.Available, "warnings: %v", got.Warnings)
	require.Len(t, got.Tiers, 1)
	require.Len(t, got.Tiers[0].Instruments, 1)
	holding := got.Tiers[0].Instruments[0].Holding
	require.NotNil(t, holding)
	assert.Equal(t, "2026-10-14", holding.LastPurchaseDate)
}
