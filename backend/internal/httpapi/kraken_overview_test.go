package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken/krakentest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cryptoOverview is the part of the Instrument Overview that the tests of a crypto Instrument read.
type cryptoOverview struct {
	Ticker            string            `json:"ticker"`
	Name              string            `json:"name"`
	Fills             []cryptoFill      `json:"fills"`
	Holding           *holdingResponse  `json:"holding"`
	OpenLots          []json.RawMessage `json:"openLots"`
	ClosedPositions   []json.RawMessage `json:"closedPositions"`
	RealizedProfitSum *string           `json:"realizedProfitSum"`
	Rewards           []overviewReward  `json:"rewards"`
	FeeSum            *string           `json:"feeSum"`
}

// cryptoFill is one purchase in the Instrument Overview, with the fields of a crypto purchase.
type cryptoFill struct {
	XTBID       string  `json:"xtbId"`
	Time        string  `json:"time"`
	Volume      string  `json:"volume"`
	Price       *string `json:"price"`
	PositionID  string  `json:"positionId"`
	Cost        *string `json:"cost"`
	Fee         *string `json:"fee"`
	FeeCurrency *string `json:"feeCurrency"`
}

// overviewReward is one Reward in the Instrument Overview.
type overviewReward struct {
	XTBID  string  `json:"xtbId"`
	Time   string  `json:"time"`
	Volume string  `json:"volume"`
	Value  *string `json:"value"`
}

// cryptoOverview returns the Instrument Overview with the crypto fields. It stops the test if the status is not 200.
func (s *testServer) cryptoOverview(t *testing.T, accountID int64, ticker string) cryptoOverview {
	t.Helper()

	var got cryptoOverview
	require.Equal(t, http.StatusOK, s.getJSON(t, overviewPath(accountID, ticker), &got))
	return got
}

// TestCryptoOverview_PurchasesAndRewards_GivesFillsAverageCostRewardsAndFeeSum checks the Instrument Overview
// of a crypto Instrument. ADR 0009.
//
//	TRD-0001: 100.00 EUR, 0.00200000 BTC, fee 0.00000500 BTC → volume 0.001995, price 50000, fee value 0.25
//	TRD-0002: 200.00 EUR, 0.00250000 BTC, fee 0.00000625 BTC → volume 0.00249375, price 80000, fee value 0.50
//	ERN-0001 on 2026-03-10: 0.00000110 × 62000 (close of the day)                 = 0.0682
//	ERN-0002 on 2026-03-17: 0.00000120 × 64000 (close of 2026-03-16, on or before) = 0.0768
//	Volume 0.00449105, cost 300.1450, Average Cost 300.145 / 0.00449105 = 66831.8099330891439641
//	Sum of the fees: 0.25 + 0.50 = 0.75
func TestCryptoOverview_PurchasesAndRewards_GivesFillsAverageCostRewardsAndFeeSum(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-10=62000", "2026-03-16=64000", "2026-03-18=66000")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(
		krakenReward("ERN-0001", krakenTime(10), "0.00000110"),
		krakenReward("ERN-0002", krakenTime(17), "0.00000120"),
	))
	srv.refreshHistory(t)

	got := srv.cryptoOverview(t, id, "BTC")

	assert.Equal(t, "BTC", got.Ticker)
	assert.Equal(t, "BTC", got.Name)
	assert.Equal(t, []cryptoFill{
		{
			XTBID: "TRD-0001", Time: "2026-03-03T09:00:00Z", Volume: "0.001995", Price: ptr("50000"),
			Cost: ptr("100.00"), Fee: ptr("0.000005"), FeeCurrency: ptr("BTC"),
		},
		{
			XTBID: "TRD-0002", Time: "2026-03-04T09:00:00Z", Volume: "0.00249375", Price: ptr("80000"),
			Cost: ptr("200.00"), Fee: ptr("0.00000625"), FeeCurrency: ptr("BTC"),
		},
	}, got.Fills)
	require.NotNil(t, got.Holding)
	assert.Equal(t, "66831.8099330891439641", got.Holding.AverageCost)
	assert.Equal(t, "300.145", got.Holding.Cost)
	assert.Equal(t, []overviewReward{
		{XTBID: "ERN-0001", Time: "2026-03-10T09:00:00Z", Volume: "0.0000011", Value: ptr("0.0682")},
		{XTBID: "ERN-0002", Time: "2026-03-17T09:00:00Z", Volume: "0.0000012", Value: ptr("0.0768")},
	}, got.Rewards)
	assert.Equal(t, ptr("0.75"), got.FeeSum)
	assert.Empty(t, got.OpenLots)
	assert.NotNil(t, got.OpenLots, "an empty array, not null")
	assert.Empty(t, got.ClosedPositions)
	assert.NotNil(t, got.ClosedPositions, "an empty array, not null")
	assert.Nil(t, got.RealizedProfitSum)
}

// TestCryptoOverview_RewardWithNoStoredClose_GivesRewardWithNullValue checks a Reward with no known value.
// ADR 0009, rule 3: the tool has no close on or before the day of the Reward.
func TestCryptoOverview_RewardWithNoStoredClose_GivesRewardWithNullValue(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	got := srv.cryptoOverview(t, id, "BTC")

	assert.Equal(t, []overviewReward{
		{XTBID: "ERN-0001", Time: "2026-03-10T09:00:00Z", Volume: "0.0000011", Value: nil},
	}, got.Rewards)
	assert.Equal(t, ptr("0.75"), got.FeeSum, "the sum of the fees needs no close")
}

// TestCryptoOverview_FeeInEUR_GivesTheFeeInEURAndTheCostWithTheFee checks a purchase with the fee on the EUR row.
// ADR 0009, rules 1 and 2.
//
//	TRD-0003: EUR -100.00 with the fee 0.40 EUR, BTC 0.00200000 with no fee → cost 100.40, price 50000
//	Sum of the fees: 0.25 + 0.50 + 0.40 = 1.15
func TestCryptoOverview_FeeInEUR_GivesTheFeeInEURAndTheCostWithTheFee(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := krakenExport()
	export.Entries = append(export.Entries,
		krakentest.Entry{RefID: "TRD-0003", Time: krakenTime(5), Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: "-100.00", Fee: "0.40"},
		krakentest.Entry{RefID: "TRD-0003", Time: krakenTime(5), Type: "trade", Subtype: "tradespot", Asset: "BTC", Amount: "0.00200000"},
	)
	id := srv.importKraken(t, export)

	got := srv.cryptoOverview(t, id, "BTC")

	require.Len(t, got.Fills, 3)
	assert.Equal(t, cryptoFill{
		XTBID: "TRD-0003", Time: "2026-03-05T09:00:00Z", Volume: "0.002", Price: ptr("50000"),
		Cost: ptr("100.40"), Fee: ptr("0.4"), FeeCurrency: ptr("EUR"),
	}, got.Fills[2])
	assert.Equal(t, ptr("1.15"), got.FeeSum)
	assert.Empty(t, got.Rewards)
	assert.NotNil(t, got.Rewards, "an empty array, not null")
}

func TestInstrumentOverview_InstrumentWithLots_GivesNoRewardsNoFeeSumAndNoPurchaseFields(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")

	got := srv.cryptoOverview(t, accountID, "ABC.US")

	assert.Equal(t, []cryptoFill{
		{XTBID: "902", Time: "2026-02-03T10:00:00Z", Volume: "2", Price: ptr("100"), PositionID: "9000000001"},
	}, got.Fills)
	assert.Empty(t, got.Rewards)
	assert.NotNil(t, got.Rewards, "an empty array, not null")
	assert.Nil(t, got.FeeSum)
}
