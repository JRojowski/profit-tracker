package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// tiersBreakdownResponse is the body of GET /api/accounts/{id}/tiers-breakdown.
type tiersBreakdownResponse struct {
	Currency string              `json:"currency"`
	Tiers    []breakdownGroupDTO `json:"tiers"`
	NoTier   *breakdownGroupDTO  `json:"noTier"`
	Total    breakdownSumDTO     `json:"total"`
}

// breakdownGroupDTO is a Tier, or the "No Tier" group.
type breakdownGroupDTO struct {
	Order       *int                     `json:"order"`
	Name        string                   `json:"name"`
	Instruments []breakdownInstrumentDTO `json:"instruments"`
	Sum         breakdownSumDTO          `json:"sum"`
}

// breakdownInstrumentDTO is an Instrument with its optional Holding part and Closed Positions part.
type breakdownInstrumentDTO struct {
	Ticker          string               `json:"ticker"`
	Name            string               `json:"name"`
	Holding         *breakdownHoldingDTO `json:"holding"`
	ClosedPositions *breakdownClosedDTO  `json:"closedPositions"`
}

// breakdownHoldingDTO is the Holding part of an Instrument.
type breakdownHoldingDTO struct {
	NoValue          bool    `json:"noValue"`
	Value            *string `json:"value"`
	Cost             string  `json:"cost"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	ReturnOnCost     *string `json:"returnOnCost"`
}

// breakdownClosedDTO is the Closed Positions part of an Instrument.
type breakdownClosedDTO struct {
	Count          int                      `json:"count"`
	Cost           string                   `json:"cost"`
	SaleAmount     string                   `json:"saleAmount"`
	RealizedProfit string                   `json:"realizedProfit"`
	ReturnOnCost   *string                  `json:"returnOnCost"`
	LastSaleDate   string                   `json:"lastSaleDate"`
	Positions      []closedPositionResponse `json:"positions"`
}

// breakdownSumDTO is a sum row.
type breakdownSumDTO struct {
	Value            string  `json:"value"`
	UnrealizedProfit string  `json:"unrealizedProfit"`
	RealizedProfit   string  `json:"realizedProfit"`
	TotalProfit      string  `json:"totalProfit"`
	Cost             string  `json:"cost"`
	ReturnOnCost     *string `json:"returnOnCost"`
}

// holdingPageRow is the part of a row of the Holdings page that the Tiers Breakdown shows.
type holdingPageRow struct {
	Ticker           string  `json:"ticker"`
	Cost             string  `json:"cost"`
	Value            *string `json:"value"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
}

// tiersBreakdown returns the Tiers Breakdown of the Account. It stops the test if the status is not 200.
func (s *testServer) tiersBreakdown(t *testing.T, accountID int64) tiersBreakdownResponse {
	t.Helper()

	var got tiersBreakdownResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/tiers-breakdown", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// breakdownExport is a USD Account with a Deposit of 1000.00 and these Lots:
//
//	AAA.US: bought 10 for 100.00, 4 sold for 60.00 on 2026-10-08. Open: 6, cost 60.00.
//	BBB.US: bought 2 for 50.00, all sold for 40.00 on 2026-10-09.
//	CCC.US: bought 1 for 30.00.
//	DDD.US: bought 1 for 20.00.
func breakdownExport() xtbtest.Export {
	buy := func(id, ticker, amount, volume string, minute int) xtbtest.CashOperation {
		return xtbtest.CashOperation{
			Type: "Stock purchase", Ticker: ticker, Instrument: ticker[:3], Amount: "-" + amount,
			Time: time.Date(2026, 10, 6, 9, minute, 0, 0, time.UTC), ID: id,
			Comment: "OPEN BUY " + volume + " @ 1", PositionID: id,
		}
	}
	sell := func(id, ticker, amount, closed, volume, position string, day int) xtbtest.CashOperation {
		return xtbtest.CashOperation{
			Type: "Stock sell", Ticker: ticker, Instrument: ticker[:3], Amount: amount,
			Time: time.Date(2026, 10, day, 15, 30, 0, 0, time.UTC), ID: id,
			Comment: "CLOSE BUY " + closed + "/" + volume + " @ 1", PositionID: position,
		}
	}
	return xtbtest.Export{
		AccountNumber: "57200001", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			cashDeposit("5720000001", "1000.00", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)),
			buy("1001", "AAA.US", "100.00", "10", 1),
			buy("1002", "BBB.US", "50.00", "2", 2),
			buy("1003", "CCC.US", "30.00", "1", 3),
			buy("1004", "DDD.US", "20.00", "1", 4),
			sell("2001", "AAA.US", "60.00", "4", "10", "1001", 8),
			sell("2002", "BBB.US", "40.00", "2", "2", "1002", 9),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "AAA.US", Volume: "4", OpenTime: time.Date(2026, 10, 6, 9, 1, 0, 0, time.UTC), CloseTime: time.Date(2026, 10, 8, 15, 30, 0, 0, time.UTC), ProfitLoss: "20.00", PositionID: "1001"},
			{Ticker: "BBB.US", Volume: "2", OpenTime: time.Date(2026, 10, 6, 9, 2, 0, 0, time.UTC), CloseTime: time.Date(2026, 10, 9, 15, 30, 0, 0, time.UTC), ProfitLoss: "-10.00", PositionID: "1002"},
		},
	}
}

// startBreakdownServer imports breakdownExport and the Tiers:
//
//	Tier I:  AAA.US, CCC.US (CCC.US has no price).
//	Tier II: BBB.US.
//	DDD.US has no Tier.
//
// The prices in USD: AAA.US 15.00, DDD.US 25.00.
func startBreakdownServer(t *testing.T) (*testServer, int64) {
	t.Helper()

	provider := newFakeProvider()
	provider.set(t, "AAA", "15.00", "USD", "2026-10-22")
	provider.set(t, "DDD", "25.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.00")
	id := srv.importAccount(t, breakdownExport(), "USD Account")
	srv.refreshPrices(t)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "I. Core", TargetShare: "60", Instruments: []assignmentRequest{{Ticker: "AAA.US"}, {Ticker: "CCC.US"}}},
		{Order: 2, Name: "II. Bets", TargetShare: "40", Instruments: []assignmentRequest{{Ticker: "BBB.US"}}},
	}})
	return srv, id
}

// TestTiersBreakdown_Holding_ShowsValueCostAndReturnOnCost
//
//	AAA.US open: value 6 × 15.00 = 90.00, cost 100.00 × 6 / 10 = 60.00, Unrealized Profit 30.00, Return on Cost 50.
func TestTiersBreakdown_Holding_ShowsValueCostAndReturnOnCost(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	got := srv.tiersBreakdown(t, id)

	require.Len(t, got.Tiers, 2)
	assert.Equal(t, "USD", got.Currency)
	first := got.Tiers[0]
	assert.Equal(t, "I. Core", first.Name)
	require.NotNil(t, first.Order)
	assert.Equal(t, 1, *first.Order)
	require.Len(t, first.Instruments, 2)
	assert.Equal(t, &breakdownHoldingDTO{
		Value: ptr("90.00"), Cost: "60.00", UnrealizedProfit: ptr("30.00"), ReturnOnCost: ptr("50"),
	}, first.Instruments[0].Holding)
}

// TestTiersBreakdown_ClosedPositions_ShowsCountCostSaleAmountAndLastSale
//
//	BBB.US: 1 Closed Position, cost 50.00, sale amount 40.00, Realized Profit -10.00, Return on Cost -20.
func TestTiersBreakdown_ClosedPositions_ShowsCountCostSaleAmountAndLastSale(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	got := srv.tiersBreakdown(t, id)

	bets := got.Tiers[1].Instruments
	require.Len(t, bets, 1)
	assert.Nil(t, bets[0].Holding, "BBB.US has no open Lot")
	closed := bets[0].ClosedPositions
	require.NotNil(t, closed)
	assert.Equal(t, 1, closed.Count)
	assert.Equal(t, "50.00", closed.Cost)
	assert.Equal(t, "40.00", closed.SaleAmount)
	assert.Equal(t, "-10.00", closed.RealizedProfit)
	assert.Equal(t, ptr("-20"), closed.ReturnOnCost)
	assert.Equal(t, "2026-10-09", closed.LastSaleDate)
	assert.Equal(t, srv.closedPositions(t, id)[1:], closed.Positions, "the single Closed Positions are the rows of the Closed Positions page")
}

// TestTiersBreakdown_OpenAndSold_HasBothParts
//
//	AAA.US: Closed Position cost 100.00 × 4 / 10 = 40.00, sale amount 60.00, Realized Profit 20.00, Return on Cost 50.
func TestTiersBreakdown_OpenAndSold_HasBothParts(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	aaa := srv.tiersBreakdown(t, id).Tiers[0].Instruments[0]

	assert.Equal(t, "AAA.US", aaa.Ticker)
	require.NotNil(t, aaa.Holding)
	require.NotNil(t, aaa.ClosedPositions)
	assert.Equal(t, "20.00", aaa.ClosedPositions.RealizedProfit)
	assert.Equal(t, "40.00", aaa.ClosedPositions.Cost)
	assert.Equal(t, ptr("50"), aaa.ClosedPositions.ReturnOnCost)
}

// TestTiersBreakdown_HoldingWithNoPrice_HasNoValueAndAddsNothingToTheSum
//
//	CCC.US: cost 30.00 is shown, but value, Unrealized Profit and Return on Cost are nil.
func TestTiersBreakdown_HoldingWithNoPrice_HasNoValueAndAddsNothingToTheSum(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	got := srv.tiersBreakdown(t, id)

	assert.Equal(t, &breakdownHoldingDTO{NoValue: true, Cost: "30.00"}, got.Tiers[0].Instruments[1].Holding)
	assert.Equal(t, "100.00", got.Tiers[0].Sum.Cost, "60.00 open + 40.00 closed, with no 30.00 of CCC.US")
}

// TestTiersBreakdown_Sums
//
//	Tier I:   value 90.00, unrealized 30.00, realized 20.00, total 50.00, cost 60.00 + 40.00 = 100.00, Return on Cost 50
//	Tier II:  value 0.00, unrealized 0.00, realized -10.00, total -10.00, cost 50.00, Return on Cost -20
//	No Tier:  DDD.US value 25.00, cost 20.00, unrealized 5.00, Return on Cost 25
//	All:      value 115.00, unrealized 35.00, realized 10.00, total 45.00, cost 170.00, Return on Cost 45 × 100 / 170
func TestTiersBreakdown_Sums(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	got := srv.tiersBreakdown(t, id)

	assert.Equal(t, breakdownSumDTO{Value: "90.00", UnrealizedProfit: "30.00", RealizedProfit: "20.00", TotalProfit: "50.00", Cost: "100.00", ReturnOnCost: ptr("50")}, got.Tiers[0].Sum)
	assert.Equal(t, breakdownSumDTO{Value: "0.00", UnrealizedProfit: "0.00", RealizedProfit: "-10.00", TotalProfit: "-10.00", Cost: "50.00", ReturnOnCost: ptr("-20")}, got.Tiers[1].Sum)
	require.NotNil(t, got.NoTier)
	assert.Equal(t, "No Tier", got.NoTier.Name)
	assert.Nil(t, got.NoTier.Order)
	require.Len(t, got.NoTier.Instruments, 1)
	assert.Equal(t, "DDD.US", got.NoTier.Instruments[0].Ticker)
	assert.Equal(t, breakdownSumDTO{Value: "25.00", UnrealizedProfit: "5.00", RealizedProfit: "0.00", TotalProfit: "5.00", Cost: "20.00", ReturnOnCost: ptr("25")}, got.NoTier.Sum)
	assert.Equal(t, breakdownSumDTO{Value: "115.00", UnrealizedProfit: "35.00", RealizedProfit: "10.00", TotalProfit: "45.00", Cost: "170.00", ReturnOnCost: ptr("26.4705882352941176")}, got.Total)
}

// TestTiersBreakdown_ValuesAgreeWithTheHoldingsPage checks that the Holding parts equal the rows of the Holdings page.
func TestTiersBreakdown_ValuesAgreeWithTheHoldingsPage(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)

	got := srv.tiersBreakdown(t, id)

	byTicker := map[string]breakdownHoldingDTO{}
	require.NotNil(t, got.NoTier)
	for _, g := range append(got.Tiers, *got.NoTier) {
		for _, in := range g.Instruments {
			if in.Holding != nil {
				byTicker[in.Ticker] = *in.Holding
			}
		}
	}
	var rows []holdingPageRow
	require.Equal(t, http.StatusOK, srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", id), &rows))
	for _, h := range rows {
		require.Contains(t, byTicker, h.Ticker)
		assert.Equal(t, h.Value, byTicker[h.Ticker].Value, h.Ticker)
		assert.Equal(t, h.Cost, byTicker[h.Ticker].Cost, h.Ticker)
		assert.Equal(t, h.UnrealizedProfit, byTicker[h.Ticker].UnrealizedProfit, h.Ticker)
	}
}

// TestTiersBreakdown_NoTiers_PutsEverythingInNoTier checks an Account with no configuration.
func TestTiersBreakdown_NoTiers_PutsEverythingInNoTier(t *testing.T) {
	t.Parallel()
	srv, id := startBreakdownServer(t)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{}})

	got := srv.tiersBreakdown(t, id)

	assert.Empty(t, got.Tiers)
	require.NotNil(t, got.NoTier)
	assert.Len(t, got.NoTier.Instruments, 4)
}

// TestTiersBreakdown_NotATierAccount_Returns404 checks an IKZE Account, a Kraken Account and an unknown Account.
func TestTiersBreakdown_NotATierAccount_Returns404(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	ikze := srv.importIKZE(t, ikzeDeposit("1", "100.00", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)))
	kraken := srv.importKraken(t, krakenExport())

	for name, id := range map[string]int64{"IKZE": ikze, "Kraken": kraken, "unknown": 9999} {
		status, _, _ := srv.get(t, fmt.Sprintf("/api/accounts/%d/tiers-breakdown", id))
		assert.Equal(t, http.StatusNotFound, status, name)
	}
}
