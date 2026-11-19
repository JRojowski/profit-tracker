package httpapi_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// returnResult is one return in the result of the function get_results.
type returnResult struct {
	Percent *string `json:"percent"`
	Reason  *string `json:"reason"`
}

// resultsResult is the result of the function get_results.
type resultsResult struct {
	Scope       string   `json:"scope"`
	AccountID   *int64   `json:"accountId"`
	AccountName string   `json:"accountName"`
	Currency    string   `json:"currency"`
	Available   bool     `json:"available"`
	Warnings    []string `json:"warnings"`

	Value            *string `json:"value"`
	Cash             *string `json:"cash"`
	NetDeposits      *string `json:"netDeposits"`
	TotalResult      *string `json:"totalResult"`
	RealizedProfit   *string `json:"realizedProfit"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	NetDividends     *string `json:"netDividends"`
	NetInterest      *string `json:"netInterest"`
	Fees             *string `json:"fees"`
	CurrencyResult   *string `json:"currencyResult"`
	PriceDate        *string `json:"priceDate"`
	UnpricedHoldings int     `json:"unpricedHoldings"`

	SimpleReturn       returnResult            `json:"simpleReturn"`
	AnnualReturn       returnResult            `json:"annualReturn"`
	TimeWeightedReturn map[string]returnResult `json:"timeWeightedReturn"`
	ValuedAtCost       []string                `json:"valuedAtCost"`
	Error              string                  `json:"error"`
}

// pageSummary is the body of GET /api/accounts/{id}/summary, with all fields that get_results gives.
type pageSummary struct {
	Cash             string  `json:"cash"`
	NetDeposits      string  `json:"netDeposits"`
	RealizedProfit   string  `json:"realizedProfit"`
	NetDividends     string  `json:"netDividends"`
	NetInterest      string  `json:"netInterest"`
	Fees             string  `json:"fees"`
	UnrealizedProfit string  `json:"unrealizedProfit"`
	TotalResult      string  `json:"totalResult"`
	Value            string  `json:"value"`
	PriceDate        *string `json:"priceDate"`
	UnpricedHoldings int     `json:"unpricedHoldings"`
	allReturns
}

// fullSummary returns the summary of the Account with all fields.
func (s *testServer) fullSummary(t *testing.T, accountID int64) pageSummary {
	t.Helper()

	var got pageSummary
	s.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountID), &got)
	return got
}

// askResults asks one question. The fake model calls get_results with the arguments once.
// It returns the result of get_results as the model got it.
func askResults(t *testing.T, srv *testServer, model *fakeModel, arguments string) resultsResult {
	t.Helper()

	_, content := askWithStep(t, srv, model, step("get_results", arguments))
	var got resultsResult
	decode(t, content, &got)
	return got
}

// twrPeriods are the periods of the Time-Weighted Return, in the names of the Value History endpoint.
var twrPeriods = []string{"1M", "3M", "YTD", "1Y", "All"}

// twrByPeriod returns the Time-Weighted Returns of the Value History endpoint by period.
func twrByPeriod(r timeWeightedReturns) map[string]*string {
	return map[string]*string{"1M": r.OneMonth, "3M": r.ThreeMonths, "YTD": r.YearToDate, "1Y": r.OneYear, "All": r.All}
}

// checkTimeWeightedMatches checks that each period of the result has the number of the endpoint,
// or no number and a reason if the endpoint has none.
func checkTimeWeightedMatches(t *testing.T, got map[string]returnResult, want timeWeightedReturns) {
	t.Helper()

	require.Len(t, got, len(twrPeriods))
	for period, wantPercent := range twrByPeriod(want) {
		g, ok := got[period]
		require.True(t, ok, "period %s", period)
		assert.Equal(t, wantPercent, g.Percent, "period %s", period)
		assert.Equal(t, wantPercent == nil, g.Reason != nil, "period %s: a rate with no number has a reason", period)
	}
}

// checkAccountResultsMatchPages checks that the get_results result for the Account has the same numbers as
// GET /api/accounts/{id}/summary and GET /api/accounts/{id}/value-history.
// The failure messages show only names and amounts, so that the real-export tests can use the check.
func checkAccountResultsMatchPages(t *testing.T, srv *testServer, got resultsResult, id int64) {
	t.Helper()

	summary := srv.fullSummary(t, id)
	assert.Equal(t, got.Scope, fmt.Sprintf("Account %q", got.AccountName))
	require.NotNil(t, got.AccountID)
	assert.Equal(t, id, *got.AccountID)
	assert.True(t, got.Available)
	for name, pair := range map[string]struct {
		want string
		got  *string
	}{
		"value": {summary.Value, got.Value}, "cash": {summary.Cash, got.Cash},
		"netDeposits": {summary.NetDeposits, got.NetDeposits}, "totalResult": {summary.TotalResult, got.TotalResult},
		"realizedProfit": {summary.RealizedProfit, got.RealizedProfit}, "unrealizedProfit": {summary.UnrealizedProfit, got.UnrealizedProfit},
		"netDividends": {summary.NetDividends, got.NetDividends}, "netInterest": {summary.NetInterest, got.NetInterest},
		"fees": {summary.Fees, got.Fees},
	} {
		require.NotNil(t, pair.got, name)
		assert.Equal(t, pair.want, *pair.got, name)
	}
	assert.Nil(t, got.CurrencyResult, "the tool calculates the Currency Result only for all Accounts")
	assert.Equal(t, summary.PriceDate, got.PriceDate)
	assert.Equal(t, summary.UnpricedHoldings, got.UnpricedHoldings)
	assert.Equal(t, returnResult{Percent: summary.SimpleReturn, Reason: summary.SimpleReturnNote}, got.SimpleReturn)
	assert.Equal(t, returnResult{Percent: summary.AnnualReturn, Reason: summary.AnnualReturnNote}, got.AnnualReturn)
	var history valueHistoryResponse
	srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/value-history", id), &history)
	checkTimeWeightedMatches(t, got.TimeWeightedReturn, history.TimeWeightedReturn)
}

func TestAskAssistant_Results_GiveTheModelTheNumbersOfTheAccountPage(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)

	got := askResults(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	checkAccountResultsMatchPages(t, srv, got, id)
	assert.Equal(t, `Account "XTB USD"`, got.Scope)
	assert.Equal(t, "XTB USD", got.AccountName)
	assert.Equal(t, "USD", got.Currency)
}

func TestAskAssistant_Results_ReturnThatIsNotAvailableHasAReasonAndNoNumber(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	// The first deposit is 30 days old: the Annual Return is not available. The Account has no Holding.
	id := srv.importAccount(t, interestExport(daysBeforeToday(30)), "XTB USD")

	got := askResults(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	assertPercent(t, "10", got.SimpleReturn.Percent)
	assert.Nil(t, got.SimpleReturn.Reason)
	assert.Nil(t, got.AnnualReturn.Percent)
	require.NotNil(t, got.AnnualReturn.Reason)
	assert.Equal(t, "The first deposit is less than one year old.", *got.AnnualReturn.Reason)
}

func TestAskAssistant_Results_TimeWeightedReturnWithNoValueHistoryHasAReason(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	// An Account with no Cash Operation has no Value History. The summary has no deposit, so no Simple Return.
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "56000077", Currency: "USD"}, "XTB Empty")

	got := askResults(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	require.Len(t, got.TimeWeightedReturn, len(twrPeriods))
	for period, r := range got.TimeWeightedReturn {
		assert.Nil(t, r.Percent, period)
		require.NotNil(t, r.Reason, period)
		assert.Equal(t, "The Value History has no day with a value before it.", *r.Reason, period)
	}
	assert.NotNil(t, got.SimpleReturn.Reason)
	assert.Nil(t, got.SimpleReturn.Percent)
}

// checkAllResultsMatchPages checks that the get_results result for all Accounts has the same numbers as
// GET /api/all-accounts/summary and GET /api/all-accounts/value-history.
// The failure messages show only names and amounts, so that the real-export tests can use the check.
func checkAllResultsMatchPages(t *testing.T, srv *testServer, got resultsResult) {
	t.Helper()

	page := srv.allAccounts(t)
	assert.Equal(t, "all Accounts", got.Scope)
	assert.Nil(t, got.AccountID)
	assert.Equal(t, "PLN", got.Currency)
	assert.Equal(t, page.Available, got.Available)
	assert.Equal(t, page.Warnings, got.Warnings)
	assert.Equal(t, page.Value, got.Value)
	assert.Equal(t, page.Cash, got.Cash)
	assert.Equal(t, page.NetDeposits, got.NetDeposits)
	assert.Equal(t, page.TotalResult, got.TotalResult)
	assert.Equal(t, page.RealizedProfit, got.RealizedProfit)
	assert.Equal(t, page.UnrealizedProfit, got.UnrealizedProfit)
	assert.Equal(t, page.NetDividends, got.NetDividends)
	assert.Equal(t, page.NetInterest, got.NetInterest)
	assert.Equal(t, page.Fees, got.Fees)
	assert.Equal(t, page.CurrencyResult, got.CurrencyResult)
	assert.Equal(t, returnResult{Percent: page.SimpleReturn, Reason: page.SimpleReturnNote}, got.SimpleReturn)
	assert.Equal(t, returnResult{Percent: page.AnnualReturn, Reason: page.AnnualReturnNote}, got.AnnualReturn)
	history := srv.allValueHistory(t)
	if history.Available {
		checkTimeWeightedMatches(t, got.TimeWeightedReturn, history.TimeWeightedReturn)
		return
	}
	for period, r := range got.TimeWeightedReturn {
		assert.Nil(t, r.Percent, period)
		assert.NotNil(t, r.Reason, period)
	}
}

func TestAskAssistant_Results_ForAllAccountsGiveThePLNTotalsOfThePage(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")
	srv.refreshPrices(t)

	got := askResults(t, srv, model, `{}`)

	checkAllResultsMatchPages(t, srv, got)
	require.True(t, got.Available)
}

func TestAskAssistant_Results_ForAllAccountsWithAMissingRateGiveReasonsAndNoNumbers(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	// No Exchange Rate for USD: the page cannot total the Accounts in PLN.
	srv.importAccount(t, adr0004Export(), "XTB USD")

	got := askResults(t, srv, model, `{}`)

	page := srv.allAccounts(t)
	require.False(t, page.Available)
	assert.False(t, got.Available)
	assert.Equal(t, page.Warnings, got.Warnings)
	assert.NotEmpty(t, got.Warnings)
	for name, v := range map[string]*string{
		"value": got.Value, "cash": got.Cash, "netDeposits": got.NetDeposits, "totalResult": got.TotalResult,
		"currencyResult": got.CurrencyResult, "simpleReturn": got.SimpleReturn.Percent, "annualReturn": got.AnnualReturn.Percent,
	} {
		assert.Nil(t, v, name)
	}
	assert.NotNil(t, got.SimpleReturn.Reason)
	for period, r := range got.TimeWeightedReturn {
		assert.Nil(t, r.Percent, period)
		assert.NotNil(t, r.Reason, period)
	}
}

func TestAskAssistant_Results_NameTheInstrumentsValuedAtCost(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	// pricedExport holds Instruments with no stored price history: the Value History values them at cost.
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	got := askResults(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	var want []string
	for _, inst := range srv.valueHistory(t, id).ValuedAtCost {
		want = append(want, inst.Ticker)
	}
	require.NotEmpty(t, want)
	assert.Equal(t, want, got.ValuedAtCost)
}

func TestAskAssistant_Results_UnknownAccount_GivesTheModelAnError(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, dashboardExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_results", `{"accountId": 999}`))

	assert.Contains(t, content, "unknown Account 999")
}

func TestAskAssistant_Results_NoBrokerCommentAndNoAccountNumberGoToTheModel(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	pln := adr0003PLNExport()
	pln.CashOperations = append(pln.CashOperations, xtbtest.CashOperation{
		Type: "Deposit", Time: adr0003TransferOutTime.Add(-time.Minute), Amount: "600.00", ID: "3003", Comment: "Blik deposit",
	})
	srv.importWithWarnings(t, pln, "XTB PLN", "regular")
	srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")

	got := askResults(t, srv, model, `{}`)

	assert.True(t, got.Available)
	for _, call := range model.recordedCalls() {
		for _, m := range call {
			for _, private := range []string{plnAccountNumber, ikzeAccountNumber, "Transfer out operation", "Transfer in operation", "Blik deposit"} {
				assert.NotContains(t, m.Content, private, "a %s message", m.Role)
			}
		}
	}
}
