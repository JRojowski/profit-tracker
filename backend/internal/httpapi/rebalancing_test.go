package httpapi_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// rebalancingResponse is the body of GET /api/all-accounts/rebalancing.
type rebalancingResponse struct {
	Available        bool                  `json:"available"`
	Warnings         []string              `json:"warnings"`
	ExchangeRate     *rebalancingRate      `json:"exchangeRate"`
	Accounts         []rebalancingAccount  `json:"accounts"`
	MissingAccounts  []string              `json:"missingAccounts"`
	Value            *string               `json:"value"`
	Base             *string               `json:"base"`
	Tiers            []rebalancingTier     `json:"tiers"`
	NoTier           *rebalancingNoTier    `json:"noTier"`
	UnpricedHoldings []rebalancingUnpriced `json:"unpricedHoldings"`
}

// rebalancingRate is the latest Exchange Rate that converts the USD amounts.
type rebalancingRate struct {
	Currency string `json:"currency"`
	Rate     string `json:"rate"`
	Date     string `json:"date"`
}

// rebalancingAccount is one Tier Account with its cash for Rebalancing.
type rebalancingAccount struct {
	AccountID        int64  `json:"accountId"`
	Name             string `json:"name"`
	Currency         string `json:"currency"`
	FreeCash         string `json:"freeCash"`
	Cash             string `json:"cash"`
	PlannedPurchases string `json:"plannedPurchases"`
	UnusedCash       string `json:"unusedCash"`
}

// rebalancingTier is one Tier of the table.
type rebalancingTier struct {
	Order             int                   `json:"order"`
	Name              string                `json:"name"`
	Value             string                `json:"value"`
	ActualShare       string                `json:"actualShare"`
	TargetShare       string                `json:"targetShare"`
	TargetValue       string                `json:"targetValue"`
	Gap               string                `json:"gap"`
	PlannedPurchases  []rebalancingPurchase `json:"plannedPurchases"`
	InstrumentTargets []rebalancingPart     `json:"instrumentTargets"`
	SharedPart        rebalancingSharedPart `json:"sharedPart"`
}

// rebalancingPurchase is a Planned Purchase in the currency of its Account.
type rebalancingPurchase struct {
	AccountID *int64  `json:"accountId"`
	Currency  string  `json:"currency"`
	Amount    *string `json:"amount"`
}

// rebalancingPart is an Instrument Target.
type rebalancingPart struct {
	Ticker          string              `json:"ticker"`
	Name            string              `json:"name"`
	NoValue         bool                `json:"noValue"`
	Value           string              `json:"value"`
	ActualShare     string              `json:"actualShare"`
	TargetShare     string              `json:"targetShare"`
	TargetValue     string              `json:"targetValue"`
	Gap             string              `json:"gap"`
	PlannedPurchase rebalancingPurchase `json:"plannedPurchase"`
}

// rebalancingSharedPart is the Shared Part of a Tier with its Instruments.
type rebalancingSharedPart struct {
	Name            string                  `json:"name"`
	Value           string                  `json:"value"`
	ActualShare     string                  `json:"actualShare"`
	TargetShare     string                  `json:"targetShare"`
	TargetValue     string                  `json:"targetValue"`
	Gap             string                  `json:"gap"`
	PlannedPurchase rebalancingPurchase     `json:"plannedPurchase"`
	Instruments     []rebalancingInstrument `json:"instruments"`
}

// rebalancingInstrument is a Holding in the Shared Part or in the "No Tier" group.
type rebalancingInstrument struct {
	Ticker  string  `json:"ticker"`
	Name    string  `json:"name"`
	NoValue bool    `json:"noValue"`
	Value   *string `json:"value"`
	Share   *string `json:"share"`
}

// rebalancingNoTier is the group of the Holdings with no Tier.
type rebalancingNoTier struct {
	Value       string                  `json:"value"`
	ActualShare string                  `json:"actualShare"`
	TargetShare string                  `json:"targetShare"`
	TargetValue string                  `json:"targetValue"`
	Gap         string                  `json:"gap"`
	Holdings    []rebalancingInstrument `json:"holdings"`
}

// rebalancingUnpriced is a Holding with no price.
type rebalancingUnpriced struct {
	AccountID int64  `json:"accountId"`
	Ticker    string `json:"ticker"`
	Name      string `json:"name"`
}

// rebalancing returns GET /api/all-accounts/rebalancing with the query. It stops the test if the status is not 200.
func (s *testServer) rebalancing(t *testing.T, query string) rebalancingResponse {
	t.Helper()

	var got rebalancingResponse
	require.Equal(t, http.StatusOK, s.getJSON(t, "/api/all-accounts/rebalancing"+query, &got))
	return got
}

// rebalancingQuote is the close price of a price ticker on 2026-10-22.
type rebalancingQuote struct {
	priceTicker, close, currency string
}

// startRebalancingServer starts the backend with the prices and a USD rate of 4.00, imports each export as an
// Account and refreshes the prices. It returns the Account IDs in the order of the exports.
func startRebalancingServer(t *testing.T, quotes []rebalancingQuote, exports ...xtbtest.Export) (*testServer, []int64) {
	t.Helper()

	provider := newFakeProvider()
	for _, q := range quotes {
		provider.set(t, q.priceTicker, q.close, q.currency, "2026-10-22")
	}
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.00")
	ids := make([]int64, 0, len(exports))
	for i, export := range exports {
		ids = append(ids, srv.importAccount(t, export, fmt.Sprintf("Account %d", i+1)))
	}
	srv.refreshPrices(t)
	return srv, ids
}

// cashQuery returns the query that sets the cash for Rebalancing of the Accounts, as pairs of ID and amount.
func cashQuery(pairs ...any) string {
	query := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		sep := "&"
		if query == "" {
			sep = "?"
		}
		query += fmt.Sprintf("%scash=%d:%s", sep, pairs[i], pairs[i+1])
	}
	return query
}

func purchase(accountID int64, currency, amount string) rebalancingPurchase {
	return rebalancingPurchase{AccountID: &accountID, Currency: currency, Amount: &amount}
}

// TestRebalancing_GapsMoreThanCash_ScalesThePlannedPurchasesOfTheAccount checks the full table on two Accounts.
// The cash parameter sets the cash for Rebalancing. The USD rate is 4.00.
//
//	PLN Account: ETF.PL (PLN) value 400.00. Cash for Rebalancing 400.00 PLN.
//	USD Account: AAA.US value 150.00 USD = 600.00 PLN, BBB.US 50.00 USD = 200.00 PLN.
//	             Cash for Rebalancing 100.00 USD = 400.00 PLN.
//	Value of the Holdings: 1200.00. Base: 1200.00 + 400.00 + 400.00 = 2000.00
//
//	Tier I "Core" 50: value 1000.00, Actual Share 1000 / 1200, Target Value 1000.00, Gap 0.00
//	  ETF.PL Instrument Target 50: value 400.00, Actual Share 40, Target Value 500.00, Gap 100.00 -> PLN Account
//	  Shared Part "Blue Chips" 50 (AAA.US): value 600.00, Actual Share 60, Target Value 500.00, Gap -100.00 -> 0
//	Tier II "Bets" 50: value 200.00, Actual Share 200 / 1200, Target Value 1000.00, Gap 800.00
//	  Shared Part 100 (BBB.US): value 200.00, Target Value 1000.00, Gap 800.00 PLN = 200.00 USD -> USD Account
//
//	PLN Account: Gaps 100.00 <= cash 400.00, factor 1. Planned Purchase 100.00, unused cash 300.00.
//	USD Account: Gaps 200.00 USD > cash 100.00 USD, factor 0.5. Planned Purchase 100.00 USD, unused cash 0.00.
func TestRebalancing_GapsMoreThanCash_ScalesThePlannedPurchasesOfTheAccount(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t,
		[]rebalancingQuote{{"ETF.WA", "400.00", "PLN"}, {"AAA", "150.00", "USD"}, {"BBB", "50.00", "USD"}},
		allocationExport("57300001", "PLN", "1000.00", allocationHolding{ticker: "ETF.PL", cost: "300.00"}),
		allocationExport("57300002", "USD", "1000.00",
			allocationHolding{ticker: "AAA.US", cost: "100.00"}, allocationHolding{ticker: "BBB.US", cost: "100.00"}),
	)
	pln, usd := ids[0], ids[1]
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{
			Order: 1, Name: "Core", TargetShare: "50", SharedPartName: "Blue Chips",
			Instruments: []assignmentRequest{{Ticker: "ETF.PL", TargetShare: share("50")}, {Ticker: "AAA.US"}},
		},
		{Order: 2, Name: "Bets", TargetShare: "50", Instruments: []assignmentRequest{{Ticker: "BBB.US"}}},
	}})

	got := srv.rebalancing(t, cashQuery(pln, "400", usd, "100.00"))

	assert.True(t, got.Available)
	assert.Empty(t, got.Warnings)
	assert.Equal(t, &rebalancingRate{Currency: "USD", Rate: "4", Date: "2026-10-22"}, got.ExchangeRate)
	assert.Equal(t, []rebalancingAccount{
		{AccountID: pln, Name: "Account 1", Currency: "PLN", FreeCash: "700.00", Cash: "400.00", PlannedPurchases: "100.00", UnusedCash: "300.00"},
		{AccountID: usd, Name: "Account 2", Currency: "USD", FreeCash: "800.00", Cash: "100.00", PlannedPurchases: "100.00", UnusedCash: "0.00"},
	}, got.Accounts)
	assert.Empty(t, got.MissingAccounts)
	assert.Equal(t, ptr("1200.00"), got.Value)
	assert.Equal(t, ptr("2000.00"), got.Base)
	assert.Equal(t, []rebalancingTier{
		{
			Order: 1, Name: "Core", Value: "1000.00", ActualShare: "83.3333333333333333", TargetShare: "50",
			TargetValue: "1000.00", Gap: "0.00",
			PlannedPurchases: []rebalancingPurchase{purchase(pln, "PLN", "100.00"), purchase(usd, "USD", "0.00")},
			InstrumentTargets: []rebalancingPart{{
				Ticker: "ETF.PL", Name: "ETF.PL", Value: "400.00", ActualShare: "40", TargetShare: "50",
				TargetValue: "500.00", Gap: "100.00", PlannedPurchase: purchase(pln, "PLN", "100.00"),
			}},
			SharedPart: rebalancingSharedPart{
				Name: "Blue Chips", Value: "600.00", ActualShare: "60", TargetShare: "50", TargetValue: "500.00",
				Gap: "-100.00", PlannedPurchase: purchase(usd, "USD", "0.00"),
				Instruments: []rebalancingInstrument{{Ticker: "AAA.US", Name: "AAA", Value: ptr("600.00"), Share: ptr("100")}},
			},
		},
		{
			Order: 2, Name: "Bets", Value: "200.00", ActualShare: "16.6666666666666667", TargetShare: "50",
			TargetValue: "1000.00", Gap: "800.00",
			PlannedPurchases:  []rebalancingPurchase{purchase(pln, "PLN", "0.00"), purchase(usd, "USD", "100.00")},
			InstrumentTargets: []rebalancingPart{},
			SharedPart: rebalancingSharedPart{
				Value: "200.00", ActualShare: "100", TargetShare: "100", TargetValue: "1000.00",
				Gap: "800.00", PlannedPurchase: purchase(usd, "USD", "100.00"),
				Instruments: []rebalancingInstrument{{Ticker: "BBB.US", Name: "BBB", Value: ptr("200.00"), Share: ptr("100")}},
			},
		},
	}, got.Tiers)
	assert.Equal(t, &rebalancingNoTier{
		Value: "0.00", ActualShare: "0", TargetShare: "0", TargetValue: "0.00", Gap: "0.00",
		Holdings: []rebalancingInstrument{},
	}, got.NoTier)
	assert.Empty(t, got.UnpricedHoldings)
}

// TestRebalancing_NoCashParameter_UsesTheFreeCash checks the default cash and a Shared Part with no Instruments.
// The tool has no PLN Account.
//
//	USD Account: AAA.US value 100.00 USD = 400.00 PLN. Free cash 900.00 USD = 3600.00 PLN. Base 4000.00
//	Tier "Held" 50 (AAA.US): Target Value 2000.00, Gap 1600.00 PLN = 400.00 USD
//	Tier "New"  50 (no Instruments): value 0.00, Target Value 2000.00, Gap 2000.00 PLN = 500.00 USD
//	USD Account: Gaps 900.00 USD = cash 900.00 USD, factor 1. Unused cash 0.00.
func TestRebalancing_NoCashParameter_UsesTheFreeCash(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"AAA", "100.00", "USD"}},
		allocationExport("57300003", "USD", "1000.00", allocationHolding{ticker: "AAA.US", cost: "100.00"}))
	usd := ids[0]
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "Held", TargetShare: "50", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
		{Order: 2, Name: "New", TargetShare: "50"},
	}})

	got := srv.rebalancing(t, "")

	assert.Equal(t, []rebalancingAccount{
		{AccountID: usd, Name: "Account 1", Currency: "USD", FreeCash: "900.00", Cash: "900.00", PlannedPurchases: "900.00", UnusedCash: "0.00"},
	}, got.Accounts)
	assert.Equal(t, []string{"PLN"}, got.MissingAccounts)
	require.Len(t, got.Tiers, 2)
	assert.Equal(t, purchase(usd, "USD", "400.00"), got.Tiers[0].SharedPart.PlannedPurchase)
	assert.Equal(t, rebalancingSharedPart{
		Value: "0.00", ActualShare: "0", TargetShare: "100", TargetValue: "2000.00", Gap: "2000.00",
		PlannedPurchase: purchase(usd, "USD", "500.00"), Instruments: []rebalancingInstrument{},
	}, got.Tiers[1].SharedPart)
	assert.Equal(t, []rebalancingPurchase{purchase(usd, "USD", "500.00")}, got.Tiers[1].PlannedPurchases)
}

// TestRebalancing_InstrumentOnBothAccounts_CountsAsOneHolding checks that one Instrument on the PLN Account and on the
// USD Account is one Holding in its Tier.
//
//	PLN Account: AAA.US 1 unit, 100.00 USD × 4.00 = 400.00 PLN
//	USD Account: AAA.US 1 unit, 100.00 USD = 400.00 PLN
//	Tier "All" 100 (AAA.US): value 800.00. Cash for Rebalancing 0 on both Accounts.
func TestRebalancing_InstrumentOnBothAccounts_CountsAsOneHolding(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"AAA", "100.00", "USD"}},
		allocationExport("57300004", "PLN", "1000.00", allocationHolding{ticker: "AAA.US", cost: "400.00"}),
		allocationExport("57300005", "USD", "1000.00", allocationHolding{ticker: "AAA.US", cost: "100.00"}),
	)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "All", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
	}})

	got := srv.rebalancing(t, cashQuery(ids[0], "0", ids[1], "0"))

	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "800.00", got.Tiers[0].Value)
	assert.Equal(t, "0.00", got.Tiers[0].Gap)
	assert.Equal(t, []rebalancingInstrument{{Ticker: "AAA.US", Name: "AAA", Value: ptr("800.00"), Share: ptr("100")}},
		got.Tiers[0].SharedPart.Instruments)
	assert.Empty(t, got.NoTier.Holdings)
}

// TestRebalancing_HoldingWithNoTier_IsInTheNoTierGroup checks the "No Tier" group.
//
//	USD Account: AAA.US 400.00 PLN in Tier "All" 100, BBB.US 400.00 PLN with no Tier. Cash 0. Base 800.00
//	Tier "All": value 400.00, Actual Share 50, Target Value 800.00, Gap 400.00. Cash 0, so factor 0.
//	No Tier: value 400.00, Actual Share 50, Target Share 0, Target Value 0.00, Gap -400.00
func TestRebalancing_HoldingWithNoTier_IsInTheNoTierGroup(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"AAA", "100.00", "USD"}, {"BBB", "100.00", "USD"}},
		allocationExport("57300006", "USD", "1000.00",
			allocationHolding{ticker: "AAA.US", cost: "100.00"}, allocationHolding{ticker: "BBB.US", cost: "100.00"}))
	usd := ids[0]
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "All", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
	}})

	got := srv.rebalancing(t, cashQuery(usd, "0"))

	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "50", got.Tiers[0].ActualShare)
	assert.Equal(t, "400.00", got.Tiers[0].Gap)
	assert.Equal(t, purchase(usd, "USD", "0.00"), got.Tiers[0].SharedPart.PlannedPurchase)
	assert.Equal(t, &rebalancingNoTier{
		Value: "400.00", ActualShare: "50", TargetShare: "0", TargetValue: "0.00", Gap: "-400.00",
		Holdings: []rebalancingInstrument{{Ticker: "BBB.US", Name: "BBB", Value: ptr("400.00"), Share: ptr("100")}},
	}, got.NoTier)
}

// TestRebalancing_HoldingWithNoPrice_IsLeftOutOfTheValues checks that a Holding with no price adds nothing.
//
//	USD Account: AAA.US 400.00 PLN, CCC.US with no price, both in Tier "All" 100. Cash 0.
func TestRebalancing_HoldingWithNoPrice_IsLeftOutOfTheValues(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"AAA", "100.00", "USD"}},
		allocationExport("57300007", "USD", "1000.00",
			allocationHolding{ticker: "AAA.US", cost: "100.00"}, allocationHolding{ticker: "CCC.US", cost: "100.00"}))
	usd := ids[0]
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "All", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}, {Ticker: "CCC.US"}}},
	}})

	got := srv.rebalancing(t, cashQuery(usd, "0"))

	assert.Equal(t, ptr("400.00"), got.Value)
	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "400.00", got.Tiers[0].Value)
	assert.Equal(t, "0.00", got.Tiers[0].Gap)
	assert.Equal(t, []rebalancingInstrument{
		{Ticker: "AAA.US", Name: "AAA", Value: ptr("400.00"), Share: ptr("100")},
		{Ticker: "CCC.US", Name: "CCC", NoValue: true},
	}, got.Tiers[0].SharedPart.Instruments)
	assert.Equal(t, []rebalancingUnpriced{{AccountID: usd, Ticker: "CCC.US", Name: "CCC"}}, got.UnpricedHoldings)
}

// TestRebalancing_InstrumentTargetWithNoPrice_HasTheNoValueFlag checks the "no value" label of an Instrument Target.
func TestRebalancing_InstrumentTargetWithNoPrice_HasTheNoValueFlag(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, nil,
		allocationExport("57300008", "USD", "1000.00", allocationHolding{ticker: "CCC.US", cost: "100.00"}))
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "All", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "CCC.US", TargetShare: share("100")}}},
	}})

	got := srv.rebalancing(t, cashQuery(ids[0], "0"))

	require.Len(t, got.Tiers, 1)
	require.Len(t, got.Tiers[0].InstrumentTargets, 1)
	assert.True(t, got.Tiers[0].InstrumentTargets[0].NoValue)
	assert.Equal(t, "0.00", got.Tiers[0].InstrumentTargets[0].Value)
}

// TestRebalancing_NoUSDAccount_PartsOfTheUSDAccountHaveNoPlannedPurchase checks a missing USD Account.
//
//	PLN Account: ETF.PL value 400.00. Free cash 600.00. Base 1000.00
//	Tier "Core" 100: ETF.PL Instrument Target 50, Target Value 500.00, Gap 100.00 -> PLN Account 100.00
//	                 Shared Part 50, Target Value 500.00, Gap 500.00 -> no USD Account, no Planned Purchase
func TestRebalancing_NoUSDAccount_PartsOfTheUSDAccountHaveNoPlannedPurchase(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"ETF.WA", "400.00", "PLN"}},
		allocationExport("57300009", "PLN", "1000.00", allocationHolding{ticker: "ETF.PL", cost: "400.00"}))
	pln := ids[0]
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "ETF.PL", TargetShare: share("50")}}},
	}})

	got := srv.rebalancing(t, "")

	assert.Equal(t, []string{"USD"}, got.MissingAccounts)
	require.Len(t, got.Tiers, 1)
	assert.Equal(t, purchase(pln, "PLN", "100.00"), got.Tiers[0].InstrumentTargets[0].PlannedPurchase)
	assert.Equal(t, "500.00", got.Tiers[0].SharedPart.Gap)
	assert.Equal(t, rebalancingPurchase{Currency: "USD"}, got.Tiers[0].SharedPart.PlannedPurchase)
	assert.Equal(t, []rebalancingPurchase{purchase(pln, "PLN", "100.00")}, got.Tiers[0].PlannedPurchases)
	assert.Equal(t, []rebalancingAccount{
		{AccountID: pln, Name: "Account 1", Currency: "PLN", FreeCash: "600.00", Cash: "600.00", PlannedPurchases: "100.00", UnusedCash: "500.00"},
	}, got.Accounts)
}

// TestRebalancing_IKZEAccount_IsNotPartOfRebalancing checks that only the Tier Accounts count.
func TestRebalancing_IKZEAccount_IsNotPartOfRebalancing(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, []rebalancingQuote{{"AAA", "100.00", "USD"}},
		allocationExport("57300010", "USD", "1000.00", allocationHolding{ticker: "AAA.US", cost: "100.00"}))
	ikze := srv.importIKZE(t, ikzeDeposit("2", "1000.00", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)))

	got := srv.rebalancing(t, "")

	require.Len(t, got.Accounts, 1)
	assert.Equal(t, ids[0], got.Accounts[0].AccountID)
	status, _, body := srv.get(t, "/api/all-accounts/rebalancing"+cashQuery(ikze, "10"))
	assert.Equal(t, http.StatusBadRequest, status, body)
}

// TestRebalancing_NoTiersAndNoAccounts_ReturnsAnEmptyTable checks the start state.
func TestRebalancing_NoTiersAndNoAccounts_ReturnsAnEmptyTable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	got := srv.rebalancing(t, "")

	assert.True(t, got.Available)
	assert.Empty(t, got.Tiers)
	assert.Empty(t, got.Accounts)
	assert.Equal(t, []string{"PLN", "USD"}, got.MissingAccounts)
	assert.Equal(t, ptr("0.00"), got.Base)
}

// TestRebalancing_CashParameterNotValid_Returns400 checks each form of the cash parameter that the tool refuses.
func TestRebalancing_CashParameterNotValid_Returns400(t *testing.T) {
	t.Parallel()
	srv, ids := startRebalancingServer(t, nil, allocationExport("57300011", "USD", "1000.00"))
	id := ids[0]
	tests := []struct {
		name, query string
	}{
		{"no colon", "?cash=100"},
		{"ID is not a number", "?cash=x:100"},
		{"amount is not a number", fmt.Sprintf("?cash=%d:abc", id)},
		{"negative amount", fmt.Sprintf("?cash=%d:-1", id)},
		{"two parameters for one Account", cashQuery(id, "1", id, "2")},
		{"unknown Account", "?cash=9999:100"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, _, body := srv.get(t, "/api/all-accounts/rebalancing"+tc.query)

			assert.Equal(t, http.StatusBadRequest, status, body)
		})
	}
}

// TestRebalancing_NoUSDRate_NotAvailable checks that the table needs the latest USD rate when a USD Account exists.
func TestRebalancing_NoUSDRate_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, allocationExport("57300012", "USD", "1000.00"), "USD Account")
	srv.refreshPrices(t)

	got := srv.rebalancing(t, "")

	assert.False(t, got.Available)
	assert.Equal(t, []string{"No Exchange Rate for USD. Refresh the prices."}, got.Warnings)
	assert.Nil(t, got.ExchangeRate)
	assert.Nil(t, got.Base)
	assert.Empty(t, got.Tiers)
}
