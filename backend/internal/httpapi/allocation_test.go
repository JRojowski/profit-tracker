package httpapi_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// allocationResponse is the body of GET /api/accounts/{id}/allocation and GET /api/all-accounts/allocation.
type allocationResponse struct {
	Available        bool                    `json:"available"`
	Warnings         []string                `json:"warnings"`
	Currency         string                  `json:"currency"`
	Total            *string                 `json:"total"`
	ByInstrument     allocationByInstrument  `json:"byInstrument"`
	ByQuoteCurrency  []allocationCurrencyDTO `json:"byQuoteCurrency"`
	UnpricedHoldings int                     `json:"unpricedHoldings"`
}

// allocationByInstrument is the Allocation by Instrument.
type allocationByInstrument struct {
	Holdings []allocationPart     `json:"holdings"`
	Cash     *allocationPart      `json:"cash"`
	Other    *allocationOtherPart `json:"other"`
}

// allocationPart is one part of the Allocation.
type allocationPart struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Share  string `json:"share"`
}

// allocationOtherPart holds the Holdings after the 8 largest.
type allocationOtherPart struct {
	Count    int              `json:"count"`
	Value    string           `json:"value"`
	Share    string           `json:"share"`
	Holdings []allocationPart `json:"holdings"`
}

// allocationCurrencyDTO is one Quote Currency of the Allocation.
type allocationCurrencyDTO struct {
	Currency string `json:"currency"`
	Value    string `json:"value"`
	Share    string `json:"share"`
}

// accountAllocation returns the Allocation of the Account. It stops the test if the status is not 200.
func (s *testServer) accountAllocation(t *testing.T, accountID int64) allocationResponse {
	t.Helper()

	var got allocationResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/allocation", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// allAllocation returns the Allocation of all Accounts. It stops the test if the status is not 200.
func (s *testServer) allAllocation(t *testing.T) allocationResponse {
	t.Helper()

	var got allocationResponse
	status := s.getJSON(t, "/api/all-accounts/allocation", &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// allocationHolding is a purchase of 1 unit of a ".US" Instrument for cost, with the close price in USD.
// An empty price gives the Instrument no price.
type allocationHolding struct {
	ticker, cost, price string
}

// name is the Instrument name of the Holding: the ticker with no suffix.
func (h allocationHolding) name() string {
	return strings.TrimSuffix(h.ticker, ".US")
}

// allocationExport is an export of an Account with a Deposit and one purchase for each Holding.
// The Deposit is on 2026-10-05.
func allocationExport(accountNumber, currency, deposit string, holdings ...allocationHolding) xtbtest.Export {
	ops := make([]xtbtest.CashOperation, 0, len(holdings)+1)
	for i, h := range holdings {
		ops = append(ops, xtbtest.CashOperation{
			Type: "Stock purchase", Ticker: h.ticker, Instrument: h.name(), Amount: "-" + h.cost,
			Time: time.Date(2026, 10, 6, 9, i, 0, 0, time.UTC), ID: fmt.Sprintf("%s%02d", accountNumber, i+2),
			Comment: "OPEN BUY 1 @ " + h.cost, PositionID: fmt.Sprintf("%s%02d", accountNumber, i+2),
		})
	}
	ops = append(ops, cashDeposit(accountNumber+"01", deposit, time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)))
	return xtbtest.Export{AccountNumber: accountNumber, Currency: currency, CashOperations: ops}
}

// startAllocationServer starts the backend with the USD prices of the Holdings and refreshes the prices.
// It imports each export as an Account and returns the Account IDs in the order of the exports.
func startAllocationServer(t *testing.T, holdings []allocationHolding, exports ...xtbtest.Export) (*testServer, []int64) {
	t.Helper()

	provider := newFakeProvider()
	for _, h := range holdings {
		if h.price != "" {
			provider.set(t, h.name(), h.price, "USD", "2026-10-22")
		}
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

// part is an Allocation part of a Holding. The name is the ticker with no suffix.
func part(ticker, value, share string) allocationPart {
	return allocationPart{Ticker: ticker, Name: strings.TrimSuffix(ticker, ".US"), Value: value, Share: share}
}

// cashPart is the cash part of the Allocation.
func cashPart(value, share string) *allocationPart {
	return &allocationPart{Ticker: "", Name: "Cash", Value: value, Share: share}
}

// TestAllocation_ThreeHoldingsAndCash_SharesAddUpTo100 checks the parts of one Account.
//
//	Cash:  900.00 − 100.00 − 200.00 − 100.00 = 500.00
//	Total: 500.00 + 250.00 + 150.00 + 100.00 = 1000.00
//	Shares: 50 + 25 + 15 + 10 = 100
func TestAllocation_ThreeHoldingsAndCash_SharesAddUpTo100(t *testing.T) {
	t.Parallel()
	holdings := []allocationHolding{
		{ticker: "AAA.US", cost: "100.00", price: "150.00"},
		{ticker: "BBB.US", cost: "200.00", price: "100.00"},
		{ticker: "CCC.US", cost: "100.00", price: "250.00"},
	}
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000001", "USD", "900.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	assert.Equal(t, allocationResponse{
		Available: true, Warnings: []string{}, Currency: "USD", Total: ptr("1000.00"),
		ByInstrument: allocationByInstrument{
			Holdings: []allocationPart{
				part("CCC.US", "250.00", "25"),
				part("AAA.US", "150.00", "15"),
				part("BBB.US", "100.00", "10"),
			},
			Cash: cashPart("500.00", "50"),
		},
		ByQuoteCurrency: []allocationCurrencyDTO{{Currency: "USD", Value: "1000.00", Share: "100"}},
	}, got)
}

// tenHoldings are H01.US to H10.US. The value of Hnn is nn × 10.00.
func tenHoldings() []allocationHolding {
	holdings := make([]allocationHolding, 0, 10)
	for i := 1; i <= 10; i++ {
		value := fmt.Sprintf("%d.00", i*10)
		holdings = append(holdings, allocationHolding{ticker: fmt.Sprintf("H%02d.US", i), cost: value, price: value})
	}
	return holdings
}

// TestAllocation_TenHoldings_TwoSmallestGoToOther checks the 8 largest Holdings and the part "Other".
//
//	Holdings: 10.00 + 20.00 + … + 100.00 = 550.00
//	Cash:     1000.00 − 550.00 = 450.00
//	Total:    1000.00, so each share is the value / 10
//	Other:    H02.US 20.00 + H01.US 10.00 = 30.00
func TestAllocation_TenHoldings_TwoSmallestGoToOther(t *testing.T) {
	t.Parallel()
	holdings := tenHoldings()
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000002", "USD", "1000.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	assert.Equal(t, ptr("1000.00"), got.Total)
	assert.Equal(t, []allocationPart{
		part("H10.US", "100.00", "10"), part("H09.US", "90.00", "9"), part("H08.US", "80.00", "8"),
		part("H07.US", "70.00", "7"), part("H06.US", "60.00", "6"), part("H05.US", "50.00", "5"),
		part("H04.US", "40.00", "4"), part("H03.US", "30.00", "3"),
	}, got.ByInstrument.Holdings)
	assert.Equal(t, cashPart("450.00", "45"), got.ByInstrument.Cash)
	assert.Equal(t, &allocationOtherPart{
		Count: 2, Value: "30.00", Share: "3",
		Holdings: []allocationPart{part("H02.US", "20.00", "2"), part("H01.US", "10.00", "1")},
	}, got.ByInstrument.Other)
}

// TestAllocation_EightHoldings_NoOther checks that "Other" is null when there are exactly 8 Holdings.
//
//	Holdings: 30.00 + 40.00 + … + 100.00 = 520.00
//	Cash:     1000.00 − 520.00 = 480.00
func TestAllocation_EightHoldings_NoOther(t *testing.T) {
	t.Parallel()
	holdings := tenHoldings()[2:]
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000003", "USD", "1000.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	assert.Len(t, got.ByInstrument.Holdings, 8)
	assert.Nil(t, got.ByInstrument.Other)
	assert.Equal(t, cashPart("480.00", "48"), got.ByInstrument.Cash)
}

// TestAllocation_EqualValues_SortByTicker checks the order of two Holdings with the same value.
// The share does not terminate. It keeps 16 decimal places, as the other divisions of the API.
//
//	Total: 100.00 + 100.00 + 100.00 of cash = 300.00
//	Share: 100.00 × 100 / 300.00 = 33.3333333333333333
func TestAllocation_EqualValues_SortByTicker(t *testing.T) {
	t.Parallel()
	holdings := []allocationHolding{
		{ticker: "BBB.US", cost: "100.00", price: "100.00"},
		{ticker: "AAA.US", cost: "100.00", price: "100.00"},
	}
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000004", "USD", "300.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	third := "33.3333333333333333"
	assert.Equal(t, []allocationPart{part("AAA.US", "100.00", third), part("BBB.US", "100.00", third)}, got.ByInstrument.Holdings)
	assert.Equal(t, cashPart("100.00", third), got.ByInstrument.Cash)
}

// TestAllocation_HoldingWithNoPrice_IsNotAPart checks that a Holding with no known value is only counted.
//
//	Cash:  500.00 − 100.00 − 100.00 = 300.00
//	Total: 300.00 + 200.00 of AAA.US = 500.00. NOP.US has no price.
func TestAllocation_HoldingWithNoPrice_IsNotAPart(t *testing.T) {
	t.Parallel()
	holdings := []allocationHolding{
		{ticker: "AAA.US", cost: "100.00", price: "200.00"},
		{ticker: "NOP.US", cost: "100.00"},
	}
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000005", "USD", "500.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	assert.Equal(t, 1, got.UnpricedHoldings)
	assert.Equal(t, ptr("500.00"), got.Total)
	assert.Equal(t, []allocationPart{part("AAA.US", "200.00", "40")}, got.ByInstrument.Holdings)
	assert.Equal(t, cashPart("300.00", "60"), got.ByInstrument.Cash)
	assert.Equal(t, []allocationCurrencyDTO{{Currency: "USD", Value: "500.00", Share: "100"}}, got.ByQuoteCurrency)
}

// TestAllocation_CashZero_NoCashPart checks that the cash is not a part when it is zero.
//
//	Cash:  200.00 − 100.00 − 100.00 = 0.00
//	Total: 300.00 + 100.00 = 400.00
func TestAllocation_CashZero_NoCashPart(t *testing.T) {
	t.Parallel()
	holdings := []allocationHolding{
		{ticker: "AAA.US", cost: "100.00", price: "300.00"},
		{ticker: "BBB.US", cost: "100.00", price: "100.00"},
	}
	srv, ids := startAllocationServer(t, holdings, allocationExport("57000006", "USD", "200.00", holdings...))

	got := srv.accountAllocation(t, ids[0])

	assert.Nil(t, got.ByInstrument.Cash)
	assert.Equal(t, ptr("400.00"), got.Total)
	assert.Equal(t, []allocationPart{part("AAA.US", "300.00", "75"), part("BBB.US", "100.00", "25")}, got.ByInstrument.Holdings)
}

// TestAllocation_AllAccounts_SameInstrumentOnTwoAccounts_IsOnePartInPLN checks rules 4, 5, 6 and 7.
// The latest Exchange Rate of USD is 4.00. ABC.US has the price 100.00 USD.
//
//	USD Account: ABC.US 1 × 100.00 × 4.00 = 400.00 PLN, cash (250.00 − 100.00) × 4.00 = 600.00 PLN
//	PLN Account: ABC.US 1 × 100.00 × 4.00 = 400.00 PLN, cost 400.00 PLN, cash 600.00 − 400.00 = 200.00 PLN
//	ABC.US:      400.00 + 400.00 = 800.00 PLN
//	Cash:        600.00 + 200.00 = 800.00 PLN
//	Total:       1600.00 PLN
//	USD:         800.00 + 600.00 of USD cash = 1400.00 PLN, PLN: 200.00 PLN of PLN cash
func TestAllocation_AllAccounts_SameInstrumentOnTwoAccounts_IsOnePartInPLN(t *testing.T) {
	t.Parallel()
	onUSD := allocationHolding{ticker: "ABC.US", cost: "100.00", price: "100.00"}
	onPLN := allocationHolding{ticker: "ABC.US", cost: "400.00", price: "100.00"}
	srv, _ := startAllocationServer(t, []allocationHolding{onUSD},
		allocationExport("57000007", "USD", "250.00", onUSD),
		allocationExport("57000008", "PLN", "600.00", onPLN),
	)

	got := srv.allAllocation(t)

	assert.Equal(t, allocationResponse{
		Available: true, Warnings: []string{}, Currency: "PLN", Total: ptr("1600.00"),
		ByInstrument: allocationByInstrument{
			Holdings: []allocationPart{part("ABC.US", "800.00", "50")},
			Cash:     cashPart("800.00", "50"),
		},
		ByQuoteCurrency: []allocationCurrencyDTO{
			{Currency: "USD", Value: "1400.00", Share: "87.5"},
			{Currency: "PLN", Value: "200.00", Share: "12.5"},
		},
	}, got)
}

// TestAllocation_AllAccounts_MissingExchangeRate_NotAvailable checks rule 8.
func TestAllocation_AllAccounts_MissingExchangeRate_NotAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.importAccount(t, depositOnlyExport("57000009", "PLN"), "XTB PLN")
	srv.refreshPrices(t)

	status, _, body := srv.get(t, "/api/all-accounts/allocation")

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"available": false, "warnings": ["No Exchange Rate for USD. Refresh the prices."], "currency": "PLN",
		"total": null, "byInstrument": {"holdings": [], "cash": null, "other": null}, "byQuoteCurrency": [],
		"unpricedHoldings": 0
	}`, body)
}

// TestAllocation_PLNAccountWithUSDHolding_ByQuoteCurrency checks rule 7 on one Account.
//
//	ABC.US: 2 × 100.00 USD × 4.00 = 800.00 PLN, quoted in USD
//	Cash:   1000.00 − 800.00 = 200.00 PLN
func TestAllocation_PLNAccountWithUSDHolding_ByQuoteCurrency(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "100.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.00")
	id := srv.importAccount(t, plnExport(), "XTB PLN")
	srv.refreshPrices(t)

	got := srv.accountAllocation(t, id)

	assert.Equal(t, "PLN", got.Currency)
	assert.Equal(t, ptr("1000.00"), got.Total)
	assert.Equal(t, []allocationCurrencyDTO{
		{Currency: "USD", Value: "800.00", Share: "80"},
		{Currency: "PLN", Value: "200.00", Share: "20"},
	}, got.ByQuoteCurrency)
}

// TestAllocation_NoHoldingsAndNoCash_ZeroTotalAndEmptyLists checks an Account with a Deposit and an equal Withdrawal.
func TestAllocation_NoHoldingsAndNoCash_ZeroTotalAndEmptyLists(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "57000010", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		{Type: "Withdrawal", Amount: "-250.00", Time: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC), ID: "5700001002", Comment: "Withdrawal"},
		cashDeposit("5700001001", "250.00", time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)),
	}}, "XTB PLN")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/allocation", id))

	require.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{
		"available": true, "warnings": [], "currency": "PLN", "total": "0.00",
		"byInstrument": {"holdings": [], "cash": null, "other": null}, "byQuoteCurrency": [], "unpricedHoldings": 0
	}`, body)
}

func TestAllocation_UnknownAccount_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{"/api/accounts/999/allocation", "/api/accounts/abc/allocation"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			var got errorBody
			status := srv.getJSON(t, path, &got)

			assert.Equal(t, http.StatusNotFound, status)
			assert.NotEmpty(t, got.Error)
		})
	}
}
