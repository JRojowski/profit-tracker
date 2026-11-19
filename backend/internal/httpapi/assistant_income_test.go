package httpapi_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// incomeSums are the sums of the result of get_income_and_fees. NetInterest is nil when the filter leaves it out.
type incomeSums struct {
	Currency     string  `json:"currency"`
	NetDividends string  `json:"netDividends"`
	NetInterest  *string `json:"netInterest"`
	Fees         string  `json:"fees"`
}

// incomeAccountResult is the part of the get_income_and_fees result for one Account.
type incomeAccountResult struct {
	AccountID   int64  `json:"accountId"`
	AccountName string `json:"accountName"`
	incomeSums
}

// incomeResult is the result of the function get_income_and_fees.
type incomeResult struct {
	Scope      string                `json:"scope"`
	Instrument string                `json:"instrument"`
	From       string                `json:"from"`
	To         string                `json:"to"`
	Totals     []incomeSums          `json:"totals"`
	Accounts   []incomeAccountResult `json:"accounts"`
	LeftOut    []string              `json:"leftOut"`
	Error      string                `json:"error"`
}

// incomeExport is an export of a USD Account with income and fees on different days. The times are in UTC.
// Two Cash Operations are near the end of a UTC day, so that the day limits in Polish time matter:
// 2026-02-28 23:30 UTC is 2026-03-01 in Poland.
//
//	2026-02-27 ABC.US dividend 10.00, withholding tax -1.50
//	2026-02-28 XYZ.US dividend 20.00, withholding tax -3.00 (at 23:30 UTC, Polish day 2026-03-01)
//	2026-03-31 interest 4.00, interest tax -0.76
//	2026-04-10 ABC.US SEC fee -0.05
//	2026-04-12 Correction -0.20 (no Instrument)
//
//	Net dividends: 25.50 (ABC.US 8.50, XYZ.US 17.00). Net interest: 3.24. Fees: -0.25.
func incomeExport() xtbtest.Export {
	day := func(m time.Month, d, hour, minute, second int) time.Time {
		return time.Date(2026, m, d, hour, minute, second, 0, time.UTC)
	}
	return xtbtest.Export{
		AccountNumber: "56000041",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Deposit", Time: day(time.January, 5, 10, 0, 0), Amount: "1000.00", ID: "6001", Comment: "Blik deposit"},
			{Type: "Dividend", Ticker: "ABC.US", Instrument: "ABC", Time: day(time.February, 27, 10, 0, 0), Amount: "10.00", ID: "6002", Comment: "ABC.US USD 1.0000/ SHR"},
			{Type: "Withholding tax", Ticker: "ABC.US", Instrument: "ABC", Time: day(time.February, 27, 10, 0, 1), Amount: "-1.50", ID: "6003", Comment: "ABC.US USD WHT 15%"},
			{Type: "Dividend", Ticker: "XYZ.US", Instrument: "XYZ", Time: day(time.February, 28, 23, 30, 0), Amount: "20.00", ID: "6004", Comment: "XYZ.US USD 2.0000/ SHR"},
			{Type: "Withholding tax", Ticker: "XYZ.US", Instrument: "XYZ", Time: day(time.February, 28, 23, 30, 1), Amount: "-3.00", ID: "6005", Comment: "XYZ.US USD WHT 15%"},
			{Type: "Free funds interest", Time: day(time.March, 31, 10, 0, 0), Amount: "4.00", ID: "6006", Comment: "Free-funds Interest 2026-03"},
			{Type: "Free funds interest tax", Time: day(time.March, 31, 10, 0, 1), Amount: "-0.76", ID: "6007", Comment: "Free-funds Interest Tax 2026-03"},
			{Type: "SEC fee", Ticker: "ABC.US", Instrument: "ABC", Time: day(time.April, 10, 10, 0, 0), Amount: "-0.05", ID: "6008", Comment: "SEC fee ABC.US"},
			{Type: "Correction", Time: day(time.April, 12, 10, 0, 0), Amount: "-0.20", ID: "6009", Comment: "Correction of interest"},
		},
	}
}

// incomePLNExport is an export of a PLN Account with one dividend.
func incomePLNExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000042",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Deposit", Time: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), Amount: "500.00", ID: "6101", Comment: "Blik deposit"},
			{Type: "Dividend", Ticker: "PKN.PL", Instrument: "Orlen", Time: time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC), Amount: "126.00", ID: "6102", Comment: "PKN.PL PLN 4.2000/ SHR"},
			{Type: "Withholding tax", Ticker: "PKN.PL", Instrument: "Orlen", Time: time.Date(2026, 2, 20, 10, 0, 1, 0, time.UTC), Amount: "-23.94", ID: "6103", Comment: "PKN.PL PLN WHT 19%"},
		},
	}
}

// askIncome asks one question. The fake model calls get_income_and_fees with the arguments once.
// It returns the answer and the result of get_income_and_fees as the model got it.
func askIncome(t *testing.T, srv *testServer, model *fakeModel, arguments string) (answerResponse, incomeResult) {
	t.Helper()

	answer, content := askWithStep(t, srv, model, step("get_income_and_fees", arguments))
	var got incomeResult
	decode(t, content, &got)
	return answer, got
}

// accountPeriod returns the arguments of get_income_and_fees for an Account and a period.
func accountPeriod(accountID int64, from, to string) string {
	return fmt.Sprintf(`{"accountId": %d, "from": %q, "to": %q}`, accountID, from, to)
}

// usdSums returns the sums of a USD result with no left-out part.
func usdSums(dividends, interest, fees string) incomeSums {
	return incomeSums{Currency: "USD", NetDividends: dividends, NetInterest: &interest, Fees: fees}
}

// startIncomeServer starts the backend with a fake model and imports incomeExport. It returns the ID of the Account.
func startIncomeServer(t *testing.T) (*testServer, *fakeModel, int64) {
	t.Helper()

	srv, model := startAssistantServer(t, newFakeProvider())
	return srv, model, srv.importAccount(t, incomeExport(), "XTB USD")
}

// requireOneTotal returns the only sum of the result and checks that the Account row has the same sums.
func requireOneTotal(t *testing.T, got incomeResult) incomeSums {
	t.Helper()

	require.Len(t, got.Totals, 1)
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, got.Totals[0], got.Accounts[0].incomeSums)
	return got.Totals[0]
}

func TestAskAssistant_IncomeAndFees_NoFilter_MatchesTheSummaryEndpoint(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	accountID := srv.importAccount(t, incomeAndCostsExport(), "XTB USD")

	answer, got := askIncome(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountID))

	assert.Equal(t, []answerStep{{Function: "get_income_and_fees", Arguments: map[string]any{"accountId": float64(accountID)}}}, withoutResults(answer.Steps))
	assert.Equal(t, `Account "XTB USD"`, got.Scope)
	assert.Empty(t, got.LeftOut)
	assert.Equal(t, usdSums("10.62", "2.75", "-0.13"), requireOneTotal(t, got))
	assert.Equal(t, accountID, got.Accounts[0].AccountID)
	assert.Equal(t, "XTB USD", got.Accounts[0].AccountName)
	summary := srv.summary(t, accountID)
	assert.Equal(t, summary.NetDividends, got.Totals[0].NetDividends)
	assert.Equal(t, summary.NetInterest, *got.Totals[0].NetInterest)
	assert.Equal(t, summary.Fees, got.Totals[0].Fees)
}

func TestAskAssistant_IncomeAndFees_FilterByInstrument_GivesOnlyTheAmountsOfThatInstrument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		instrument string
		want       incomeSums
	}{
		// The match of the ticker ignores the case of the letters. The SEC fee has the ticker. The Correction has none.
		{instrument: "abc.us", want: incomeSums{Currency: "USD", NetDividends: "8.50", Fees: "-0.05"}},
		{instrument: "XYZ.US", want: incomeSums{Currency: "USD", NetDividends: "17.00", Fees: "0.00"}},
	}
	for _, tc := range tests {
		t.Run(tc.instrument, func(t *testing.T) {
			t.Parallel()
			srv, model, accountID := startIncomeServer(t)
			_, got := askIncome(t, srv, model, fmt.Sprintf(`{"accountId": %d, "instrument": %q}`, accountID, tc.instrument))

			assert.Equal(t, tc.want, requireOneTotal(t, got), "interest is not tied to an Instrument: it is null")
			assert.Equal(t, strings.ToUpper(tc.instrument), got.Instrument)
			require.Len(t, got.LeftOut, 2)
			assert.Contains(t, got.LeftOut[0], "net interest")
			assert.Contains(t, got.LeftOut[0], "does not tie interest to an Instrument")
			assert.Contains(t, got.LeftOut[1], "fees")
		})
	}
}

func TestAskAssistant_IncomeAndFees_Period_UsesPolishDaysAndIncludesBothLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		from, to string
		want     incomeSums
	}{
		// The XYZ.US dividend is at 2026-02-28 23:30 UTC. That is 2026-03-01 in Poland.
		{name: "the last UTC day of February", from: "2026-02-01", to: "2026-02-28", want: usdSums("8.50", "0.00", "0.00")},
		{name: "the first Polish day of March", from: "2026-03-01", to: "2026-03-31", want: usdSums("17.00", "3.24", "0.00")},
		{name: "one day, both limits included", from: "2026-02-27", to: "2026-02-27", want: usdSums("8.50", "0.00", "0.00")},
		{name: "the day of the fee and the Correction", from: "2026-04-10", to: "2026-04-12", want: usdSums("0.00", "0.00", "-0.25")},
		{name: "the whole year", from: "2026-01-01", to: "2026-12-31", want: usdSums("25.50", "3.24", "-0.25")},
		{name: "a period with no operation", from: "2025-01-01", to: "2025-12-31", want: usdSums("0.00", "0.00", "0.00")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model, accountID := startIncomeServer(t)

			_, got := askIncome(t, srv, model, accountPeriod(accountID, tc.from, tc.to))

			assert.Equal(t, tc.from, got.From)
			assert.Equal(t, tc.to, got.To)
			assert.Equal(t, tc.want, requireOneTotal(t, got))
			assert.Empty(t, got.LeftOut)
		})
	}
}

func TestAskAssistant_IncomeAndFees_OnlyFromOrOnlyTo_LimitsOneSide(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	accountID := srv.importAccount(t, incomeExport(), "XTB USD")

	_, from := askIncome(t, srv, model, fmt.Sprintf(`{"accountId": %d, "from": "2026-03-01"}`, accountID))
	_, to := askIncome(t, srv, model, fmt.Sprintf(`{"accountId": %d, "to": "2026-02-28"}`, accountID))

	assert.Equal(t, usdSums("17.00", "3.24", "-0.25"), requireOneTotal(t, from))
	assert.Equal(t, usdSums("8.50", "0.00", "0.00"), requireOneTotal(t, to))
}

func TestAskAssistant_IncomeAndFees_InstrumentAndPeriodTogether(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		instrument string
		from, to   string
		want       incomeSums
	}{
		{name: "dividends in the period", instrument: "ABC.US", from: "2026-02-01", to: "2026-02-28", want: incomeSums{Currency: "USD", NetDividends: "8.50", Fees: "0.00"}},
		{name: "fee in the period", instrument: "ABC.US", from: "2026-04-01", to: "2026-04-30", want: incomeSums{Currency: "USD", NetDividends: "0.00", Fees: "-0.05"}},
		{name: "the other Instrument has nothing in the period", instrument: "XYZ.US", from: "2026-02-01", to: "2026-02-28", want: incomeSums{Currency: "USD", NetDividends: "0.00", Fees: "0.00"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model, accountID := startIncomeServer(t)
			arguments := fmt.Sprintf(`{"accountId": %d, "instrument": %q, "from": %q, "to": %q}`, accountID, tc.instrument, tc.from, tc.to)

			_, got := askIncome(t, srv, model, arguments)

			assert.Equal(t, tc.want, requireOneTotal(t, got))
			assert.Equal(t, tc.instrument, got.Instrument)
			assert.Equal(t, tc.from, got.From)
			assert.Equal(t, tc.to, got.To)
			assert.NotEmpty(t, got.LeftOut)
		})
	}
}

func TestAskAssistant_IncomeAndFees_FollowingPeriods_AddToTheJoinedPeriod(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	accountID := srv.importAccount(t, incomeExport(), "XTB USD")

	periods := [][2]string{{"2026-01-01", "2026-02-28"}, {"2026-03-01", "2026-03-31"}, {"2026-04-01", "2026-12-31"}}
	sum := incomeSums{NetDividends: "0", Fees: "0"}
	interest := decimal.Zero
	for _, p := range periods {
		_, got := askIncome(t, srv, model, accountPeriod(accountID, p[0], p[1]))
		part := requireOneTotal(t, got)
		sum.NetDividends = decimal.RequireFromString(sum.NetDividends).Add(decimal.RequireFromString(part.NetDividends)).String()
		sum.Fees = decimal.RequireFromString(sum.Fees).Add(decimal.RequireFromString(part.Fees)).String()
		interest = interest.Add(decimal.RequireFromString(*part.NetInterest))
	}
	_, joined := askIncome(t, srv, model, accountPeriod(accountID, periods[0][0], periods[2][1]))

	want := requireOneTotal(t, joined)
	assert.Equal(t, want.NetDividends, decimal.RequireFromString(sum.NetDividends).StringFixed(2))
	assert.Equal(t, want.Fees, decimal.RequireFromString(sum.Fees).StringFixed(2))
	assert.Equal(t, *want.NetInterest, interest.StringFixed(2))
}

func TestAskAssistant_IncomeAndFees_AllAccounts_GivesOneSumPerCurrency(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	usdID := srv.importAccount(t, incomeExport(), "XTB USD")
	otherUSDID := srv.importAccount(t, incomeAndCostsExport(), "XTB USD 2")
	plnID := srv.importAccount(t, incomePLNExport(), "XTB PLN")

	_, got := askIncome(t, srv, model, `{}`)

	assert.Equal(t, "all Accounts", got.Scope)
	assert.ElementsMatch(t, []incomeSums{
		usdSums("36.12", "5.99", "-0.38"),
		{Currency: "PLN", NetDividends: "102.06", NetInterest: ptr("0.00"), Fees: "0.00"},
	}, got.Totals)
	require.Len(t, got.Accounts, 3)
	byID := map[int64]incomeAccountResult{}
	for _, acc := range got.Accounts {
		byID[acc.AccountID] = acc
	}
	assert.Equal(t, usdSums("25.50", "3.24", "-0.25"), byID[usdID].incomeSums)
	assert.Equal(t, usdSums("10.62", "2.75", "-0.13"), byID[otherUSDID].incomeSums)
	assert.Equal(t, "102.06", byID[plnID].NetDividends)
	assert.Equal(t, "PLN", byID[plnID].Currency)
}

func TestAskAssistant_IncomeAndFees_AllAccountsWithAnInstrument_GivesTheSumsOfThatInstrument(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, incomeExport(), "XTB USD")
	srv.importAccount(t, incomePLNExport(), "XTB PLN")

	_, got := askIncome(t, srv, model, `{"instrument": "PKN.PL"}`)

	assert.ElementsMatch(t, []incomeSums{
		{Currency: "USD", NetDividends: "0.00", Fees: "0.00"},
		{Currency: "PLN", NetDividends: "102.06", Fees: "0.00"},
	}, got.Totals)
}

func TestAskAssistant_IncomeAndFees_StepWithAProblem_GivesTheModelAnErrorAndTheLoopContinues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arguments string
		wantError []string
	}{
		{name: "unknown Account", arguments: `{"accountId": 999}`, wantError: []string{"unknown Account 999", `(\"XTB USD\")`}},
		{name: "unknown Instrument", arguments: `{"instrument": "NVDA"}`, wantError: []string{`unknown Instrument \"NVDA\"`, "ABC.US", "XYZ.US"}},
		{name: "a date that is not a date", arguments: `{"from": "yesterday"}`, wantError: []string{`from`, "YYYY-MM-DD"}},
		{name: "a day that does not exist", arguments: `{"to": "2026-02-30"}`, wantError: []string{`to`, "YYYY-MM-DD"}},
		{name: "the first day after the last day", arguments: `{"from": "2026-03-02", "to": "2026-03-01"}`, wantError: []string{"from", "after", "to"}},
		{name: "an unknown argument", arguments: `{"year": 2026}`, wantError: []string{"arguments are not valid", "year"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model := startAssistantServer(t, newFakeProvider())
			srv.importAccount(t, incomeExport(), "XTB USD")

			answer, content := askWithStep(t, srv, model, step("get_income_and_fees", tc.arguments))

			var got incomeResult
			decode(t, content, &got)
			assert.NotEmpty(t, got.Error)
			for _, want := range tc.wantError {
				assert.Contains(t, content, want)
			}
			assert.Empty(t, got.Totals)
			assert.Equal(t, "Here is the answer.", answer.Answer, "the loop must continue after an error result")
		})
	}
}

func TestAskAssistant_IncomeAndFees_NoBrokerCommentAndNoAccountNumberGoToTheModel(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, incomeExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_income_and_fees", `{}`))

	for _, private := range []string{"56000041", "Blik deposit", "WHT 15%", "Correction of interest", "SEC fee ABC.US", "Free-funds Interest", "SHR"} {
		assert.NotContains(t, content, private)
	}
	for _, call := range model.recordedCalls() {
		for _, m := range call {
			assert.NotContains(t, m.Content, "56000041", "a %s message", m.Role)
		}
	}
}

func TestAskAssistant_IncomeAndFees_IsOneOfTheFunctionsOfTheModel(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, incomeExport(), "XTB USD")

	askIncome(t, srv, model, `{}`)

	functions := model.recordedFunctions()
	require.NotEmpty(t, functions)
	var found *assistant.FunctionDefinition
	for _, f := range functions[0] {
		if f.Name == "get_income_and_fees" {
			found = &f
		}
	}
	require.NotNil(t, found)
	for _, argument := range []string{"accountId", "instrument", "from", "to"} {
		assert.Contains(t, string(found.Parameters), argument)
	}
}

// checkIncomeMatchesSummaries checks that each sum of the get_income_and_fees result with no filter is equal to
// the net dividends, the net interest and the fees of the page endpoint: per Account, and per currency for the
// Accounts together. The failure messages show only amounts, so that the real-export tests can use the check.
func checkIncomeMatchesSummaries(t *testing.T, srv *testServer, got incomeResult) {
	t.Helper()

	type totals struct{ dividends, interest, fees decimal.Decimal }
	byCurrency := map[string]totals{}
	for _, acc := range got.Accounts {
		summary := srv.summary(t, acc.AccountID)
		require.NotNil(t, acc.NetInterest, "Account %d: net interest", acc.AccountID)
		assert.Equal(t, summary.Currency, acc.Currency, "Account %d: currency", acc.AccountID)
		assert.Equal(t, cents(t, summary.NetDividends), acc.NetDividends, "Account %d: net dividends", acc.AccountID)
		assert.Equal(t, cents(t, summary.NetInterest), *acc.NetInterest, "Account %d: net interest", acc.AccountID)
		assert.Equal(t, cents(t, summary.Fees), acc.Fees, "Account %d: fees", acc.AccountID)
		sum := byCurrency[summary.Currency]
		byCurrency[summary.Currency] = totals{
			dividends: sum.dividends.Add(decimal.RequireFromString(summary.NetDividends)),
			interest:  sum.interest.Add(decimal.RequireFromString(summary.NetInterest)),
			fees:      sum.fees.Add(decimal.RequireFromString(summary.Fees)),
		}
	}
	assert.Len(t, got.Totals, len(byCurrency), "one sum per currency")
	for _, sum := range got.Totals {
		want, ok := byCurrency[sum.Currency]
		require.True(t, ok, "currency %s", sum.Currency)
		require.NotNil(t, sum.NetInterest, "%s: net interest", sum.Currency)
		// A sum is the exact sum, rounded once.
		assert.Equal(t, want.dividends.StringFixed(2), sum.NetDividends, "%s: net dividends", sum.Currency)
		assert.Equal(t, want.interest.StringFixed(2), *sum.NetInterest, "%s: net interest", sum.Currency)
		assert.Equal(t, want.fees.StringFixed(2), sum.Fees, "%s: fees", sum.Currency)
	}
}

func TestAskAssistant_IncomeAndFees_AllAccounts_MatchTheSummaryEndpoints(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, incomeExport(), "XTB USD")
	srv.importAccount(t, incomeAndCostsExport(), "XTB USD 2")
	srv.importAccount(t, incomePLNExport(), "XTB PLN")

	_, got := askIncome(t, srv, model, `{}`)

	checkIncomeMatchesSummaries(t, srv, got)
}
