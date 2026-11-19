package httpapi_test

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// closedPositionsResult is the result of the function get_closed_positions.
type closedPositionsResult struct {
	Scope              string                       `json:"scope"`
	Instrument         string                       `json:"instrument"`
	From               string                       `json:"from"`
	To                 string                       `json:"to"`
	Order              string                       `json:"order"`
	Limit              int                          `json:"limit"`
	TotalRows          int                          `json:"totalRows"`
	RowsLeftOut        int                          `json:"rowsLeftOut"`
	RealizedProfitSums []realizedProfitSumResult    `json:"realizedProfitSums"`
	Rows               []assistantClosedPositionRow `json:"rows"`
	Error              string                       `json:"error"`
}

// realizedProfitSumResult is the Realized Profit sum of one currency.
type realizedProfitSumResult struct {
	Currency string `json:"currency"`
	Sum      string `json:"sum"`
	Rows     int    `json:"rows"`
}

// assistantClosedPositionRow is one row of the get_closed_positions result.
type assistantClosedPositionRow struct {
	AccountID   int64  `json:"accountId"`
	AccountName string `json:"accountName"`
	Currency    string `json:"currency"`
	// The row has no broker IDs: no xtbId and no positionId.
	Time           string  `json:"time"`
	Ticker         string  `json:"ticker"`
	Name           string  `json:"name"`
	Volume         string  `json:"volume"`
	Amount         string  `json:"amount"`
	Cost           string  `json:"cost"`
	RealizedProfit string  `json:"realizedProfit"`
	OpenTime       string  `json:"openTime"`
	OpenPrice      *string `json:"openPrice"`
	ClosePrice     *string `json:"closePrice"`
}

// rowOf returns the row of the Assistant for a row of the endpoint. It has the Account fields empty and
// no broker IDs.
func rowOf(c closedPositionResponse) assistantClosedPositionRow {
	return assistantClosedPositionRow{
		Time: c.Time, Ticker: c.Ticker, Name: c.Name, Volume: c.Volume, Amount: c.Amount, Cost: c.Cost,
		RealizedProfit: c.RealizedProfit, OpenTime: c.OpenTime, OpenPrice: c.OpenPrice, ClosePrice: c.ClosePrice,
	}
}

// tradeAt is a Position that the test buys and sells in full, with one unit of volume.
type tradeAt struct {
	ticker, name, buy, sell string
	closed                  time.Time
}

// tradesExport builds an export of fully closed Positions. The XTB IDs and the Position IDs start at base.
func tradesExport(accountNumber, currency string, base int, trades ...tradeAt) xtbtest.Export {
	export := xtbtest.Export{AccountNumber: accountNumber, Currency: currency}
	export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{
		Type: "Deposit", Time: time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC), Amount: "100000.00", ID: strconv.Itoa(base), Comment: "Blik deposit",
	})
	for i, tr := range trades {
		position := strconv.Itoa(4000000000 + base + i)
		export.CashOperations = append(export.CashOperations,
			xtbtest.CashOperation{
				Type: "Stock purchase", Ticker: tr.ticker, Instrument: tr.name, Time: tr.closed.AddDate(0, 0, -30), Amount: "-" + tr.buy,
				ID: strconv.Itoa(base + 1000 + i), Comment: "OPEN BUY 1 @ " + tr.buy, PositionID: position,
			},
			xtbtest.CashOperation{
				Type: "Stock sell", Ticker: tr.ticker, Instrument: tr.name, Time: tr.closed, Amount: tr.sell,
				ID: strconv.Itoa(base + 2000 + i), Comment: "CLOSE BUY 1 @ " + tr.sell, PositionID: position,
			})
	}
	return export
}

// closedPositionsFixture has four Closed Positions in USD and two in PLN. The Realized Profit values are
//
//	USD: ABC.US +30.00 (2025-12-31 23:30 UTC, that is 2026-01-01 in Polish time), ABC.US -10.00 (2026-03-10),
//	     XYZ.US +70.00 (2026-05-04), ABC.US +5.00 (2026-06-01). The sum is 95.00.
//	PLN: CDR.PL +60.00 (2026-04-02), CDR.PL -10.00 (2026-04-20). The sum is 50.00.
func closedPositionsFixture(t *testing.T) (srv *testServer, model *fakeModel, usdID, plnID int64) {
	t.Helper()

	srv, model = startAssistantServer(t, newFakeProvider())
	usdID = srv.importAccount(t, tradesExport("57000001", "USD", 100,
		tradeAt{"ABC.US", "ABC", "100.00", "130.00", time.Date(2025, 12, 31, 23, 30, 0, 0, time.UTC)},
		tradeAt{"ABC.US", "ABC", "100.00", "90.00", time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)},
		tradeAt{"XYZ.US", "XYZ", "50.00", "120.00", time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)},
		tradeAt{"ABC.US", "ABC", "200.00", "205.00", time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)},
	), "XTB USD")
	plnID = srv.importAccount(t, tradesExport("57000002", "PLN", 500,
		tradeAt{"CDR.PL", "CD Projekt", "100.00", "160.00", time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)},
		tradeAt{"CDR.PL", "CD Projekt", "40.00", "30.00", time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)},
	), "XTB PLN")
	return srv, model, usdID, plnID
}

// askClosedPositions asks one question. The fake model calls get_closed_positions with the arguments once.
// It returns the answer and the result as the model got it.
func askClosedPositions(t *testing.T, srv *testServer, model *fakeModel, arguments string) (answerResponse, closedPositionsResult) {
	t.Helper()

	answer, content := askWithStep(t, srv, model, step("get_closed_positions", arguments))
	var got closedPositionsResult
	decode(t, content, &got)
	return answer, got
}

// profits returns the Realized Profit of the rows, in order.
func profits(rows []assistantClosedPositionRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.RealizedProfit)
	}
	return out
}

func TestAskAssistant_ClosedPositions_NoFilter_RowsAgreeWithTheEndpointNewestFirst(t *testing.T) {
	t.Parallel()
	srv, model, usdID, _ := closedPositionsFixture(t)

	answer, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, usdID))

	assert.Equal(t, []answerStep{{Function: "get_closed_positions", Arguments: map[string]any{"accountId": float64(usdID)}}}, answer.Steps)
	assert.Equal(t, `Account "XTB USD"`, got.Scope)
	assert.Equal(t, "newest_first", got.Order)
	assert.Equal(t, 4, got.TotalRows)
	assert.Equal(t, 0, got.RowsLeftOut)
	assert.Equal(t, []realizedProfitSumResult{{Currency: "USD", Sum: "95.00", Rows: 4}}, got.RealizedProfitSums)

	endpoint := srv.closedPositions(t, usdID)
	slices.Reverse(endpoint)
	require.Len(t, got.Rows, len(endpoint))
	for i, want := range endpoint {
		wantRow := rowOf(want)
		wantRow.AccountID, wantRow.AccountName, wantRow.Currency = usdID, "XTB USD", "USD"
		assert.Equal(t, wantRow, got.Rows[i], "row %d", i)
	}
}

func TestAskAssistant_ClosedPositions_FilterByInstrument(t *testing.T) {
	t.Parallel()
	srv, model, usdID, _ := closedPositionsFixture(t)

	// The match of the ticker ignores the case of the letters.
	_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "instrument": "abc.us"}`, usdID))

	assert.Equal(t, "ABC.US", got.Instrument)
	assert.Equal(t, []string{"5.00", "-10.00", "30.00"}, profits(got.Rows))
	assert.Equal(t, []realizedProfitSumResult{{Currency: "USD", Sum: "25.00", Rows: 3}}, got.RealizedProfitSums)
}

func TestAskAssistant_ClosedPositions_Period_UsesPolishDaysBothIncluded(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arguments  string
		wantProfit []string
		wantSum    string
	}{
		{
			name:       "the first day is included, and the close at 00:30 Polish time belongs to it",
			arguments:  `"from": "2026-01-01", "to": "2026-03-10"`,
			wantProfit: []string{"-10.00", "30.00"},
			wantSum:    "20.00",
		},
		{
			name:       "the last day is included",
			arguments:  `"from": "2026-03-10", "to": "2026-05-04"`,
			wantProfit: []string{"70.00", "-10.00"},
			wantSum:    "60.00",
		},
		{
			name:       "the close at 23:30 UTC is not in the last day of the year before",
			arguments:  `"to": "2025-12-31"`,
			wantProfit: []string{},
		},
		{
			name:       "only the first day",
			arguments:  `"from": "2026-06-01"`,
			wantProfit: []string{"5.00"},
			wantSum:    "5.00",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model, usdID, _ := closedPositionsFixture(t)

			_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, %s}`, usdID, tc.arguments))

			assert.Equal(t, tc.wantProfit, profits(got.Rows))
			assert.Equal(t, len(tc.wantProfit), got.TotalRows)
			if tc.wantSum == "" {
				assert.Empty(t, got.RealizedProfitSums)
				return
			}
			assert.Equal(t, []realizedProfitSumResult{{Currency: "USD", Sum: tc.wantSum, Rows: len(tc.wantProfit)}}, got.RealizedProfitSums)
		})
	}
}

func TestAskAssistant_ClosedPositions_PeriodIsEchoedInTheResult(t *testing.T) {
	t.Parallel()
	srv, model, usdID, _ := closedPositionsFixture(t)

	_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "from": "2026-01-01", "to": "2026-12-31"}`, usdID))

	assert.Equal(t, "2026-01-01", got.From)
	assert.Equal(t, "2026-12-31", got.To)
}

func TestAskAssistant_ClosedPositions_Order(t *testing.T) {
	t.Parallel()
	tests := []struct {
		order      string
		wantProfit []string
	}{
		{"profit_desc", []string{"70.00", "30.00", "5.00", "-10.00"}},
		{"profit_asc", []string{"-10.00", "5.00", "30.00", "70.00"}},
	}
	for _, tc := range tests {
		t.Run(tc.order, func(t *testing.T) {
			t.Parallel()
			srv, model, usdID, _ := closedPositionsFixture(t)

			_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "order": %q}`, usdID, tc.order))

			assert.Equal(t, tc.order, got.Order)
			assert.Equal(t, tc.wantProfit, profits(got.Rows))
		})
	}
}

func TestAskAssistant_ClosedPositions_RowLimit_SumCoversAllRowsAndResultCountsTheLeftOutRows(t *testing.T) {
	t.Parallel()
	srv, model, usdID, _ := closedPositionsFixture(t)

	_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "order": "profit_desc", "limit": 2}`, usdID))

	assert.Equal(t, []string{"70.00", "30.00"}, profits(got.Rows))
	assert.Equal(t, 2, got.Limit)
	assert.Equal(t, 4, got.TotalRows)
	assert.Equal(t, 2, got.RowsLeftOut)
	assert.Equal(t, []realizedProfitSumResult{{Currency: "USD", Sum: "95.00", Rows: 4}}, got.RealizedProfitSums)
}

func TestAskAssistant_ClosedPositions_RowLimit_HasADefaultAndAMaximum(t *testing.T) {
	t.Parallel()
	const count = 105
	trades := make([]tradeAt, count)
	for i := range trades {
		trades[i] = tradeAt{"ABC.US", "ABC", "10.00", "11.00", time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour)}
	}
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, tradesExport("57000003", "USD", 100, trades...), "XTB USD")

	_, byDefault := askClosedPositions(t, srv, model, `{}`)
	_, tooMany := askClosedPositions(t, srv, model, `{"limit": 1000}`)

	assert.Len(t, byDefault.Rows, 20)
	assert.Equal(t, 20, byDefault.Limit)
	assert.Equal(t, count-20, byDefault.RowsLeftOut)
	assert.Equal(t, []realizedProfitSumResult{{Currency: "USD", Sum: "105.00", Rows: count}}, byDefault.RealizedProfitSums)
	assert.Len(t, tooMany.Rows, 100)
	assert.Equal(t, 100, tooMany.Limit)
	assert.Equal(t, count-100, tooMany.RowsLeftOut)
}

func TestAskAssistant_ClosedPositions_AllAccounts_GiveOneSumPerCurrency(t *testing.T) {
	t.Parallel()
	srv, model, usdID, plnID := closedPositionsFixture(t)

	_, got := askClosedPositions(t, srv, model, `{}`)

	assert.Equal(t, "all Accounts", got.Scope)
	assert.Equal(t, 6, got.TotalRows)
	assert.Equal(t, []realizedProfitSumResult{
		{Currency: "PLN", Sum: "50.00", Rows: 2},
		{Currency: "USD", Sum: "95.00", Rows: 4},
	}, got.RealizedProfitSums)
	// Newest first, across the Accounts.
	assert.Equal(t, []string{"5.00", "70.00", "-10.00", "60.00", "-10.00", "30.00"}, profits(got.Rows))
	require.Len(t, got.Rows, 6)
	assert.Equal(t, plnID, got.Rows[3].AccountID)
	assert.Equal(t, "PLN", got.Rows[3].Currency)
	assert.Equal(t, usdID, got.Rows[0].AccountID)

	// The sums stay separate also with a row limit.
	_, limited := askClosedPositions(t, srv, model, `{"limit": 1, "order": "profit_asc"}`)
	assert.Equal(t, []string{"-10.00"}, profits(limited.Rows))
	assert.Equal(t, got.RealizedProfitSums, limited.RealizedProfitSums)
}

func TestAskAssistant_ClosedPositions_AllAccounts_TieOfProfitIsNewestFirst(t *testing.T) {
	t.Parallel()
	srv, model, _, _ := closedPositionsFixture(t)

	_, got := askClosedPositions(t, srv, model, `{"order": "profit_asc", "limit": 2}`)

	// The two rows with -10.00: the PLN row (2026-04-20) is newer than the USD row (2026-03-10).
	require.Len(t, got.Rows, 2)
	assert.Equal(t, []string{"-10.00", "-10.00"}, profits(got.Rows))
	assert.Equal(t, []string{"PLN", "USD"}, []string{got.Rows[0].Currency, got.Rows[1].Currency})
	// RFC 3339 text in UTC sorts in time order.
	assert.Less(t, got.Rows[1].Time, got.Rows[0].Time)
}

func TestAskAssistant_ClosedPositions_ProblemWithTheArguments_GivesTheModelAnError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		arguments string
		wantError []string
	}{
		{"first day after last day", `{"from": "2026-05-01", "to": "2026-01-01"}`, []string{"from", "2026-05-01", "after", "to", "2026-01-01"}},
		{"date in a wrong form", `{"from": "01.05.2026"}`, []string{"from", "01.05.2026", "YYYY-MM-DD"}},
		{"date that does not exist", `{"to": "2026-02-30"}`, []string{"to", "2026-02-30", "YYYY-MM-DD"}},
		{"unknown order", `{"order": "best"}`, []string{"order", "best", "profit_asc", "profit_desc"}},
		{"limit below 1", `{"limit": 0}`, []string{"limit", "1"}},
		{"unknown Account", `{"accountId": 999}`, []string{"unknown Account 999", `(\"XTB USD\")`}},
		{"unknown Instrument", `{"instrument": "NVDA"}`, []string{`unknown Instrument \"NVDA\"`, "ABC.US"}},
		{"limit that is not an integer", `{"limit": 2.5}`, []string{"arguments are not valid"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, model, _, _ := closedPositionsFixture(t)

			answer, content := askWithStep(t, srv, model, step("get_closed_positions", tc.arguments))

			var got closedPositionsResult
			decode(t, content, &got)
			assert.NotEmpty(t, got.Error)
			assert.Empty(t, got.Rows)
			for _, want := range tc.wantError {
				assert.Contains(t, content, want)
			}
			assert.Equal(t, "Here is the answer.", answer.Answer, "the loop must continue after an error result")
		})
	}
}

func TestAskAssistant_ClosedPositions_NoBrokerCommentAndNoAccountNumberGoToTheModel(t *testing.T) {
	t.Parallel()
	srv, model, _, _ := closedPositionsFixture(t)

	_, got := askClosedPositions(t, srv, model, `{}`)

	require.NotEmpty(t, got.Rows)
	for _, call := range model.recordedCalls() {
		for _, m := range call {
			// The fixture gives the Position IDs 4000000100 and up, and the XTB IDs of the sell operations 2100 and up.
			// The row has no field for a broker ID.
			for _, private := range []string{
				"57000001", "57000002", "CLOSE BUY", "OPEN BUY", "Blik deposit", "xtbId", "positionId", "4000000",
			} {
				assert.NotContains(t, m.Content, private, "a %s message", m.Role)
			}
		}
	}
}

func TestAskAssistant_ClosedPositions_IsOneOfTheFunctionsOfTheModel(t *testing.T) {
	t.Parallel()
	srv, model, _, _ := closedPositionsFixture(t)

	askClosedPositions(t, srv, model, `{}`)

	functions := model.recordedFunctions()
	require.NotEmpty(t, functions)
	var names []string
	for _, f := range functions[0] {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "get_closed_positions")
}
