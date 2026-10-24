package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
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

// checkSales checks that the sales and the Closed Positions rows are one to one, and that the Realized Profit
// of each sale is within 0.01 of the Profit/Loss of its row.
// salePrices are the prices of the Stock sell Cash Operations of the export, by XTB ID.
//
// A sale and its row have the same position ID and the same closed volume. One Lot can close in parts, so more
// than one row can fit. Then the row with the close time nearest to the sale time wins. The API gives the sale
// time in whole seconds, and the sheet can have milliseconds. On a tie, the row with the Close Price nearest
// to the sale price wins. The check does not use the Realized Profit to select a row, because that value is
// the value under test.
func checkSales(t *testing.T, sales []saleResponse, salePrices map[string]decimal.Decimal, rows []xtb.ClosedPosition) {
	t.Helper()

	used := make([]bool, len(rows))
	for _, sale := range sales {
		volume := decimal.RequireFromString(sale.Volume)
		profit := decimal.RequireFromString(sale.RealizedProfit)
		at, err := time.Parse(time.RFC3339, sale.Time)
		require.NoError(t, err, "time of sale %s", sale.XTBID)
		price, hasPrice := salePrices[sale.XTBID]

		best := -1
		for i, row := range rows {
			if used[i] || row.PositionID != sale.PositionID || !row.Volume.Equal(volume) {
				continue
			}
			if best < 0 || fitsBetter(row, rows[best], at, decimal.NullDecimal{Decimal: price, Valid: hasPrice}) {
				best = i
			}
		}
		if best < 0 {
			t.Errorf("sale %s of Lot %s (volume %s) has no Closed Positions row", sale.XTBID, sale.PositionID, sale.Volume)
			continue
		}
		used[best] = true
		broker := rows[best].ProfitLoss
		if profit.Sub(broker).Abs().GreaterThan(realCheckTolerance) {
			t.Errorf("sale %s of Lot %s: Realized Profit %s, Closed Positions Profit/Loss %s", sale.XTBID, sale.PositionID, profit, broker)
		}
	}
	for i, row := range rows {
		if !used[i] {
			t.Errorf("Closed Positions row %d of Lot %s (volume %s, Profit/Loss %s) has no sale", row.Row, row.PositionID, row.Volume, row.ProfitLoss)
		}
	}
}

// fitsBetter returns true if the row a is a better match than the row b for a sale with the time at and the price.
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

// salePrices returns the prices of the Stock sell Cash Operations, by XTB ID.
func salePrices(ops []xtb.CashOperation) map[string]decimal.Decimal {
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
// The open volume of a Lot is the sum of its Fill volumes minus the closed volumes of its sales from the API.
func checkOpenLots(t *testing.T, db *store.Store, accountID int64, sales []saleResponse, openLots []xtb.OpenLot) {
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
	for _, sale := range sales {
		calculated[sale.PositionID] = calculated[sale.PositionID].Sub(decimal.RequireFromString(sale.Volume))
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
			var sales []saleResponse
			srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/sales", accountID), &sales)
			var holdings []holdingResponse
			srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &holdings)

			t.Run("each sale matches Closed Positions", func(t *testing.T) {
				t.Parallel()
				checkSales(t, sales, salePrices(sheets.CashOperations), sheets.ClosedPositions)
			})
			t.Run("each open Lot matches Open Positions", func(t *testing.T) {
				t.Parallel()
				checkOpenLots(t, srv.db, accountID, sales, sheets.OpenLots)
			})
			t.Run("each Holding matches Open Positions", func(t *testing.T) {
				t.Parallel()
				checkHoldings(t, holdings, sheets.OpenLots)
			})
		})
	}
}
