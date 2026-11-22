package httpapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// rewardsAmount is the Rewards part of the All Accounts totals or of one row.
type rewardsAmount struct {
	Rewards *string `json:"rewards"`
}

// startXTBAndKrakenServer starts the backend with one XTB Account in PLN and the Kraken Account, and refreshes
// the prices. It returns the server and the IDs of the XTB Account and the Kraken Account.
//
//	XTB PLN: a Deposit of 1000.00 PLN on 2026-03-02.
//	Kraken:  krakenExport with a Reward of 0.00000110 BTC on 2026-03-10 (close 62000), BTC-EUR today 70000.
//	EUR:     4.20 on Friday 2026-02-27, 4.30 on 2026-03-02, 4.25 on 2026-10-22 (the latest rate).
func startXTBAndKrakenServer(t *testing.T) (srv *testServer, xtbID, krakenID int64) {
	t.Helper()

	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR",
		"2026-03-03=50000", "2026-03-04=52000", "2026-03-07=54000", "2026-03-08=55000", "2026-03-10=62000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv = startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "EUR", "2026-02-27", "4.20")
	srv.rateProvider.set(t, "EUR", "2026-03-02", "4.30")
	srv.rateProvider.set(t, "EUR", "2026-10-22", "4.25")
	xtbID = srv.importAccount(t, xtbtest.Export{AccountNumber: "59000001", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		cashDeposit("9101", "1000.00", krakenTime(2)),
	}}, "XTB PLN")
	krakenID = srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))
	srv.refreshHistory(t)
	return srv, xtbID, krakenID
}

// TestAllAccounts_XTBAndKraken_TotalResultInPLNHasTheRewardsAndTheEURCurrencyResult checks the Kraken Account in
// the All Accounts totals. ADR 0004 and ADR 0009.
//
//	Kraken in EUR: value 500.00 + 0.00448985 × 70000 = 814.2895, cost 300.00 + 0.0682 = 300.0682
//	               Unrealized Profit 314.2895 − 300.0682 = 14.2213, Rewards 0.0682
//	Kraken in PLN: value 814.2895 × 4.25 = 3460.730375, cash 500.00 × 4.25 = 2125.00
//	               Net Deposits 500.00 × 4.20 (Sunday: Friday rate) + 300.00 × 4.30 = 3390.00
//	               Unrealized Profit 14.2213 × 4.25 = 60.440525, Rewards 0.0682 × 4.25 = 0.28985
//	               Currency Result 500.00 × (4.25 − 4.20) + 300.00 × (4.25 − 4.30) = 10.00
//	               Total Result 60.440525 + 0.28985 + 10.00 = 70.730375 = 3460.730375 − 3390.00
//	All Accounts:  value 4460.730375, Net Deposits 4390.00, Total Result 70.730375
func TestAllAccounts_XTBAndKraken_TotalResultInPLNHasTheRewardsAndTheEURCurrencyResult(t *testing.T) {
	t.Parallel()
	srv, xtbID, krakenID := startXTBAndKrakenServer(t)

	got := srv.allAccounts(t)

	require.True(t, got.Available)
	require.Len(t, got.Accounts, 2)
	kraken := got.Accounts[0]
	require.Equal(t, krakenID, kraken.AccountID, "the rows are in the order of the names")
	assert.Equal(t, xtbID, got.Accounts[1].AccountID)
	assert.Equal(t, ptr("4.25"), kraken.ExchangeRate)
	assert.Equal(t, plnAmounts{
		Value: ptr("3460.730375"), Cash: ptr("2125.00"), NetDeposits: ptr("3390.00"), TotalResult: ptr("70.730375"),
		RealizedProfit: ptr("0.00"), UnrealizedProfit: ptr("60.440525"), NetDividends: ptr("0.00"),
		NetInterest: ptr("0.00"), Fees: ptr("0.00"), CurrencyResult: ptr("10.00"),
	}, kraken.plnAmounts)
	assert.Equal(t, ptr("4460.730375"), got.Value)
	assert.Equal(t, ptr("4390.00"), got.NetDeposits)
	assert.Equal(t, ptr("70.730375"), got.TotalResult)
	assert.Equal(t, ptr("10.00"), got.CurrencyResult)

	var rewards struct {
		rewardsAmount
		Accounts []rewardsAmount `json:"accounts"`
	}
	srv.getJSON(t, "/api/all-accounts/summary", &rewards)
	assert.Equal(t, ptr("0.28985"), rewards.Rewards)
	assert.Equal(t, []rewardsAmount{{Rewards: ptr("0.28985")}, {Rewards: ptr("0.00")}}, rewards.Accounts)
}

// TestAllAccounts_XTBAndKraken_AllocationHasBitcoinAsOnePart checks the Kraken Holding in the Allocation of all
// Accounts.
//
//	BTC:   0.00448985 × 70000 × 4.25 = 1335.730375 PLN
//	Cash:  1000.00 + 500.00 × 4.25   = 3125.00 PLN
//	Total: 4460.730375 PLN
func TestAllAccounts_XTBAndKraken_AllocationHasBitcoinAsOnePart(t *testing.T) {
	t.Parallel()
	srv, _, _ := startXTBAndKrakenServer(t)

	got := srv.allAllocation(t)

	assert.True(t, got.Available)
	assert.Equal(t, ptr("4460.730375"), got.Total)
	assert.Equal(t, []allocationPart{{Ticker: "BTC", Name: "BTC", Value: "1335.730375", Share: "29.9442078473528004"}},
		got.ByInstrument.Holdings)
	assert.Equal(t, cashPart("3125.00", "70.0557921526471996"), got.ByInstrument.Cash)
}

// TestAllValueHistory_XTBAndKraken_TimeWeightedReturnHasTheKrakenAccount checks the Kraken Account in the Value
// History and the Time-Weighted Return of all Accounts. The XTB Account holds only cash, so all of the return
// comes from the Kraken Account. The base day of 1M is 2026-09-23.
//
//	2026-09-23: 1000.00 + (500.00 + 0.00448985 × 62000) × 4.30 = 4346.99401
//	2026-10-22: 1000.00 + 814.2895 × 4.25 (close of 2026-10-22) = 4460.730375
//	2026-10-23: 1000.00 + 814.2895 × 4.25                      = 4460.730375
//	1M: 4460.730375 / 4346.99401 − 1 = 2.61643712 %
func TestAllValueHistory_XTBAndKraken_TimeWeightedReturnHasTheKrakenAccount(t *testing.T) {
	t.Parallel()
	srv, _, _ := startXTBAndKrakenServer(t)

	got := srv.allValueHistory(t)

	require.True(t, got.Available)
	assert.Equal(t, []valuePoint{
		{Date: "2026-03-01", Value: "2100.00", NetDeposits: "2100.00"},
		{Date: "2026-09-23", Value: "4346.99401", NetDeposits: "4390.00"},
		{Date: "2026-10-22", Value: "4460.730375", NetDeposits: "4390.00"},
		{Date: "2026-10-23", Value: "4460.730375", NetDeposits: "4390.00"},
	}, pointsOn(t, got.Points, "2026-03-01", "2026-09-23", "2026-10-22", "2026-10-23"))
	assert.Equal(t, ptr("2.61643712"), got.TimeWeightedReturn.OneMonth)
}
