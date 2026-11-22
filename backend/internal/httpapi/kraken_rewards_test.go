package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken/krakentest"
)

// krakenReward returns one weekly Kraken Reward of the BTC amount at the time.
func krakenReward(refID string, at time.Time, amount string) krakentest.Entry {
	return krakentest.Entry{RefID: refID, Time: at, Type: "earn", Subtype: "reward", Asset: "BTC", Amount: amount}
}

// krakenExportWithRewards is krakenExport with the Rewards added after the purchases.
func krakenExportWithRewards(rewards ...krakentest.Entry) krakentest.Export {
	export := krakenExport()
	export.Entries = append(export.Entries, rewards...)
	return export
}

// TestKrakenReward_StoredClose_AddsVolumeAndCostAtTheCloseOfItsDay checks the cost of a Reward. ADR 0009, rule 3.
//
//	Reward on 2026-03-10: 0.00000110 BTC, the BTC-EUR close of 2026-03-10 is 62000.
//	Volume: 0.00448875 + 0.00000110 = 0.00448985
//	Cost:   300.00 + 0.00000110 × 62000 = 300.0682
//	Average Cost: 300.0682 / 0.00448985 = 66832.5667895363987661
func TestKrakenReward_StoredClose_AddsVolumeAndCostAtTheCloseOfItsDay(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-09=60000", "2026-03-10=62000", "2026-03-11=65000")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	srv.refreshHistory(t)

	assert.Equal(t, []holdingResponse{{
		Ticker: "BTC", Name: "BTC", Volume: "0.00448985", Cost: "300.0682", AverageCost: "66832.5667895363987661",
	}}, srv.holdings(t, id))
}

// rewardsSummary is the part of GET /api/accounts/{id}/summary that the Reward tests read.
type rewardsSummary struct {
	Rewards          *string `json:"rewards"`
	UnrealizedProfit string  `json:"unrealizedProfit"`
	TotalResult      string  `json:"totalResult"`
	Value            string  `json:"value"`
	UnpricedHoldings int     `json:"unpricedHoldings"`
	FeeSum           string  `json:"feeSum"`
}

// rewardsSummary returns the Reward fields of the summary of the Account.
func (s *testServer) rewardsSummary(t *testing.T, accountID int64) rewardsSummary {
	t.Helper()

	var got rewardsSummary
	require.Equal(t, http.StatusOK, s.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountID), &got))
	return got
}

// TestKrakenReward_LaterPriceChange_GoesIntoUnrealizedProfitNotIntoRewards checks the Rewards part. ADR 0009, rule 4.
//
//	Reward on 2026-03-10: 0.00000110 × 62000 (close of the day)                 = 0.0682
//	Reward on 2026-03-17: 0.00000120 × 64000 (close of 2026-03-16, on or before) = 0.0768
//	Rewards: 0.0682 + 0.0768 = 0.1450
//	Volume: 0.00448875 + 0.00000110 + 0.00000120 = 0.00449105, cost 300.00 + 0.1450 = 300.1450
//	Value at 70000: 314.3735. Unrealized Profit: 314.3735 - 300.1450 = 14.2285
//	Total Result: 14.2285 + 0.1450 = 14.3735 = 500.00 + 314.3735 - 800.00
func TestKrakenReward_LaterPriceChange_GoesIntoUnrealizedProfitNotIntoRewards(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-10=62000", "2026-03-16=64000", "2026-03-18=66000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(
		krakenReward("ERN-0001", krakenTime(10), "0.00000110"),
		krakenReward("ERN-0002", krakenTime(17), "0.00000120"),
	))

	srv.refreshHistory(t)

	got := srv.rewardsSummary(t, id)
	assert.Equal(t, ptr("0.145"), got.Rewards)
	assert.Equal(t, "14.2285", got.UnrealizedProfit)
	assert.Equal(t, "14.3735", got.TotalResult)
	assert.Equal(t, "814.3735", got.Value)
}

// TestKrakenFeeSum_Purchases_GivesTheSumOfTheFeesInEUR checks the sum of the fees. ADR 0009, rule 2.
//
//	TRD-0001: fee 0.00000500 BTC × price 100.00 / 0.00200000 = 50000 → 0.25
//	TRD-0002: fee 0.00000625 BTC × price 200.00 / 0.00250000 = 80000 → 0.50
//	Sum of the fees: 0.75. It is information only, so the fees part and the Total Result do not include it.
func TestKrakenFeeSum_Purchases_GivesTheSumOfTheFeesInEUR(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	got := srv.rewardsSummary(t, id)

	assert.Equal(t, "0.75", got.FeeSum)
	assert.Equal(t, "0.00", got.TotalResult, "no price, so no Unrealized Profit, and the Rewards part is not known")
}

// TestKrakenFeeSum_FeeInEUR_GoesIntoCashCostAndFeeSum checks a purchase with the fee on the EUR row. ADR 0009, rules 1 and 2.
//
//	TRD-0003: EUR -100.00 with the fee 0.40 EUR, BTC 0.00200000 with no fee
//	Cash:         500.00 - 100.00 - 0.40 = 399.60
//	BTC:          volume 0.00448875 + 0.00200000 = 0.00648875 (no fee deduction)
//	              cost 300.00 + 100.40 = 400.40, Average Cost 400.40 / 0.00648875 = 61706.8002311693315353
//	Sum of the fees: 0.25 + 0.50 (fees in BTC × price) + 0.40 (fee in EUR) = 1.15
func TestKrakenFeeSum_FeeInEUR_GoesIntoCashCostAndFeeSum(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := krakenExport()
	export.Entries = append(export.Entries,
		krakentest.Entry{RefID: "TRD-0003", Time: krakenTime(5), Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: "-100.00", Fee: "0.40"},
		krakentest.Entry{RefID: "TRD-0003", Time: krakenTime(5), Type: "trade", Subtype: "tradespot", Asset: "BTC", Amount: "0.00200000"},
	)
	id := srv.importKraken(t, export)

	summary := srv.summary(t, id)
	assert.Equal(t, "399.60", summary.Cash)
	assert.Equal(t, "0.00", summary.Fees, "ADR 0009: the fee is in the cost, not in the fees part")
	assert.Equal(t, "1.15", srv.rewardsSummary(t, id).FeeSum)
	assert.Equal(t, []holdingResponse{{
		Ticker: "BTC", Name: "BTC", Volume: "0.00648875", Cost: "400.40", AverageCost: "61706.8002311693315353",
	}}, srv.holdings(t, id))
}

func TestKrakenFeeSum_KnownPurchaseWithChangedEURFee_GivesImportConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	withFee := func(fee string) krakentest.Export {
		return krakentest.Export{Entries: []krakentest.Entry{
			krakentest.Deposit("DEP-0001", krakenTime(1), "500.00"),
			{RefID: "TRD-0001", Time: krakenTime(2), Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: "-100.00", Fee: fee},
			{RefID: "TRD-0001", Time: krakenTime(2), Type: "trade", Subtype: "tradespot", Asset: "BTC", Amount: "0.00200000"},
		}}
	}
	srv.importKraken(t, withFee("0.40"))

	preview := srv.previewKraken(t, withFee("0.50"))

	require.Len(t, preview.Conflicts, 1)
	assert.Equal(t, "changed-data", preview.Conflicts[0].Kind)
	assert.Equal(t, "TRD-0001", preview.Conflicts[0].XTBID)
	assert.Equal(t, "-100.40", preview.Conflicts[0].Existing.Amount, "the EUR fee is a part of the amount")
	assert.Equal(t, "-100.50", preview.Conflicts[0].New.Amount)
}

func TestKrakenFeeSum_XTBAccount_GivesZeroRewardsAndZeroFeeSum(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, dashboardExport(), "XTB USD")

	got := srv.rewardsSummary(t, id)

	assert.Equal(t, ptr("0.00"), got.Rewards)
	assert.Equal(t, "0.00", got.FeeSum)
}

// rewardHolding is the part of a row of GET /api/accounts/{id}/holdings that the test of an unknown cost reads.
type rewardHolding struct {
	Ticker           string  `json:"ticker"`
	Volume           string  `json:"volume"`
	Cost             *string `json:"cost"`
	AverageCost      *string `json:"averageCost"`
	Value            *string `json:"value"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	Note             *string `json:"note"`
}

// TestKrakenReward_NoStoredClose_GivesRewardsAndHoldingCostWithNoKnownValue checks a Reward before the tool
// has the close of its day. The Holding has no known cost, so it has no value, like a Holding with no price.
func TestKrakenReward_NoStoredClose_GivesRewardsAndHoldingCostWithNoKnownValue(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	// The current price is known. The daily history has no close on or before the day of the Reward.
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-11=65000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	srv.refreshHistory(t)

	summary := srv.rewardsSummary(t, id)
	assert.Nil(t, summary.Rewards, "the Rewards part has no known value")
	assert.Equal(t, 1, summary.UnpricedHoldings)
	assert.Equal(t, "0.00", summary.UnrealizedProfit, "the Holding with no known cost has no Unrealized Profit")
	assert.Equal(t, "0.00", summary.TotalResult)
	assert.Equal(t, "500.00", summary.Value, "the value leaves out the Holding with no known cost")

	var holdings []rewardHolding
	require.Equal(t, http.StatusOK, srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", id), &holdings))
	assert.Equal(t, []rewardHolding{{
		Ticker: "BTC", Volume: "0.00448985",
		Note: ptr("A Reward has no close of its day or an earlier day. Refresh the prices to get the price history."),
	}}, holdings)
}
