package httpapi_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// usdIKZEExport is an export of an IKZE Account in USD with one Deposit.
func usdIKZEExport() xtbtest.Export {
	return xtbtest.Export{AccountNumber: "57000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		ikzeDeposit("8131", "1000.00", time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)),
	}}
}

// allocationResult is the result of the function get_allocation_and_net_deposits.
type allocationResult struct {
	Scope       string                    `json:"scope"`
	Currency    string                    `json:"currency"`
	NetDeposits *string                   `json:"netDeposits"`
	Allocation  allocationResponse        `json:"allocation"`
	Accounts    []allocationAccountResult `json:"accounts"`
	Error       string                    `json:"error"`
}

// allocationAccountResult is the part of the result for one Account.
type allocationAccountResult struct {
	AccountID   int64       `json:"accountId"`
	AccountName string      `json:"accountName"`
	Type        string      `json:"type"`
	Currency    string      `json:"currency"`
	NetDeposits string      `json:"netDeposits"`
	IKZE        *ikzeResult `json:"ikze"`
}

// ikzeResult is the IKZE data of one year in the result.
type ikzeResult struct {
	Year          int     `json:"year"`
	Contributions *string `json:"contributions"`
	Limit         *string `json:"limit"`
	Unused        *string `json:"unused"`
	OverLimit     *string `json:"overLimit"`
	Note          *string `json:"note"`
}

// askAllocation asks one question. The fake model calls get_allocation_and_net_deposits with the arguments once.
// It returns the result as the model got it.
func askAllocation(t *testing.T, srv *testServer, model *fakeModel, arguments string) allocationResult {
	t.Helper()

	_, content := askWithStep(t, srv, model, step("get_allocation_and_net_deposits", arguments))
	var got allocationResult
	decode(t, content, &got)
	return got
}

// startAllocationAssistant starts the backend with the fake model, the USD prices of the Holdings and the
// Exchange Rates, and refreshes the prices.
func startAllocationAssistant(t *testing.T) (*testServer, *fakeModel) {
	t.Helper()

	provider := newFakeProvider()
	provider.set(t, "AAA", "120.00", "USD", "2026-10-22")
	provider.set(t, "BBB", "40.00", "USD", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "4.00")
	return srv, model
}

// allocationHoldings are the Holdings of the Accounts in the tests of the Allocation.
var allocationHoldings = []allocationHolding{{"AAA.US", "100.00", "120.00"}, {"BBB.US", "50.00", "40.00"}}

func TestAskAssistant_Allocation_ForOneAccount_MatchesTheAllocationAndSummaryEndpoints(t *testing.T) {
	t.Parallel()
	srv, model := startAllocationAssistant(t)
	id := srv.importAccount(t, allocationExport("56000101", "USD", "1000.00", allocationHoldings...), "XTB USD")
	srv.refreshPrices(t)

	got := askAllocation(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	assert.Equal(t, `Account "XTB USD"`, got.Scope)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, srv.accountAllocation(t, id), got.Allocation)
	assert.Equal(t, ptr(srv.summary(t, id).NetDeposits), got.NetDeposits)
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, allocationAccountResult{
		AccountID: id, AccountName: "XTB USD", Type: "regular", Currency: "USD", NetDeposits: srv.summary(t, id).NetDeposits,
	}, got.Accounts[0], "an Account that is not IKZE has no IKZE data")
	assert.NotEmpty(t, got.Allocation.ByInstrument.Holdings)
}

func TestAskAssistant_Allocation_ForAllAccounts_MatchesTheAllAccountsEndpoints(t *testing.T) {
	t.Parallel()
	srv, model := startAllocationAssistant(t)
	usdID := srv.importAccount(t, allocationExport("56000101", "USD", "1000.00", allocationHoldings...), "XTB USD")
	plnID := srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")
	srv.refreshPrices(t)

	got := askAllocation(t, srv, model, `{}`)

	page := srv.allAccounts(t)
	assert.Equal(t, "all Accounts", got.Scope)
	assert.Equal(t, "PLN", got.Currency)
	assert.Equal(t, srv.allAllocation(t), got.Allocation)
	assert.Equal(t, page.NetDeposits, got.NetDeposits)
	require.Len(t, got.Accounts, 2)
	byID := map[int64]allocationAccountResult{}
	for _, acc := range got.Accounts {
		byID[acc.AccountID] = acc
		assert.Nil(t, acc.IKZE)
	}
	assert.Equal(t, srv.summary(t, usdID).NetDeposits, byID[usdID].NetDeposits)
	assert.Equal(t, "USD", byID[usdID].Currency)
	assert.Equal(t, srv.summary(t, plnID).NetDeposits, byID[plnID].NetDeposits)
}

func TestAskAssistant_Allocation_ForAllAccountsWithAMissingRate_GivesNoNetDepositsTotal(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, allocationExport("56000101", "USD", "1000.00"), "XTB USD")

	got := askAllocation(t, srv, model, `{}`)

	require.False(t, srv.allAccounts(t).Available)
	assert.Nil(t, got.NetDeposits, "the tool has no total in PLN without the Exchange Rate")
	assert.False(t, got.Allocation.Available)
	assert.NotEmpty(t, got.Allocation.Warnings)
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, "1000.00", got.Accounts[0].NetDeposits, "the Net Deposits of the Account stay in its currency")
}

func TestAskAssistant_Allocation_IKZEAccount_GivesTheIKZELimitOfTheYear(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		arguments string
		// wantYear is the year of the IKZE data. wantRow is the row of GET /api/accounts/{id}/ikze-limit for it, or nil.
		wantYear int
		wantRow  *ikzeLimitRow
		// wantEmpty is the result for a year that the endpoint has no row for.
		wantEmpty *ikzeResult
	}{
		{name: "no year means the current year", arguments: `{"accountId": %d}`, wantYear: 2026, wantRow: new(limitRow(2026, "3000.00", "16956.00", "13956.00", "0.00"))},
		{name: "a year with a row", arguments: `{"accountId": %d, "year": 2025}`, wantYear: 2025, wantRow: new(limitRow(2025, "16000.00", "15611.40", "0.00", "388.60"))},
		{
			name: "a year before the first contribution", arguments: `{"accountId": %d, "year": 2024}`, wantYear: 2024,
			wantEmpty: &ikzeResult{Year: 2024, Contributions: ptr("0.00")},
		},
		{
			name: "a year in the future", arguments: `{"accountId": %d, "year": 2027}`, wantYear: 2027,
			wantEmpty: &ikzeResult{Year: 2027, Contributions: ptr("0.00")},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model := startAssistantServer(t, newFakeProvider())
			id := srv.importIKZE(t,
				ikzeDeposit("8102", "3000.00", time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)),
				ikzeDeposit("8101", "16000.00", time.Date(2025, 3, 3, 10, 0, 0, 0, time.UTC)),
			)

			got := askAllocation(t, srv, model, fmt.Sprintf(tc.arguments, id))

			require.Len(t, got.Accounts, 1)
			assert.Equal(t, "IKZE", got.Accounts[0].Type)
			require.NotNil(t, got.Accounts[0].IKZE)
			want := tc.wantEmpty
			if tc.wantRow != nil {
				want = &ikzeResult{
					Year: tc.wantRow.Year, Contributions: &tc.wantRow.Contributions, Limit: tc.wantRow.Limit,
					Unused: tc.wantRow.Unused, OverLimit: tc.wantRow.OverLimit,
				}
				var endpoint *ikzeLimitRow
				for _, row := range srv.ikzeLimit(t, id).Rows {
					if row.Year == tc.wantYear {
						endpoint = &row
					}
				}
				require.NotNil(t, endpoint, "the IKZE Limit endpoint has a row for the year")
				assert.Equal(t, *tc.wantRow, *endpoint)
			}
			assert.Equal(t, want, got.Accounts[0].IKZE)
		})
	}
}

func TestAskAssistant_Allocation_IKZEYearWithKnownLimitAndNoContribution_GivesTheWholeLimitUnused(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importIKZE(t, ikzeDeposit("8111", "1000.00", time.Date(2025, 3, 3, 10, 0, 0, 0, time.UTC)))
	srv.clock.set(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC))

	got := askAllocation(t, srv, model, fmt.Sprintf(`{"accountId": %d, "year": 2026}`, id))

	require.Len(t, got.Accounts, 1)
	assert.Equal(t, &ikzeResult{
		Year: 2026, Contributions: ptr("0.00"), Limit: ptr("16956.00"), Unused: ptr("16956.00"), OverLimit: ptr("0.00"),
	}, got.Accounts[0].IKZE)
}

func TestAskAssistant_Allocation_AccountsOfAllAccounts_HaveIKZEDataOnlyForIKZE(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	ikzeID := srv.importIKZE(t, ikzeDeposit("8121", "2000.00", time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC)))
	plnID := srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")

	got := askAllocation(t, srv, model, `{"year": 2026}`)

	byID := map[int64]allocationAccountResult{}
	for _, acc := range got.Accounts {
		byID[acc.AccountID] = acc
	}
	require.Len(t, byID, 2)
	assert.Nil(t, byID[plnID].IKZE)
	require.NotNil(t, byID[ikzeID].IKZE)
	assert.Equal(t, ptr("2000.00"), byID[ikzeID].IKZE.Contributions)
}

func TestAskAssistant_Allocation_IKZEAccountNotInPLN_HasANoteAndNoNumbers(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.confirm(t, srv.preview(t, usdIKZEExport()).ID, newAccountBody("XTB IKZE USD", "IKZE", "USD")).AccountID

	got := askAllocation(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, id))

	require.Len(t, got.Accounts, 1)
	require.NotNil(t, got.Accounts[0].IKZE)
	assert.Nil(t, got.Accounts[0].IKZE.Contributions)
	assert.Nil(t, got.Accounts[0].IKZE.Limit)
	require.NotNil(t, got.Accounts[0].IKZE.Note)
	assert.Contains(t, *got.Accounts[0].IKZE.Note, "needs an Account in PLN")
}

func TestAskAssistant_Allocation_ProblemWithTheArguments_GivesTheModelAnError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{name: "unknown Account", arguments: `{"accountId": 999}`, want: "unknown Account 999"},
		{name: "year that is not valid", arguments: `{"year": 1800}`, want: "year"},
		{name: "year of the wrong type", arguments: `{"year": "last"}`, want: "arguments are not valid"},
		{name: "an argument of another function", arguments: `{"instrument": "AAA.US"}`, want: "arguments are not valid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model := startAssistantServer(t, newFakeProvider())
			srv.importAccount(t, dashboardExport(), "XTB USD")

			_, content := askWithStep(t, srv, model, step("get_allocation_and_net_deposits", tc.arguments))

			var got struct {
				Error string `json:"error"`
			}
			decode(t, content, &got)
			assert.Contains(t, got.Error, tc.want)
		})
	}
}

func TestAskAssistant_Allocation_NoBrokerCommentAndNoAccountNumberGoToTheModel(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	pln := adr0003PLNExport()
	srv.importWithWarnings(t, pln, "XTB PLN", "regular")
	srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")

	got := askAllocation(t, srv, model, `{}`)

	require.Len(t, got.Accounts, 2)
	for _, call := range model.recordedCalls() {
		for _, m := range call {
			for _, private := range []string{plnAccountNumber, ikzeAccountNumber, "Transfer out operation", "Transfer in operation"} {
				assert.NotContains(t, m.Content, private, "a %s message", m.Role)
			}
		}
	}
}

// checkAllocationAccountsMatchPages checks that each Account of the result has the Net Deposits of its summary.
// An IKZE Account in PLN has the IKZE data of the current year, as the IKZE Limit endpoint gives it.
// Another Account has no IKZE data.
// The failure messages show only names and amounts, so that the real-export tests can use the check.
func checkAllocationAccountsMatchPages(t *testing.T, srv *testServer, got allocationResult) {
	t.Helper()

	for _, acc := range got.Accounts {
		assert.Equal(t, srv.fullSummary(t, acc.AccountID).NetDeposits, acc.NetDeposits, "Account %d: Net Deposits", acc.AccountID)
		if acc.Type != "IKZE" {
			assert.Nil(t, acc.IKZE, "Account %d is not IKZE", acc.AccountID)
			continue
		}
		require.NotNil(t, acc.IKZE, "Account %d is IKZE", acc.AccountID)
		if acc.Currency != "PLN" {
			assert.NotNil(t, acc.IKZE.Note, "Account %d", acc.AccountID)
			continue
		}
		var endpoint ikzeLimitResponse
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/ikze-limit", acc.AccountID), &endpoint)
		for _, row := range endpoint.Rows {
			if row.Year == acc.IKZE.Year {
				assert.Equal(t, ikzeResult{
					Year: row.Year, Contributions: &row.Contributions, Limit: row.Limit, Unused: row.Unused, OverLimit: row.OverLimit,
				}, *acc.IKZE, "Account %d: IKZE Limit", acc.AccountID)
			}
		}
	}
}
