package httpapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pointsOn returns the points of the dates, in the order of the dates. It stops the test if a date has no point.
func pointsOn(t *testing.T, points []valuePoint, dates ...string) []valuePoint {
	t.Helper()

	byDate := make(map[string]valuePoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}
	result := make([]valuePoint, 0, len(dates))
	for _, d := range dates {
		p, ok := byDate[d]
		require.True(t, ok, "no point on %s", d)
		result = append(result, p)
	}
	return result
}

// TestKrakenValueHistory_EachDay_UsesTheAverageCostPathAndTheCloseOnOrBeforeTheDay checks the Value History of
// a Kraken Account. ADR 0009. The Value History steps over calendar days, so the weekend closes count.
//
//	2026-03-02 (before a purchase): cash 800.00                                      = 800.00
//	2026-03-03 (purchase):          700.00 + 0.001995 × 50000                        = 799.75
//	2026-03-04 (purchase):          500.00 + 0.00448875 × 52000                      = 733.415
//	2026-03-05 (no close):          500.00 + 0.00448875 × 52000 (close of 03-04)     = 733.415
//	2026-03-07 (Saturday):          500.00 + 0.00448875 × 54000                      = 742.3925
//	2026-03-08 (Sunday):            500.00 + 0.00448875 × 55000                      = 746.88125
//	2026-03-10 (Reward):            500.00 + 0.00448985 × 62000                      = 778.3707
//	2026-10-23 (today):             500.00 + 0.00448985 × 70000 (current price)      = 814.2895
func TestKrakenValueHistory_EachDay_UsesTheAverageCostPathAndTheCloseOnOrBeforeTheDay(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR",
		"2026-03-03=50000", "2026-03-04=52000", "2026-03-07=54000", "2026-03-08=55000", "2026-03-10=62000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	assert.Equal(t, "EUR", got.Currency)
	assert.Equal(t, []valuePoint{
		{Date: "2026-03-02", Value: "800.00", NetDeposits: "800.00"},
		{Date: "2026-03-03", Value: "799.75", NetDeposits: "800.00"},
		{Date: "2026-03-04", Value: "733.415", NetDeposits: "800.00"},
		{Date: "2026-03-05", Value: "733.415", NetDeposits: "800.00"},
		{Date: "2026-03-07", Value: "742.3925", NetDeposits: "800.00"},
		{Date: "2026-03-08", Value: "746.88125", NetDeposits: "800.00"},
		{Date: "2026-03-10", Value: "778.3707", NetDeposits: "800.00"},
		{Date: "2026-10-23", Value: "814.2895", NetDeposits: "800.00"},
	}, pointsOn(t, got.Points, "2026-03-02", "2026-03-03", "2026-03-04", "2026-03-05", "2026-03-07", "2026-03-08",
		"2026-03-10", "2026-10-23"))
	assert.Empty(t, got.ValuedAtCost)
	assert.Equal(t, "814.2895", srv.rewardsSummary(t, id).Value, "the value of today is the value in the hero")
}

// TestKrakenValueHistory_RewardWithNoClose_ValuesAtTheKnownCostAndTodayAtThePrice checks a Reward before the
// first stored close. The past days with no close value the Holding at the known part of its cost. ADR 0009,
// rule 3: the value does not need the cost, so a past day with a close and today use the price. The value of
// today is the value in the hero.
//
//	2026-03-04 (no close):         500.00 + cost 300.00                   = 800.00
//	2026-03-10 (Reward, no close): 500.00 + cost 300.00 (Reward unknown)  = 800.00
//	2026-03-11:                    500.00 + 0.00448985 × 65000            = 791.84025
//	2026-10-23 (today):            500.00 + 0.00448985 × 70000            = 814.2895
func TestKrakenValueHistory_RewardWithNoClose_ValuesAtTheKnownCostAndTodayAtThePrice(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-11=65000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))
	srv.refreshHistory(t)

	got := srv.valueHistory(t, id)

	assert.Equal(t, []valuePoint{
		{Date: "2026-03-04", Value: "800.00", NetDeposits: "800.00"},
		{Date: "2026-03-10", Value: "800.00", NetDeposits: "800.00"},
		{Date: "2026-03-11", Value: "791.84025", NetDeposits: "800.00"},
		{Date: "2026-10-23", Value: "814.2895", NetDeposits: "800.00"},
	}, pointsOn(t, got.Points, "2026-03-04", "2026-03-10", "2026-03-11", "2026-10-23"))
	assert.Equal(t, []valuedAtCostDTO{{Ticker: "BTC", Name: "BTC"}}, got.ValuedAtCost)
	assert.Equal(t, "814.2895", srv.rewardsSummary(t, id).Value, "the value of today is the value in the hero")
}
