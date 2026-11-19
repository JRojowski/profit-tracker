package httpapi_test

import (
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

// periodResult is the result of the function get_results with a period.
type periodResult struct {
	Scope       string   `json:"scope"`
	AccountID   *int64   `json:"accountId"`
	AccountName string   `json:"accountName"`
	Currency    string   `json:"currency"`
	Available   bool     `json:"available"`
	Warnings    []string `json:"warnings"`
	Reason      *string  `json:"reason"`

	From         string   `json:"from"`
	To           string   `json:"to"`
	StartDate    string   `json:"startDate"`
	EndDate      string   `json:"endDate"`
	StartValue   *string  `json:"startValue"`
	EndValue     *string  `json:"endValue"`
	NetDeposits  *string  `json:"netDeposits"`
	TotalResult  *string  `json:"totalResult"`
	ValuedAtCost []string `json:"valuedAtCost"`
	Error        string   `json:"error"`

	// The result of a period holds no return and no part of the all-time result.
	SimpleReturn *returnResult `json:"simpleReturn"`
	AnnualReturn *returnResult `json:"annualReturn"`
	TimeWeighted any           `json:"timeWeightedReturn"`
	Value        *string       `json:"value"`
}

// askPeriod asks one question. The fake model calls get_results with the arguments once.
// It returns the result as the model got it.
func askPeriod(t *testing.T, srv *testServer, model *fakeModel, arguments string) periodResult {
	t.Helper()

	_, content := askWithStep(t, srv, model, step("get_results", arguments))
	var got periodResult
	decode(t, content, &got)
	return got
}

// accountPeriodArguments returns the arguments of get_results for the Account and the period.
func accountPeriodArguments(id int64, from, to string) string {
	return fmt.Sprintf(`{"accountId": %d, "from": %q, "to": %q}`, id, from, to)
}

// pointOn returns the point of the day. It stops the test if the Value History has no such day.
func pointOn(t *testing.T, points []valuePoint, date string) valuePoint {
	t.Helper()

	for _, p := range points {
		if p.Date == date {
			return p
		}
	}
	require.Failf(t, "no point", "the Value History has no point on %s", date)
	return valuePoint{}
}

// zeroPoint is the state before the first Cash Operation.
var zeroPoint = valuePoint{Value: "0", NetDeposits: "0"}

// expectedPeriod returns the values that the points of two days give for the period.
// The start point is the point of the day before the period.
func expectedPeriod(start, end valuePoint) (startValue, endValue, netDeposits, totalResult decimal.Decimal) {
	startValue, endValue = decimal.RequireFromString(start.Value), decimal.RequireFromString(end.Value)
	netDeposits = decimal.RequireFromString(end.NetDeposits).Sub(decimal.RequireFromString(start.NetDeposits))
	return startValue, endValue, netDeposits, endValue.Sub(startValue).Sub(netDeposits)
}

// requireAmount checks an amount of a result and returns it as a decimal.
func requireAmount(t *testing.T, name string, got *string) decimal.Decimal {
	t.Helper()

	require.NotNil(t, got, name)
	d, err := decimal.NewFromString(*got)
	require.NoError(t, err, "%s %q is not decimal text", name, *got)
	return d
}

// checkPeriodMatchesPoints checks that the result has the values of the points of the two ends of the period.
func checkPeriodMatchesPoints(t *testing.T, got periodResult, start, end valuePoint) {
	t.Helper()

	startValue, endValue, netDeposits, total := expectedPeriod(start, end)
	require.True(t, got.Available)
	for name, pair := range map[string]struct {
		want decimal.Decimal
		got  *string
	}{
		"startValue": {startValue, got.StartValue}, "endValue": {endValue, got.EndValue},
		"netDeposits": {netDeposits, got.NetDeposits}, "totalResult": {total, got.TotalResult},
	} {
		g := requireAmount(t, name, pair.got)
		assert.True(t, pair.want.Equal(g), "%s: want %s, got %s", name, pair.want, g)
	}
}

// periodServer returns a server with a USD Account with a deposit on 2026-10-21. historyProvider gives the prices.
func periodServer(t *testing.T) (*testServer, *fakeModel, int64) {
	t.Helper()

	srv, model := startAssistantServer(t, historyProvider(t))
	id := srv.importAccount(t, historyExport(cashDeposit("802", "500.00", historyTime(21))), "XTB USD")
	srv.refreshHistory(t)
	return srv, model, id
}

func TestAskAssistant_ResultsWithPeriod_AgreeWithThePointsOfTheValueHistory(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	got := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-21", "2026-10-22"))

	points := srv.valueHistory(t, id).Points
	checkPeriodMatchesPoints(t, got, pointOn(t, points, "2026-10-20"), pointOn(t, points, "2026-10-22"))
	assert.Equal(t, `Account "XTB USD"`, got.Scope)
	require.NotNil(t, got.AccountID)
	assert.Equal(t, id, *got.AccountID)
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, "2026-10-21", got.From)
	assert.Equal(t, "2026-10-22", got.To)
	assert.Equal(t, "2026-10-20", got.StartDate, "the value at the start is the value at the end of the day before the period")
	assert.Equal(t, "2026-10-22", got.EndDate)
	// The deposit of the day 2026-10-21 is a Net Deposit of the period.
	assert.Equal(t, "500.00", *got.NetDeposits)
}

func TestAskAssistant_ResultsWithPeriod_HoldNoReturnAndNoAllTimePart(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	_, content := askWithStep(t, srv, model, step("get_results", accountPeriodArguments(id, "2026-10-21", "2026-10-22")))

	for _, name := range []string{"simpleReturn", "annualReturn", "timeWeightedReturn", "value", "cash", "realizedProfit", "unrealizedProfit"} {
		assert.NotContains(t, content, fmt.Sprintf("%q", name))
	}
}

func TestAskAssistant_ResultsWithPeriod_StartingBeforeTheFirstCashOperation_GiveTheTotalResultAtTheEnd(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	got := askPeriod(t, srv, model, accountPeriodArguments(id, "2025-01-01", "2026-10-22"))

	points := srv.valueHistory(t, id).Points
	checkPeriodMatchesPoints(t, got, zeroPoint, pointOn(t, points, "2026-10-22"))
	assert.Equal(t, "0.00", *got.StartValue)
	assert.Equal(t, "2024-12-31", got.StartDate)
	// The Total Result at the end of the period is the value minus the Net Deposits of that day.
	end := pointOn(t, points, "2026-10-22")
	want := decimal.RequireFromString(end.Value).Sub(decimal.RequireFromString(end.NetDeposits))
	assert.True(t, want.Equal(requireAmount(t, "totalResult", got.TotalResult)), "want %s, got %s", want, *got.TotalResult)
}

func TestAskAssistant_ResultsWithPeriod_ThatEndsBeforeTheFirstCashOperation_GiveZero(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	got := askPeriod(t, srv, model, accountPeriodArguments(id, "2025-01-01", "2025-12-31"))

	require.True(t, got.Available)
	assert.Equal(t, "0.00", *got.EndValue)
	assert.Equal(t, "0.00", *got.NetDeposits)
	assert.Equal(t, "0.00", *got.TotalResult)
}

func TestAskAssistant_ResultsWithPeriod_TwoPeriodsThatFollowEachOtherAddToTheJoinedPeriod(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	first := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-01", "2026-10-20"))
	second := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-21", "2026-10-23"))
	joined := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-01", "2026-10-23"))

	sum := requireAmount(t, "first", first.TotalResult).Add(requireAmount(t, "second", second.TotalResult))
	want := requireAmount(t, "joined", joined.TotalResult)
	assert.True(t, want.Equal(sum), "want %s, got %s", want, sum)
	assert.NotEqual(t, "0.00", *first.TotalResult, "the test needs a period with a result")
}

func TestAskAssistant_ResultsWithPeriod_ThatEndsAfterToday_EndsToday(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	toToday := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-20", "2026-10-23"))
	afterToday := askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-20", "2099-12-31"))

	assert.Equal(t, "2026-10-23", afterToday.EndDate, "the result states the end day that it used")
	assert.Equal(t, "2099-12-31", afterToday.To, "the result keeps the period that the model asked for")
	afterToday.To = toToday.To
	assert.Equal(t, toToday, afterToday)
}

func TestAskAssistant_ResultsWithPeriod_NotValidPeriods_GiveTheModelAnError(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	tests := []struct {
		name      string
		arguments string
		want      string
	}{
		{"from alone", fmt.Sprintf(`{"accountId": %d, "from": "2026-10-01"}`, id), "from and to"},
		{"to alone", fmt.Sprintf(`{"accountId": %d, "to": "2026-10-01"}`, id), "from and to"},
		{"from after to", accountPeriodArguments(id, "2026-10-22", "2026-10-21"), "is after to"},
		{"fully after today", accountPeriodArguments(id, "2026-10-24", "2026-12-31"), "after today"},
		{"bad date", accountPeriodArguments(id, "2026-13-01", "2026-12-31"), "not a date"},
		{"unknown Account", `{"accountId": 999, "from": "2026-10-01", "to": "2026-10-02"}`, "unknown Account 999"},
	}
	// The fake model has one script at a time. Thus the cases run one after the other, with no subtests.
	for _, tc := range tests {
		_, content := askWithStep(t, srv, model, step("get_results", tc.arguments))

		var got periodResult
		decode(t, content, &got)
		assert.Contains(t, got.Error, tc.want, tc.name)
		assert.Nil(t, got.TotalResult, tc.name)
	}
}

func TestAskAssistant_ResultsWithPeriod_NameTheInstrumentsValuedAtCostOnTheTwoDaysOnly(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	// XYZ.US has no stored close before 2026-10-22. The Value History values it at cost on 2026-10-20 and 2026-10-21.
	provider.setHistory(t, "ABC", "USD", "2026-10-19=101", "2026-10-20=103", "2026-10-21=105", "2026-10-22=104")
	provider.setHistory(t, "XYZ", "USD", "2026-10-22=52")
	provider.set(t, "XYZ", "52", "USD", "2026-10-23")
	srv, model := startAssistantServer(t, provider)
	id := srv.importAccount(t, historyExport(), "XTB USD")
	srv.refreshHistory(t)
	require.Equal(t, []valuedAtCostDTO{{Ticker: "XYZ.US", Name: "XYZ"}}, srv.valueHistory(t, id).ValuedAtCost)

	tests := []struct {
		name     string
		from, to string
		want     []string
	}{
		{"the start day is valued at cost", "2026-10-22", "2026-10-23", []string{"XYZ.US"}},
		{"the end day is valued at cost", "2026-10-15", "2026-10-21", []string{"XYZ.US"}},
		{"a day between the two days is valued at cost", "2026-10-19", "2026-10-23", []string{}},
		{"no day is valued at cost", "2026-10-23", "2026-10-23", []string{}},
	}
	// The fake model has one script at a time. Thus the cases run one after the other, with no subtests.
	for _, tc := range tests {
		got := askPeriod(t, srv, model, accountPeriodArguments(id, tc.from, tc.to))

		assert.Equal(t, tc.want, got.ValuedAtCost, tc.name)
	}
}

func TestAskAssistant_ResultsWithPeriod_ForAllAccounts_GiveTheValuesInPLNOfTheValueHistory(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.rateProvider.set(t, "USD", "2026-10-05", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-22", "3.60")
	srv.importAccount(t, adr0004Export(), "XTB USD")
	srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")
	srv.refreshPrices(t)

	got := askPeriod(t, srv, model, `{"from": "2026-10-06", "to": "2026-10-22"}`)

	history := srv.allValueHistory(t)
	require.True(t, history.Available)
	checkPeriodMatchesPoints(t, got, pointOn(t, history.Points, "2026-10-05"), pointOn(t, history.Points, "2026-10-22"))
	assert.Equal(t, "all Accounts", got.Scope)
	assert.Nil(t, got.AccountID)
	assert.Equal(t, "PLN", got.Currency)
}

func TestAskAssistant_ResultsWithPeriod_ForAllAccountsWithNoValueHistory_GiveTheReasonAndNoNumber(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	// No Exchange Rate for USD: the Value History of all Accounts is not available.
	srv.importAccount(t, adr0004Export(), "XTB USD")
	require.False(t, srv.allValueHistory(t).Available)

	got := askPeriod(t, srv, model, `{"from": "2026-10-06", "to": "2026-10-22"}`)

	assert.False(t, got.Available)
	assert.Equal(t, "PLN", got.Currency)
	require.NotNil(t, got.Reason)
	assert.Equal(t, srv.allValueHistory(t).Warnings, got.Warnings)
	assert.NotEmpty(t, got.Warnings)
	for name, v := range map[string]*string{
		"startValue": got.StartValue, "endValue": got.EndValue, "netDeposits": got.NetDeposits, "totalResult": got.TotalResult,
	} {
		assert.Nil(t, v, name)
	}
}

func TestAskAssistant_ResultsWithPeriod_ForAllAccounts_NameTheInstrumentsValuedAtCost(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.rateProvider.set(t, "USD", "2026-01-02", "4.00")
	srv.rateProvider.set(t, "USD", "2026-10-23", "4.00")
	// pricedExport holds Instruments with no stored price: the Value History values them at cost.
	srv.importAccount(t, pricedExport(), "XTB USD")
	srv.importAccount(t, depositOnlyExport("56000099", "PLN"), "XTB PLN")
	srv.refreshPrices(t)
	history := srv.allValueHistory(t)
	require.True(t, history.Available)
	require.NotEmpty(t, history.ValuedAtCost)

	got := askPeriod(t, srv, model, `{"from": "2026-10-22", "to": "2026-10-23"}`)

	assert.Equal(t, []string{"ABC.US", "XYZ.US"}, got.ValuedAtCost)
}

func TestAskAssistant_ResultsWithPeriod_InstructionsTellTheModelToWarnAboutAnInstrumentValuedAtCost(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-21", "2026-10-22"))

	calls := model.recordedCalls()
	require.NotEmpty(t, calls)
	instructions := calls[0][0]
	assert.Equal(t, assistant.RoleSystem, instructions.Role)
	assert.Contains(t, instructions.Content, "names an Instrument in valuedAtCost, tell the user that the number is not exact")
}

func TestAskAssistant_ResultsFunctionDefinition_HasTheArgumentsFromAndTo(t *testing.T) {
	t.Parallel()
	srv, model, id := periodServer(t)

	askPeriod(t, srv, model, accountPeriodArguments(id, "2026-10-21", "2026-10-22"))

	var parameters string
	for _, f := range model.recordedFunctions()[0] {
		if f.Name == "get_results" {
			parameters = string(f.Parameters)
		}
	}
	assert.Contains(t, parameters, `"from"`)
	assert.Contains(t, parameters, `"to"`)
}
