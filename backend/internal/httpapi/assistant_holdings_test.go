package httpapi_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// holdingsResult is the result of the function get_holdings.
type holdingsResult struct {
	Scope      string                  `json:"scope"`
	Instrument string                  `json:"instrument"`
	Accounts   []holdingsAccountResult `json:"accounts"`
	Error      string                  `json:"error"`
}

// holdingsAccountResult is the part of the get_holdings result for one Account.
type holdingsAccountResult struct {
	AccountID   int64           `json:"accountId"`
	AccountName string          `json:"accountName"`
	Currency    string          `json:"currency"`
	Holdings    []holdingResult `json:"holdings"`
}

// holdingResult is one Holding in the get_holdings result.
type holdingResult struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	Volume string `json:"volume"`
	// Currency is the currency of Cost, Value and UnrealizedProfit: the Account currency.
	Currency                string  `json:"currency"`
	Cost                    string  `json:"cost"`
	HasValue                bool    `json:"hasValue"`
	Price                   *string `json:"price"`
	QuoteCurrency           *string `json:"quoteCurrency"`
	PriceDate               *string `json:"priceDate"`
	Value                   *string `json:"value"`
	UnrealizedProfit        *string `json:"unrealizedProfit"`
	UnrealizedProfitPercent *string `json:"unrealizedProfitPercent"`
	NoValueReason           *string `json:"noValueReason"`
}

// step returns the step "call-1" of the model that calls the function with the arguments as JSON text.
func step(function, arguments string) assistant.Step {
	return assistant.Step{ID: "call-1", Function: function, Arguments: json.RawMessage(arguments)}
}

// functionResults returns the content of each function result message that the model got in its last call,
// in conversation order.
func (f *fakeModel) functionResults(t *testing.T) []assistant.Message {
	t.Helper()

	calls := f.recordedCalls()
	require.NotEmpty(t, calls, "the model got no call")
	var results []assistant.Message
	for _, m := range calls[len(calls)-1] {
		if m.Role == assistant.RoleFunction {
			results = append(results, m)
		}
	}
	return results
}

// startAssistantServer starts the backend with a fake model with no script and the price provider.
func startAssistantServer(t *testing.T, provider *fakeProvider) (*testServer, *fakeModel) {
	t.Helper()

	model := newFakeModel()
	return startServerWithOptions(t, serverOptions{provider: provider, model: model}), model
}

// askWithStep asks one question. The fake model makes the step once and then answers with a fixed text.
// It returns the answer and the content of the step result as the model got it.
func askWithStep(t *testing.T, srv *testServer, model *fakeModel, s assistant.Step) (answerResponse, string) {
	t.Helper()

	model.setScript(assistant.Reply{Steps: []assistant.Step{s}}, assistant.Reply{Text: "Here is the answer."})

	answer := srv.ask(t, question("What are my Holdings?"))

	results := model.functionResults(t)
	require.Len(t, results, 1)
	assert.Equal(t, s.ID, results[0].StepID)
	return answer, results[0].Content
}

// askHoldings asks one question. The fake model calls get_holdings with the arguments once.
// It returns the answer and the result of get_holdings as the model got it.
func askHoldings(t *testing.T, srv *testServer, model *fakeModel, arguments string) (answerResponse, holdingsResult) {
	t.Helper()

	answer, content := askWithStep(t, srv, model, step("get_holdings", arguments))
	var got holdingsResult
	decode(t, content, &got)
	return answer, got
}

func TestAskAssistant_Holdings_GivesTheModelTheHoldingsOfTheAccount(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	accountID := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)

	answer, got := askHoldings(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountID))

	assert.Equal(t, answerResponse{
		Answer: "Here is the answer.",
		Steps:  []answerStep{{Function: "get_holdings", Arguments: map[string]any{"accountId": float64(accountID)}}},
	}, answerResponse{Answer: answer.Answer, Steps: withoutResults(answer.Steps)})
	assert.Equal(t, holdingsResult{
		Scope: `Account "XTB USD"`,
		Accounts: []holdingsAccountResult{{
			AccountID: accountID, AccountName: "XTB USD", Currency: "USD",
			Holdings: []holdingResult{
				{
					Ticker: "ABC.US", Name: "ABC", Volume: "2", Currency: "USD", Cost: "200.00", HasValue: true,
					Price: ptr("125.5"), QuoteCurrency: ptr("USD"), PriceDate: ptr("2026-10-22"),
					Value: ptr("251.00"), UnrealizedProfit: ptr("51.00"), UnrealizedProfitPercent: ptr("25.5"),
				},
				{
					Ticker: "XYZ.US", Name: "XYZ", Volume: "0.5", Currency: "USD", Cost: "60.00", HasValue: false,
					QuoteCurrency: ptr("USD"), NoValueReason: ptr(noPriceNote),
				},
			},
		}},
	}, got)
	functions := model.recordedFunctions()
	require.NotEmpty(t, functions)
	var names []string
	for _, f := range functions[0] {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "get_holdings")
}

// checkHoldingsMatchEndpoint checks that each Account of the get_holdings result has the same Holdings, with the
// same numbers, as GET /api/accounts/{id}/holdings. If ticker is not empty, the result must hold only the
// Holding of that Instrument.
// The failure messages show only tickers and amounts, so that the real-export tests can use the check.
func checkHoldingsMatchEndpoint(t *testing.T, srv *testServer, got holdingsResult, ticker string) {
	t.Helper()

	for _, acc := range got.Accounts {
		var endpoint []holdingPrices
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", acc.AccountID), &endpoint)
		var basics []holdingResponse
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", acc.AccountID), &basics)

		var want []holdingResult
		for i, row := range endpoint {
			if ticker != "" && row.Ticker != ticker {
				continue
			}
			want = append(want, holdingResult{
				Ticker: row.Ticker, Name: basics[i].Name, Volume: basics[i].Volume, Currency: acc.Currency,
				Cost: cents(t, basics[i].Cost), HasValue: row.Value != nil, Price: row.Price, QuoteCurrency: row.QuoteCurrency,
				PriceDate: row.PriceDate, Value: centsPointer(t, row.Value), UnrealizedProfit: centsPointer(t, row.UnrealizedProfit),
				UnrealizedProfitPercent: row.UnrealizedProfitPercent, NoValueReason: row.Note,
			})
		}
		assert.Len(t, acc.Holdings, len(want), "Account %d: number of Holdings", acc.AccountID)
		for i := range min(len(want), len(acc.Holdings)) {
			assert.Equal(t, want[i], acc.Holdings[i], "Account %d: Holding %s", acc.AccountID, want[i].Ticker)
		}
	}
}

func TestAskAssistant_HoldingsOfAllAccounts_MatchTheHoldingsEndpoint(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-21")
	provider.set(t, "AAPL", "180.10", "USD", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	pricedID := srv.importAccount(t, pricedExport(), "XTB USD")
	dashboardID := srv.importAccount(t, dashboardExport(), "XTB Dashboard")
	plnID := srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")
	srv.refreshPrices(t)

	_, got := askHoldings(t, srv, model, `{}`)

	assert.Equal(t, "all Accounts", got.Scope)
	var ids []int64
	for _, acc := range got.Accounts {
		ids = append(ids, acc.AccountID)
	}
	assert.ElementsMatch(t, []int64{pricedID, dashboardID, plnID}, ids)
	checkHoldingsMatchEndpoint(t, srv, got, "")
}

func TestAskAssistant_HoldingsOfOneInstrument_GivesOnlyThatHolding(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	accountID := srv.importAccount(t, dashboardExport(), "XTB USD")

	// The match of the ticker ignores the case of the letters.
	_, got := askHoldings(t, srv, model, fmt.Sprintf(`{"accountId": %d, "instrument": "aapl.us"}`, accountID))

	assert.Equal(t, "AAPL.US", got.Instrument)
	require.Len(t, got.Accounts, 1)
	require.Len(t, got.Accounts[0].Holdings, 1)
	assert.Equal(t, "AAPL.US", got.Accounts[0].Holdings[0].Ticker)
	assert.Equal(t, "3.5", got.Accounts[0].Holdings[0].Volume)
	assert.Equal(t, "612.30", got.Accounts[0].Holdings[0].Cost)
	checkHoldingsMatchEndpoint(t, srv, got, "AAPL.US")
}

func TestAskAssistant_StepWithAProblem_GivesTheModelAnErrorAndTheLoopContinues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		step assistant.Step
		// wantError are the parts of the error text that the model gets.
		wantError []string
	}{
		{
			name:      "unknown Account",
			step:      step("get_holdings", `{"accountId": 999}`),
			wantError: []string{"unknown Account 999", `(\"XTB USD\")`},
		},
		{
			name:      "unknown Instrument",
			step:      step("get_holdings", `{"instrument": "NVDA"}`),
			wantError: []string{`unknown Instrument \"NVDA\"`, "AAPL.US", "MSFT.US"},
		},
		{
			name:      "unknown function",
			step:      step("get_weather", `{}`),
			wantError: []string{`unknown function \"get_weather\"`, "get_holdings"},
		},
		{
			name:      "arguments that are not JSON",
			step:      step("get_holdings", `{"accountId": `),
			wantError: []string{"not a valid JSON object"},
		},
		{
			name:      "an argument of the wrong type",
			step:      step("get_holdings", `{"accountId": "one"}`),
			wantError: []string{"arguments are not valid"},
		},
		{
			name:      "an unknown argument",
			step:      step("get_holdings", `{"from": "2025-01-01"}`),
			wantError: []string{"arguments are not valid", "from"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model := startAssistantServer(t, newFakeProvider())
			srv.importAccount(t, dashboardExport(), "XTB USD")

			answer, content := askWithStep(t, srv, model, tc.step)

			var got struct {
				Error string `json:"error"`
			}
			decode(t, content, &got)
			for _, want := range tc.wantError {
				assert.Contains(t, content, want)
			}
			assert.NotEmpty(t, got.Error)
			assert.Equal(t, "Here is the answer.", answer.Answer, "the loop must continue after an error result")
			require.Len(t, answer.Steps, 1)
			assert.Equal(t, tc.step.Function, answer.Steps[0].Function)
		})
	}
}

func TestAskAssistant_NoBrokerCommentAndNoAccountNumberGoToTheModel(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	pln := adr0003PLNExport()
	pln.CashOperations = append(pln.CashOperations, xtbtest.CashOperation{
		Type: "Stock purchase", Ticker: "CDR.PL", Instrument: "CD Projekt", Time: adr0003TransferOutTime.Add(time.Hour),
		Amount: "-500.00", ID: "3002", Comment: "OPEN BUY 2 @ 250.00", PositionID: "3000000901",
	}, xtbtest.CashOperation{
		Type: "Deposit", Time: adr0003TransferOutTime.Add(-time.Minute), Amount: "600.00", ID: "3003", Comment: "Blik deposit",
	})
	plnID, _ := srv.importWithWarnings(t, pln, "XTB PLN", "regular")
	srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")
	require.Len(t, srv.holdings(t, plnID), 1)

	_, got := askHoldings(t, srv, model, `{}`)

	require.Len(t, got.Accounts, 2)
	for _, call := range model.recordedCalls() {
		for _, m := range call {
			for _, private := range []string{plnAccountNumber, ikzeAccountNumber, "Transfer out operation", "Transfer in operation", "Blik deposit", "OPEN BUY"} {
				assert.NotContains(t, m.Content, private, "a %s message", m.Role)
			}
		}
	}
}
