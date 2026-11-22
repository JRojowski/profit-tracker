package httpapi_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken"
)

// The real Kraken test reads the user's Kraken ledger export. It obeys the rules of the real-export tests,
// and more strictly: a failure message shows no amount, no time and no reference ID from the file.

// realKrakenExport returns the content of the first Kraken ledger export (*.csv) in the exports folder.
// It skips the test if the folder has no Kraken export.
func realKrakenExport(t *testing.T) []byte {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(exportsDir(t), "*.csv"))
	require.NoError(t, err)
	slices.Sort(paths)
	for i, p := range paths {
		data, err := os.ReadFile(p)
		require.NoError(t, err, "read csv file %d", i+1)
		if kraken.IsExport(data) {
			return data
		}
	}
	t.Skipf("no Kraken ledger export (*.csv) in the exports folder: put it in \"exports/\" at the repository root, or set %s", exportsEnv)
	return nil
}

// lastSpotBalances returns the last balance of each asset of the spot wallet in the file, by the asset code
// of the tool. The old code XBT counts as BTC.
func lastSpotBalances(t *testing.T, data []byte) map[string]decimal.Decimal {
	t.Helper()

	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	header, err := r.Read()
	require.NoError(t, err, "read the header row")
	column := func(name string) int {
		i := slices.Index(header, name)
		require.GreaterOrEqual(t, i, 0, "the header row has no column %q", name)
		return i
	}
	assetCol, walletCol, balanceCol := column("asset"), column("wallet"), column("balance")
	balances := map[string]decimal.Decimal{}
	for line := 2; ; line++ {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			return balances
		}
		require.NoError(t, err, "read row %d", line)
		if record[walletCol] != "spot / main" {
			continue
		}
		balance, err := decimal.NewFromString(record[balanceCol])
		// The message has the row number only, not the value.
		require.NoError(t, err, "row %d: the balance is not a decimal", line)
		asset := record[assetCol]
		if asset == "XBT" {
			asset = "BTC"
		}
		balances[asset] = balance
	}
}

// TestRealExports_KrakenExport_HasNoUnsupportedEntryAndMatchesTheBalance imports the real Kraken export.
// The Import has no unsupported entry. The Bitcoin quantity agrees with the last BTC balance in the file.
func TestRealExports_KrakenExport_HasNoUnsupportedEntryAndMatchesTheBalance(t *testing.T) {
	t.Parallel()
	data := realKrakenExport(t)
	srv := startServer(t)

	status, body := srv.upload(t, "kraken_real-export.csv", data)
	if status != http.StatusOK {
		var rejected unsupportedEntriesError
		_ = json.Unmarshal([]byte(body), &rejected)
		reasons := map[string]int{}
		for _, e := range rejected.UnsupportedEntries {
			reasons[e.Type+"/"+e.Subtype+": "+e.Reason]++
		}
		// The message shows only the code and the count of each kind of entry, not the times or the amounts.
		require.FailNow(t, fmt.Sprintf("preview of the Kraken export: status %d, code %q, unsupported entries %v",
			status, rejected.Code, reasons))
	}
	var preview krakenPreviewResponse
	require.NoError(t, json.Unmarshal([]byte(body), &preview), "the preview is not valid JSON")
	require.Empty(t, preview.Conflicts, "a new Account has no Import Conflict")
	path := fmt.Sprintf("/api/imports/%d/confirm", preview.ID)
	status, body = srv.postJSON(t, path, newAccountBody(preview.SuggestedName, preview.SuggestedType, preview.SuggestedCurrency))
	require.Equal(t, http.StatusOK, status, "confirm of the Kraken export: %s", apiError(body))
	var saved importResponse
	require.NoError(t, json.Unmarshal([]byte(body), &saved), "the confirm is not valid JSON")

	var holdings []holdingResponse
	srv.getRealJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", saved.AccountID), &holdings)

	want, ok := lastSpotBalances(t, data)["BTC"]
	require.True(t, ok, "the file has no BTC balance")
	i := slices.IndexFunc(holdings, func(h holdingResponse) bool { return h.Ticker == "BTC" })
	require.GreaterOrEqual(t, i, 0, "the Kraken Account has no BTC Holding")
	got, err := decimal.NewFromString(holdings[i].Volume)
	require.NoError(t, err, "the BTC volume is not a decimal")
	// The message has no amount, because the amounts are personal data.
	assert.True(t, want.Equal(got), "the BTC quantity does not agree with the last BTC balance in the file")
}
