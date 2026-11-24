package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// The real-export tests read the user's XTB exports and check the results against the broker's own sheets.
// The exports contain personal data. Thus these tests obey these rules:
//   - Do not copy the content of an export into the repository.
//   - Do not log the file name, because it contains the account number.
//   - In a failure message, show only the IDs and the amounts of the failed check.

// exportsEnv is the environment variable that sets the folder of the real exports.
// If it is not set, the folder is "exports" in the repository root.
const exportsEnv = "PROFIT_TRACKER_EXPORTS"

// exportPrefix matches a safe file name prefix of an export, for example "USD" or "IKZE".
var exportPrefix = regexp.MustCompile(`^[A-Z]{3,4}$`)

// realExport is one real export file.
type realExport struct {
	// label names the export in subtests and messages: the index and the file name prefix, for example "1 USD".
	label string
	// prefix is the part of the file name before the first "_", for example "USD" or "IKZE".
	// It is "" if that part is not 3 or 4 capital letters.
	prefix string
	data   []byte
}

// realExports returns the xlsx files in the exports folder, in file name order.
// It skips the test if the folder has no xlsx file.
func realExports(t *testing.T) []realExport {
	t.Helper()

	dir := exportsDir(t)
	paths, err := filepath.Glob(filepath.Join(dir, "*.xlsx"))
	require.NoError(t, err)
	// Excel makes a lock file "~$<name>.xlsx" while a file is open. It is not an export.
	paths = slices.DeleteFunc(paths, func(p string) bool { return strings.HasPrefix(filepath.Base(p), "~$") })
	if len(paths) == 0 {
		t.Skipf("no XTB export (*.xlsx) in the exports folder: put the exports in \"exports/\" at the repository root, or set %s", exportsEnv)
	}
	slices.Sort(paths)

	exports := make([]realExport, 0, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		require.NoError(t, err, "read export %d", i+1)
		label := strconv.Itoa(i + 1)
		prefix, _, _ := strings.Cut(filepath.Base(p), "_")
		// Only a prefix of capital letters goes into the label. Another file name can contain the account number.
		if exportPrefix.MatchString(prefix) {
			label += " " + prefix
		} else {
			prefix = ""
		}
		exports = append(exports, realExport{label: label, prefix: prefix, data: data})
	}
	return exports
}

// exportsDir returns the folder of the real exports: the value of PROFIT_TRACKER_EXPORTS, or else "exports" in
// the repository root. The repository root is the first parent of the test folder that holds "backend/go.mod".
func exportsDir(t *testing.T) string {
	t.Helper()

	if dir := os.Getenv(exportsEnv); dir != "" {
		return dir
	}
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		_, err := os.Stat(filepath.Join(dir, "backend", "go.mod"))
		if err == nil {
			return filepath.Join(dir, "exports")
		}
		require.ErrorIs(t, err, os.ErrNotExist, "look for the repository root")
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no parent folder of the test holds backend/go.mod")
		dir = parent
	}
}

// importRealExport imports the export into a new Account and returns the Account ID.
// The upload file name is the prefix of the real file name, so that the account number does not go to the server.
// It reports each Import Conflict as a failure. It accepts each Import Conflict of the kind "check-mismatch",
// so that the later checks show all differences.
// The failure messages show only the status and the error of a response, because a body can contain personal data.
func (s *testServer) importRealExport(t *testing.T, export realExport) int64 {
	t.Helper()

	status, body := s.upload(t, export.prefix+"_real-export.xlsx", export.data)
	require.Equal(t, http.StatusOK, status, "preview of export %s: %s", export.label, apiError(body))
	var preview struct {
		ID                int64           `json:"id"`
		SuggestedCurrency string          `json:"suggestedCurrency"`
		Conflicts         []checkConflict `json:"conflicts"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &preview), "preview of export %s is not valid JSON", export.label)

	for _, c := range preview.Conflicts {
		if c.Kind != "check-mismatch" || c.Check == nil {
			t.Errorf("export %s: import conflict %s for XTB ID %s", export.label, c.Kind, c.XTBID)
			continue
		}
		t.Errorf("export %s: import conflict %s for XTB ID %s: calculated %s, Closed Positions %s",
			export.label, c.Kind, c.XTBID, c.Check.Calculated, c.Check.ClosedPositions)
		path := fmt.Sprintf("/api/imports/%d/conflicts/%d", preview.ID, c.ID)
		status, body := s.postJSON(t, path, map[string]string{"decision": "accept"})
		require.Equal(t, http.StatusOK, status, "accept conflict %d: %s", c.ID, apiError(body))
	}

	typ, currency := realAccount(export.prefix, preview.SuggestedCurrency)
	require.NotEmpty(t, currency, "export %s has no currency prefix in its file name", export.label)
	path := fmt.Sprintf("/api/imports/%d/confirm", preview.ID)
	status, body = s.postJSON(t, path, newAccountBody("Export "+export.label, typ, currency))
	require.Equal(t, http.StatusOK, status, "confirm of export %s: %s", export.label, apiError(body))
	var saved importResponse
	require.NoError(t, json.Unmarshal([]byte(body), &saved), "confirm of export %s is not valid JSON", export.label)
	return saved.AccountID
}

// getRealJSON sends a GET request to path and decodes the body into v.
// The failure messages show only the status and the error of the response, because the body can contain personal data.
func (s *testServer) getRealJSON(t *testing.T, path string, v any) {
	t.Helper()

	status, _, body := s.get(t, path)
	require.Equal(t, http.StatusOK, status, "GET %s: %s", path, apiError(body))
	require.NoError(t, json.Unmarshal([]byte(body), v), "GET %s: the body is not valid JSON", path)
}

// apiError returns the error field of an error body. It returns a fixed text if the body has no error field.
// It never returns other parts of the body, because they can contain personal data.
func apiError(body string) string {
	var e errorBody
	if err := json.Unmarshal([]byte(body), &e); err != nil || e.Error == "" {
		return "the body has no error field"
	}
	return "error " + strconv.Quote(e.Error)
}

// realAccount returns the Account type and the currency for an export with the file name prefix.
// An IKE or IKZE export has no currency prefix. These Polish tax Accounts are always in PLN.
func realAccount(prefix, suggestedCurrency string) (typ, currency string) {
	switch prefix {
	case "IKE", "IKZE":
		return prefix, "PLN"
	default:
		return "regular", suggestedCurrency
	}
}

// realCheckTolerance is the largest difference between a Realized Profit and its Closed Positions Profit/Loss.
// ADR 0001: a difference of more than 0.01 from Closed Positions is a check failure.
var realCheckTolerance = decimal.RequireFromString("0.01")

// checkClosedPositions checks that the Closed Positions and the Closed Positions rows are one to one, and that the Realized Profit
// of each Closed Position is within 0.01 of the Profit/Loss of its row.
// closedPositionPrices are the prices of the Stock sell Cash Operations of the export, by XTB ID.
//
// A Closed Position and its row have the same position ID and the same closed volume. One Lot can close in parts, so more
// than one row can fit. Then the row with the close time nearest to the Closed Position time wins. The API gives the Closed Position
// time in whole seconds, and the sheet can have milliseconds. On a tie, the row with the Close Price nearest
// to the sale price wins. The check does not use the Realized Profit to select a row, because that value is
// the value under test.
func checkClosedPositions(t *testing.T, closedPositions []closedPositionResponse, closedPositionPrices map[string]decimal.Decimal, rows []xtb.ClosedPosition) {
	t.Helper()

	used := make([]bool, len(rows))
	for _, closedPosition := range closedPositions {
		volume := decimal.RequireFromString(closedPosition.Volume)
		profit := decimal.RequireFromString(closedPosition.RealizedProfit)
		at, err := time.Parse(time.RFC3339, closedPosition.Time)
		require.NoError(t, err, "time of closed position %s", closedPosition.XTBID)
		price, hasPrice := closedPositionPrices[closedPosition.XTBID]

		best := -1
		for i, row := range rows {
			if used[i] || row.PositionID != closedPosition.PositionID || !row.Volume.Equal(volume) {
				continue
			}
			if best < 0 || fitsBetter(row, rows[best], at, decimal.NullDecimal{Decimal: price, Valid: hasPrice}) {
				best = i
			}
		}
		if best < 0 {
			t.Errorf("closed position %s of Lot %s (volume %s) has no Closed Positions row", closedPosition.XTBID, closedPosition.PositionID, closedPosition.Volume)
			continue
		}
		used[best] = true
		broker := rows[best].ProfitLoss
		if profit.Sub(broker).Abs().GreaterThan(realCheckTolerance) {
			t.Errorf("closed position %s of Lot %s: Realized Profit %s, Closed Positions Profit/Loss %s", closedPosition.XTBID, closedPosition.PositionID, profit, broker)
		}
	}
	for i, row := range rows {
		if !used[i] {
			t.Errorf("Closed Positions row %d of Lot %s (volume %s, Profit/Loss %s) has no closed position", row.Row, row.PositionID, row.Volume, row.ProfitLoss)
		}
	}
}

// fitsBetter returns true if the row a is a better match than the row b for a Closed Position with the time at and the price.
// The row with the nearer close time is the better match. If the two times are equally near, the row with the
// nearer Close Price is the better match. If a price is not known, the function returns false, so the first row stays.
// The Close Price cell can carry float noise. Thus the function compares differences, not equality.
func fitsBetter(a, b xtb.ClosedPosition, at time.Time, price decimal.NullDecimal) bool {
	da, db := a.CloseTime.Sub(at).Abs(), b.CloseTime.Sub(at).Abs()
	if da != db {
		return da < db
	}
	if !price.Valid || !a.ClosePrice.Valid || !b.ClosePrice.Valid {
		return false
	}
	return a.ClosePrice.Decimal.Sub(price.Decimal).Abs().LessThan(b.ClosePrice.Decimal.Sub(price.Decimal).Abs())
}

// closedPositionPrices returns the prices of the Stock sell Cash Operations, by XTB ID.
func closedPositionPrices(ops []xtb.CashOperation) map[string]decimal.Decimal {
	prices := make(map[string]decimal.Decimal)
	for _, op := range ops {
		if op.Type == xtb.TypeStockSell && op.Sale != nil {
			prices[op.ID] = op.Sale.Price
		}
	}
	return prices
}

// checkOpenLots checks that each Lot with an open volume is in Open Positions with the same volume,
// and that each Lot in Open Positions has that open volume in the tool.
//
// No endpoint shows the Lots, because the Lots are an internal step of the calculation. Thus this check reads
// the Lots and their Fills with store.Ledger, the same input that the API gives to ledger.Calculate.
// The open volume of a Lot is the sum of its Fill volumes minus the closed volumes of its Closed Positions from the API.
func checkOpenLots(t *testing.T, db *store.Store, accountID int64, closedPositions []closedPositionResponse, openLots []xtb.OpenLot) {
	t.Helper()

	l, err := db.Ledger(t.Context(), accountID)
	require.NoError(t, err)
	calculated := make(map[string]decimal.Decimal, len(l.Lots))
	for _, lot := range l.Lots {
		volume := decimal.Zero
		for _, f := range lot.Fills {
			volume = volume.Add(f.Volume)
		}
		calculated[lot.PositionID] = volume
	}
	for _, closedPosition := range closedPositions {
		calculated[closedPosition.PositionID] = calculated[closedPosition.PositionID].Sub(decimal.RequireFromString(closedPosition.Volume))
	}
	broker := make(map[string]decimal.Decimal, len(openLots))
	for _, lot := range openLots {
		broker[lot.PositionID] = broker[lot.PositionID].Add(lot.Volume)
	}

	for id, volume := range calculated {
		if volume.IsZero() {
			continue
		}
		want, ok := broker[id]
		switch {
		case !ok:
			t.Errorf("Lot %s has the open volume %s, but it is not in Open Positions", id, volume)
		case !want.Equal(volume):
			t.Errorf("Lot %s: open volume %s, Open Positions volume %s", id, volume, want)
		}
	}
	for id, volume := range broker {
		if calculated[id].IsZero() {
			t.Errorf("Lot %s is in Open Positions with the volume %s, but it has no open volume", id, volume)
		}
	}
}

// checkHoldings checks that the volume of each Holding from the API is the sum of the Open Positions volumes
// of its Instrument, and that each Instrument in Open Positions has a Holding.
func checkHoldings(t *testing.T, holdings []holdingResponse, openLots []xtb.OpenLot) {
	t.Helper()

	broker := make(map[string]decimal.Decimal)
	for _, lot := range openLots {
		broker[lot.Ticker] = broker[lot.Ticker].Add(lot.Volume)
	}
	seen := make(map[string]bool, len(holdings))
	for _, h := range holdings {
		seen[h.Ticker] = true
		volume := decimal.RequireFromString(h.Volume)
		want, ok := broker[h.Ticker]
		switch {
		case !ok:
			t.Errorf("Holding %s has the volume %s, but it is not in Open Positions", h.Ticker, volume)
		case !want.Equal(volume):
			t.Errorf("Holding %s: volume %s, Open Positions volume %s", h.Ticker, volume, want)
		}
	}
	for ticker, volume := range broker {
		if !seen[ticker] {
			t.Errorf("Instrument %s is in Open Positions with the volume %s, but it has no Holding", ticker, volume)
		}
	}
}

func TestRealExports_EachExport_MatchesBrokerSheets(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	// All exports go into one database, because the PLN and IKZE Accounts have Transfers between them.
	// The imports run one after the other. A Transfer warning is not a failure. See ADR 0003.
	srv := startServer(t)
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// The checks only read. Thus they run in parallel.
	for i, export := range exports {
		t.Run(export.label, func(t *testing.T) {
			t.Parallel()
			accountID := accountIDs[i]
			// xtb.Read gives the broker's sheets. The checks below match the rows of the sheets with the results of
			// the tool themselves. They do not use the Import Conflicts.
			sheets, err := xtb.Read(export.data)
			require.NoError(t, err)
			var closedPositions []closedPositionResponse
			srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/closed-positions", accountID), &closedPositions)
			var holdings []holdingResponse
			srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &holdings)

			t.Run("each closed position matches Closed Positions", func(t *testing.T) {
				t.Parallel()
				checkClosedPositions(t, closedPositions, closedPositionPrices(sheets.CashOperations), sheets.ClosedPositions)
			})
			t.Run("each open Lot matches Open Positions", func(t *testing.T) {
				t.Parallel()
				checkOpenLots(t, srv.db, accountID, closedPositions, sheets.OpenLots)
			})
			t.Run("each Holding matches Open Positions", func(t *testing.T) {
				t.Parallel()
				checkHoldings(t, holdings, sheets.OpenLots)
			})
			t.Run("each Holding and each Closed Position has an Instrument Overview", func(t *testing.T) {
				t.Parallel()
				checkOverviews(t, srv, accountID, holdings, closedPositions)
			})
		})
	}
}

// checkOverviews calls the Instrument Overview of each Holding and each Closed Position.
// Its Fills must include the Lot of each Closed Position, and their volume must cover the Holding.
func checkOverviews(t *testing.T, srv *testServer, accountID int64, holdings []holdingResponse, closedPositions []closedPositionResponse) {
	t.Helper()

	tickers := map[string]bool{}
	for _, h := range holdings {
		tickers[h.Ticker] = true
	}
	for _, c := range closedPositions {
		tickers[c.Ticker] = true
	}
	holdingRows := srv.holdingRows(t, accountID)
	for ticker := range tickers {
		var got overviewResponse
		status := srv.getJSON(t, overviewPath(accountID, url.PathEscape(ticker)), &got)
		require.Equal(t, http.StatusOK, status, "overview of %s", ticker)
		assert.Equal(t, ticker, got.Ticker)
		assert.NotEmpty(t, got.Fills, "%s: the Account held it, so it has a Fill", ticker)
		assert.NotNil(t, got.Prices.Series, "%s: the series is never null", ticker)
		for i := 1; i < len(got.Prices.Series); i++ {
			assert.Less(t, got.Prices.Series[i-1].Date, got.Prices.Series[i].Date, "%s: the series is in date order", ticker)
		}

		// The Holding of the overview is the row of the Holdings endpoint, or null if there is no row.
		if row, ok := holdingRows[ticker]; ok {
			assert.JSONEq(t, string(row), string(got.Holding), "%s: the Holding equals the Holdings row", ticker)
		} else {
			assert.Equal(t, "null", string(got.Holding), "%s: no open volume gives no Holding", ticker)
			assert.Empty(t, got.OpenLots, "%s: no open volume gives no open Lots", ticker)
		}

		rows := srv.closedPositionRows(t, accountID, ticker)
		assertSameJSON(t, rows, got.ClosedPositions)
		assertRealizedProfitSum(t, ticker, rows, got.RealizedProfitSum)

		lots := map[string]bool{}
		volume := decimal.Zero
		for _, f := range got.Fills {
			lots[f.PositionID] = true
			volume = volume.Add(decimal.RequireFromString(f.Volume))
		}
		for _, c := range closedPositions {
			if c.Ticker == ticker {
				assert.True(t, lots[c.PositionID], "%s: the Fills include the Lot %s of a Closed Position", ticker, c.PositionID)
			}
		}
		for _, h := range holdings {
			if h.Ticker == ticker {
				assert.False(t, volume.LessThan(decimal.RequireFromString(h.Volume)),
					"%s: the Fill volume %s covers the Holding volume %s", ticker, volume, h.Volume)
			}
		}
	}
}

// assertRealizedProfitSum checks that the sum is the total of the Realized Profit of the Closed Positions, or nil when there is none.
func assertRealizedProfitSum(t *testing.T, ticker string, closedPositions []json.RawMessage, got *string) {
	t.Helper()

	if len(closedPositions) == 0 {
		assert.Nil(t, got, "%s: no Closed Position gives no sum", ticker)
		return
	}
	want := decimal.Zero
	for _, item := range closedPositions {
		var c closedPositionResponse
		require.NoError(t, json.Unmarshal(item, &c))
		want = want.Add(decimal.RequireFromString(c.RealizedProfit))
	}
	require.NotNil(t, got, "%s: the Closed Positions give a sum", ticker)
	assert.True(t, want.Equal(decimal.RequireFromString(*got)), "%s: want sum %s, got %s", ticker, want, *got)
}

// TestRealExports_AssistantHoldings_MatchTheHoldingsEndpoint checks that the function get_holdings gives the
// same numbers as the Holdings endpoint, for each Account and for all Accounts. The fake model calls the
// function. No test calls a real model.
func TestRealExports_AssistantHoldings_MatchTheHoldingsEndpoint(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// The fake model has one script at a time. Thus the checks run one after the other, with no subtests.
	for i, export := range exports {
		_, got := askHoldings(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountIDs[i]))
		require.Len(t, got.Accounts, 1, "export %s", export.label)
		checkHoldingsMatchEndpoint(t, srv, got, "")
	}
	_, got := askHoldings(t, srv, model, `{}`)
	require.Len(t, got.Accounts, len(exports), "all Accounts")
	checkHoldingsMatchEndpoint(t, srv, got, "")
}

// TestRealExports_AssistantResults_MatchThePageEndpoints checks that the function get_results gives the same
// numbers as the summary and Value History endpoints, for each Account and for all Accounts.
// The fake model calls the function. No test calls a real model.
func TestRealExports_AssistantResults_MatchThePageEndpoints(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// The fake model has one script at a time. Thus the checks run one after the other, with no subtests.
	for i := range exports {
		got := askResults(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountIDs[i]))
		checkAccountResultsMatchPages(t, srv, got, accountIDs[i])
	}
	checkAllResultsMatchPages(t, srv, askResults(t, srv, model, `{}`))
}

// TestRealExports_AssistantResultsWithPeriod_FromTheFirstCashOperationToToday_MatchTheTotalResult checks that the
// Total Result of the period from the first Cash Operation to today is the Total Result of the summary endpoint,
// for each Account and for all Accounts. The two numbers come from different calculations (the Value History and
// the Ledger), so the check allows a difference of less than 0.01. The fake model calls the function.
// No test calls a real model.
func TestRealExports_AssistantResultsWithPeriod_FromTheFirstCashOperationToToday_MatchTheTotalResult(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// check asks for the period from the first point to the last point of the Value History, which is today.
	check := func(label string, points []valuePoint, arguments func(from, to string) string, want *string) {
		t.Helper()
		require.NotEmpty(t, points, label)
		first, last := points[0].Date, points[len(points)-1].Date
		got := askPeriod(t, srv, model, arguments(first, last))
		require.True(t, got.Available, label)
		assert.Equal(t, "0.00", *got.StartValue, "%s: the start is before the first Cash Operation", label)
		assert.Equal(t, last, got.EndDate, label)
		require.NotNil(t, want, label)
		wantTotal, gotTotal := decimal.RequireFromString(*want), requireAmount(t, label, got.TotalResult)
		assert.True(t, wantTotal.Sub(gotTotal).Abs().LessThan(decimal.RequireFromString("0.01")),
			"%s: the summary has the Total Result %s, the period has %s", label, wantTotal, gotTotal)
	}

	// The fake model has one script at a time. Thus the checks run one after the other, with no subtests.
	for i, export := range exports {
		id := accountIDs[i]
		check("export "+export.label, srv.valueHistory(t, id).Points, func(from, to string) string {
			return accountPeriodArguments(id, from, to)
		}, new(srv.fullSummary(t, id).TotalResult))
	}
	history := srv.allValueHistory(t)
	if !history.Available {
		got := askPeriod(t, srv, model, `{"from": "2000-01-01", "to": "2100-01-01"}`)
		assert.False(t, got.Available, "all Accounts")
		assert.Nil(t, got.TotalResult, "all Accounts: no number without a Value History")
		return
	}
	check("all Accounts", history.Points, func(from, to string) string {
		return fmt.Sprintf(`{"from": %q, "to": %q}`, from, to)
	}, srv.allAccounts(t).TotalResult)
}

// TestRealExports_AssistantAllocation_MatchesThePageEndpoints checks that the function
// get_allocation_and_net_deposits gives the same numbers as the Allocation, summary and IKZE Limit endpoints,
// for each Account and for all Accounts. The fake model calls the function. No test calls a real model.
func TestRealExports_AssistantAllocation_MatchesThePageEndpoints(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	for i := range exports {
		got := askAllocation(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountIDs[i]))
		var allocation allocationResponse
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/allocation", accountIDs[i]), &allocation)
		assert.Equal(t, allocation, got.Allocation, "Account %d: Allocation", accountIDs[i])
		checkAllocationAccountsMatchPages(t, srv, got)
	}
	got := askAllocation(t, srv, model, `{}`)
	var allocation allocationResponse
	srv.getRealJSON(t, "/api/all-accounts/allocation", &allocation)
	assert.Equal(t, allocation, got.Allocation, "all Accounts: Allocation")
	assert.Equal(t, centsPointer(t, srv.allAccounts(t).NetDeposits), got.NetDeposits, "all Accounts: Net Deposits")
	require.Len(t, got.Accounts, len(exports))
	checkAllocationAccountsMatchPages(t, srv, got)
}

// TestRealExports_AssistantClosedPositions_MatchTheClosedPositionsEndpoint checks that the function
// get_closed_positions gives the same rows and the same Realized Profit sum as the Closed Positions endpoint,
// for each Account, for one Instrument and for all Accounts. The fake model calls the function.
// No test calls a real model.
func TestRealExports_AssistantClosedPositions_MatchTheClosedPositionsEndpoint(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// checkAgainst compares the result with the endpoint rows of the Accounts. It uses the limit of the result.
	checkAgainst := func(got closedPositionsResult, label string, ids []int64, ticker string) {
		t.Helper()
		var want []closedPositionResponse
		sums := map[string]decimal.Decimal{}
		counts := map[string]int{}
		for _, id := range ids {
			var accounts []accountResponse
			require.Equal(t, http.StatusOK, srv.getJSON(t, "/api/accounts", &accounts))
			var acc accountResponse
			for _, a := range accounts {
				if a.ID == id {
					acc = a
				}
			}
			require.NotEmpty(t, acc.Currency, "the Account %d has a currency", id)
			for _, c := range srv.closedPositions(t, id) {
				if ticker != "" && c.Ticker != ticker {
					continue
				}
				want = append(want, c)
				sums[acc.Currency] = sums[acc.Currency].Add(decimal.RequireFromString(c.RealizedProfit))
				counts[acc.Currency]++
			}
		}
		assert.Equal(t, len(want), got.TotalRows, "%s: number of rows", label)
		assert.Equal(t, len(want)-len(got.Rows), got.RowsLeftOut, "%s: rows left out", label)
		assert.Len(t, got.RealizedProfitSums, len(sums), "%s: number of currencies", label)
		for _, s := range got.RealizedProfitSums {
			// The Assistant gets the sum with 2 decimal places. The backend rounds the exact sum.
			assert.Equal(t, sums[s.Currency].StringFixed(2), s.Sum, "%s: %s: sum", label, s.Currency)
			assert.Equal(t, counts[s.Currency], s.Rows, "%s: %s: rows of the sum", label, s.Currency)
		}
		// Each row of the result is a row of the endpoint, with the same numbers. The rows have no broker IDs,
		// so the test counts equal rows.
		unmatched := map[string]int{}
		for _, c := range want {
			unmatched[rowKey(t, rowOf(t, c))]++
		}
		for _, r := range got.Rows {
			r.AccountID, r.AccountName, r.Currency = 0, "", ""
			key := rowKey(t, r)
			assert.Positive(t, unmatched[key], "%s: the result has a row that the endpoint has not: %s", label, key)
			unmatched[key]--
		}
	}

	// The fake model has one script at a time. Thus the checks run one after the other, with no subtests.
	for i, export := range exports {
		_, got := askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "limit": 100}`, accountIDs[i]))
		checkAgainst(got, "export "+export.label, accountIDs[i:i+1], "")

		endpoint := srv.closedPositions(t, accountIDs[i])
		if len(endpoint) == 0 {
			continue
		}
		ticker := endpoint[0].Ticker
		_, got = askClosedPositions(t, srv, model, fmt.Sprintf(`{"accountId": %d, "instrument": %q, "limit": 100}`, accountIDs[i], ticker))
		checkAgainst(got, "export "+export.label+" Instrument "+ticker, accountIDs[i:i+1], ticker)
	}
	_, got := askClosedPositions(t, srv, model, `{"limit": 100}`)
	checkAgainst(got, "all Accounts", accountIDs, "")
}

// TestRealExports_AssistantIncomeAndFees_MatchTheSummaryEndpoints checks that the function get_income_and_fees
// with no filter gives the same net dividends, net interest and fees as the summary endpoint, for each Account and
// for all Accounts. It also checks that a period that covers each day gives the same sums as no filter.
// The fake model calls the function. No test calls a real model.
func TestRealExports_AssistantIncomeAndFees_MatchTheSummaryEndpoints(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv, model := startAssistantServer(t, newFakeProvider())
	accountIDs := make([]int64, len(exports))
	for i, export := range exports {
		accountIDs[i] = srv.importRealExport(t, export)
	}

	// The fake model has one script at a time. Thus the checks run one after the other, with no subtests.
	for i, export := range exports {
		_, got := askIncome(t, srv, model, fmt.Sprintf(`{"accountId": %d}`, accountIDs[i]))
		require.Len(t, got.Accounts, 1, "export %s", export.label)
		checkIncomeMatchesSummaries(t, srv, got)

		_, whole := askIncome(t, srv, model, accountPeriod(accountIDs[i], "1900-01-01", "2999-12-31"))
		assert.Equal(t, got.Totals, whole.Totals, "export %s: a period that covers each day", export.label)
	}
	_, got := askIncome(t, srv, model, `{}`)
	require.Len(t, got.Accounts, len(exports), "all Accounts")
	checkIncomeMatchesSummaries(t, srv, got)
}

// rowKey returns the JSON text of the row. Equal rows have equal keys, also when they hold prices as pointers.
func rowKey(t *testing.T, r assistantClosedPositionRow) string {
	t.Helper()
	data, err := json.Marshal(r)
	require.NoError(t, err)
	return string(data)
}

// TestRealExports_TiersBreakdown_Returns200ForTierAccountsAndNotFoundForTheRest checks the Tiers Breakdown of each
// real Account with a sample configuration: the first Holding of each Tier Account goes to a Tier.
// The failure messages show no amounts.
func TestRealExports_TiersBreakdown_Returns200ForTierAccountsAndNotFoundForTheRest(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv := startServer(t)
	for _, export := range exports {
		srv.importRealExport(t, export)
	}
	var accounts []accountResponse
	srv.getRealJSON(t, "/api/accounts", &accounts)
	isTierAccount := func(acc accountResponse) bool {
		return acc.Type == "regular" && (acc.Currency == "PLN" || acc.Currency == "USD")
	}

	var assigned []assignmentRequest
	seen := map[string]bool{}
	for _, acc := range accounts {
		if !isTierAccount(acc) {
			continue
		}
		var holdings []holdingPageRow
		srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", acc.ID), &holdings)
		if len(holdings) > 0 && !seen[holdings[0].Ticker] {
			seen[holdings[0].Ticker] = true
			assigned = append(assigned, assignmentRequest{Ticker: holdings[0].Ticker})
		}
	}
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{Order: 1, Name: "Sample", TargetShare: "100", Instruments: assigned}}})

	for _, acc := range accounts {
		status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/tiers-breakdown", acc.ID))
		if isTierAccount(acc) {
			require.Equal(t, http.StatusOK, status, "Account %d: %s", acc.ID, apiError(body))
			continue
		}
		assert.Equal(t, http.StatusNotFound, status, "Account %d", acc.ID)
	}

	status, _, body := srv.get(t, "/api/all-accounts/tiers-breakdown")
	assert.Equal(t, http.StatusOK, status, "all Accounts: %s", apiError(body))
}

// TestRealExports_Rebalancing_ReturnsTheTableForASampleConfiguration saves a sample configuration of Tiers for the
// Instruments of the real exports and checks that GET /api/all-accounts/rebalancing returns 200.
// The sample: Tier 1 of 60 has the first Instrument as an Instrument Target of 30 and every second Instrument in its
// Shared Part. Tier 2 of 40 has the other Instruments. The failure messages show no amounts.
func TestRealExports_Rebalancing_ReturnsTheTableForASampleConfiguration(t *testing.T) {
	t.Parallel()
	exports := realExports(t)
	srv := startServer(t)
	for _, export := range exports {
		srv.importRealExport(t, export)
	}
	// The refresh gets the fake USD rate. The fake price source has no prices, so each Holding has no price.
	srv.refreshPrices(t)
	options := srv.getTiers(t).Instruments
	first := tierRequest{Order: 1, Name: "Sample I", TargetShare: "60", SharedPartName: "Sample shared"}
	second := tierRequest{Order: 2, Name: "Sample II", TargetShare: "40"}
	for i, o := range options {
		switch {
		case i == 0:
			first.Instruments = append(first.Instruments, assignmentRequest{Ticker: o.Ticker, TargetShare: share("30")})
		case i%2 == 1:
			first.Instruments = append(first.Instruments, assignmentRequest{Ticker: o.Ticker})
		default:
			second.Instruments = append(second.Instruments, assignmentRequest{Ticker: o.Ticker})
		}
	}
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{first, second}})

	var got rebalancingResponse
	srv.getRealJSON(t, "/api/all-accounts/rebalancing", &got)

	require.True(t, got.Available, "the table is available")
	require.Len(t, got.Tiers, 2)
	require.NotNil(t, got.Value)
	sum := decimal.RequireFromString(got.NoTier.Value)
	for _, tr := range got.Tiers {
		sum = sum.Add(decimal.RequireFromString(tr.Value))
	}
	assert.True(t, sum.Equal(decimal.RequireFromString(*got.Value)), "the Tiers and the No Tier group add up to the value")
}
