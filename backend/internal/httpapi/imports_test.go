package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// previewResponse is the body of POST /api/imports.
type previewResponse struct {
	ID                    int64            `json:"id"`
	FileName              string           `json:"fileName"`
	AccountNumber         string           `json:"accountNumber"`
	Account               *accountResponse `json:"account"`
	SuggestedCurrency     string           `json:"suggestedCurrency"`
	NewCashOperations     int              `json:"newCashOperations"`
	SkippedCashOperations int              `json:"skippedCashOperations"`
	State                 string           `json:"state"`
	Conflicts             []importConflict `json:"conflicts"`
}

// accountResponse is an Account in an API response.
type accountResponse struct {
	ID            int64  `json:"id"`
	AccountNumber string `json:"accountNumber"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Currency      string `json:"currency"`
}

// importResponse is one saved Import in GET /api/imports and in the confirm response.
type importResponse struct {
	ID                    int64  `json:"id"`
	FileName              string `json:"fileName"`
	ImportedAt            string `json:"importedAt"`
	AccountID             int64  `json:"accountId"`
	AccountNumber         string `json:"accountNumber"`
	AccountName           string `json:"accountName"`
	NewCashOperations     int    `json:"newCashOperations"`
	SkippedCashOperations int    `json:"skippedCashOperations"`
}

type errorBody struct {
	Error string `json:"error"`
}

// usdExport is a small export of a USD Account: one Deposit and two purchases.
// The rows have fixed times, so that a newer export with more rows gives the same data for them.
//
//	Cash:         1000.00 - 109.40 - 116.58 = 774.02
//	Net Deposits: 1000.00
//	FPS.US:       volume 10, cost 109.40
//	PENG.US:      volume 2, cost 116.58
func usdExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000001",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "PENG.US", Time: opTime(3), Amount: "-116.58", ID: "103", Comment: "OPEN BUY PENG.US 2 @ 58.29", PositionID: "2000000002"},
			{Type: "Stock purchase", Ticker: "FPS.US", Time: opTime(2), Amount: "-109.40", ID: "102", Comment: "OPEN BUY 10 @ 10.94", PositionID: "2000000001"},
			{Type: "Deposit", Time: opTime(1), Amount: "1000.00", ID: "101", Comment: "Blik deposit"},
		},
	}
}

// opTime returns the time of a Cash Operation on day n of February 2026, at 10:00 UTC.
func opTime(day int) time.Time {
	return time.Date(2026, 2, day, 10, 0, 0, 0, time.UTC)
}

// preview uploads the export and returns the preview. It stops the test if the status is not 200.
func (s *testServer) preview(t *testing.T, export xtbtest.Export) previewResponse {
	t.Helper()

	status, body := s.upload(t, export.FileName(), xtbtest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got previewResponse
	decode(t, body, &got)
	return got
}

// imports returns the list of saved Imports.
func (s *testServer) imports(t *testing.T) []importResponse {
	t.Helper()

	status, _, body := s.get(t, "/api/imports")
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got []importResponse
	decode(t, body, &got)
	return got
}

func TestImportPreview_NewAccount_SuggestsCurrencyAndSavesNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := usdExport()

	got := srv.preview(t, export)

	assert.Positive(t, got.ID)
	assert.Equal(t, "USD_56000001_2006-01-01_2026-10-22.xlsx", got.FileName)
	assert.Equal(t, "56000001", got.AccountNumber)
	assert.Nil(t, got.Account, "the account number is new")
	assert.Equal(t, "USD", got.SuggestedCurrency)
	assert.Equal(t, 3, got.NewCashOperations)
	assert.Equal(t, 0, got.SkippedCashOperations)

	status, _, body := srv.get(t, "/api/imports")
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, "[]", body, "a preview saves nothing")
}

// newAccountBody is a valid confirm body for a new Account.
func newAccountBody(name, typ, currency string) map[string]any {
	return map[string]any{"account": map[string]string{"name": name, "type": typ, "currency": currency}}
}

// confirm confirms the Import and returns the saved Import. It stops the test if the status is not 200.
func (s *testServer) confirm(t *testing.T, id int64, body any) importResponse {
	t.Helper()

	status, respBody := s.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", id), body)
	require.Equal(t, http.StatusOK, status, "body %q", respBody)
	var got importResponse
	decode(t, respBody, &got)
	return got
}

func TestImportConfirm_NewAccount_SavesAccountAndListsImport(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, usdExport())

	got := srv.confirm(t, preview.ID, newAccountBody("XTB USD", "regular", "USD"))

	assert.Equal(t, preview.ID, got.ID)
	assert.Equal(t, "USD_56000001_2006-01-01_2026-10-22.xlsx", got.FileName)
	assert.Positive(t, got.AccountID)
	assert.Equal(t, "56000001", got.AccountNumber)
	assert.Equal(t, "XTB USD", got.AccountName)
	assert.Equal(t, 3, got.NewCashOperations)
	assert.Equal(t, 0, got.SkippedCashOperations)
	importedAt, err := time.Parse(time.RFC3339, got.ImportedAt)
	require.NoError(t, err)
	assert.Equal(t, time.UTC, importedAt.Location())
	assert.WithinDuration(t, time.Now(), importedAt, time.Minute)

	assert.Equal(t, []importResponse{got}, srv.imports(t))

	again := srv.preview(t, usdExport())
	require.NotNil(t, again.Account, "the account number is known after the confirmation")
	assert.Equal(t, accountResponse{ID: got.AccountID, AccountNumber: "56000001", Name: "XTB USD", Type: "regular", Currency: "USD"}, *again.Account)
}

func TestImportConfirm_KnownAccount_GoesToSameAccountWithNoBody(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))

	// A newer export of the same Account: the old rows and one new purchase.
	newer := usdExport()
	newer.To = time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	newer.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "LSMC.DE", Time: opTime(4), Amount: "-126.35", ID: "104", Comment: "OPEN BUY 0.6169 @ 47.755", PositionID: "2000000003"},
	}, newer.CashOperations...)
	preview := srv.preview(t, newer)

	require.NotNil(t, preview.Account)
	assert.Equal(t, first.AccountID, preview.Account.ID)
	assert.Equal(t, 1, preview.NewCashOperations)
	assert.Equal(t, 3, preview.SkippedCashOperations)

	second := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, first.AccountID, second.AccountID)
	assert.Equal(t, "XTB USD", second.AccountName)
	assert.Equal(t, 1, second.NewCashOperations)
	assert.Equal(t, 3, second.SkippedCashOperations)
	assert.Equal(t, []importResponse{second, first}, srv.imports(t), "the newest Import first")
}

func TestImportPreview_RejectedFile_ReturnsErrorAndSavesNothing(t *testing.T) {
	t.Parallel()

	withRow := func(op xtbtest.CashOperation) []byte {
		export := usdExport()
		export.CashOperations = append(export.CashOperations, op)
		return xtbtest.Build(t, export)
	}
	totalMismatch := usdExport()
	totalMismatch.CashTotal = "774.03"
	brokenZip := []byte("PK\x03\x04 not a zip")

	tests := []struct {
		name      string
		data      []byte
		wantError []string
	}{
		{
			name:      "unknown type names the type and the row",
			data:      withRow(xtbtest.CashOperation{Type: "Stock split", Ticker: "FPS.US", Amount: "0.00", ID: "999", Comment: "FPS.US 1:10 split"}),
			wantError: []string{"unknown cash operation type", `"Stock split"`, "row 9"},
		},
		{
			name:      "type that no broker has",
			data:      withRow(xtbtest.CashOperation{Type: "Magic bonus", Amount: "1.00"}),
			wantError: []string{`"Magic bonus"`, "row 9"},
		},
		{
			name:      "text file",
			data:      []byte("Type,Amount\nDeposit,100\n"),
			wantError: []string{"not an XTB export"},
		},
		{
			name:      "broken zip",
			data:      brokenZip,
			wantError: []string{"not an XTB export"},
		},
		{
			name:      "xlsx with no Cash Operations sheet",
			data:      emptyWorkbook(t),
			wantError: []string{"not an XTB export", "Cash Operations"},
		},
		{
			name:      "Total row mismatch",
			data:      xtbtest.Build(t, totalMismatch),
			wantError: []string{"Total row", "774.02", "774.03"},
		},
		{
			name:      "purchase comment in an unknown form",
			data:      withRow(xtbtest.CashOperation{Type: "Stock purchase", Ticker: "FPS.US", Amount: "-1.00", Comment: "BUY SOME", PositionID: "2000000009"}),
			wantError: []string{"row 9", `"BUY SOME"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)

			status, body := srv.upload(t, "USD_56000001_2006-01-01_2026-10-22.xlsx", tc.data)

			assert.Equal(t, http.StatusBadRequest, status)
			var got errorBody
			decode(t, body, &got)
			for _, want := range tc.wantError {
				assert.Contains(t, got.Error, want)
			}
			assert.Empty(t, srv.imports(t))
		})
	}
}

func TestImportPreview_PurchaseCommentForms_AreAccepted(t *testing.T) {
	t.Parallel()

	// Each case is one full order. A Comment with an order volume is one Fill of an order with two Fills.
	tests := []struct {
		name     string
		comments []string
	}{
		{name: "whole volume", comments: []string{"OPEN BUY 2 @ 58.29"}},
		{name: "whole volume with ticker", comments: []string{"OPEN BUY PENG.US 2 @ 58.29"}},
		{name: "fills with order volume", comments: []string{"OPEN BUY 0.6169/1.6169 @ 47.755", "OPEN BUY 1/1.6169 @ 47.705"}},
		{name: "fills with ticker and order volume", comments: []string{"OPEN BUY PENG.US 0.27/1.27 @ 39.37", "OPEN BUY PENG.US 1/1.27 @ 39.24"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			var export xtbtest.Export
			for i, comment := range tc.comments {
				// The Fills of one order follow each other within seconds and share one Position ID (ADR 0002, case A).
				export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{
					Type: "Stock purchase", Ticker: "PENG.US", Time: opTime(2).Add(time.Duration(i) * time.Second),
					Amount: "-116.58", Comment: comment, PositionID: "2000000002",
				})
			}

			got := srv.preview(t, export)

			assert.Equal(t, len(tc.comments), got.NewCashOperations)
		})
	}
}

func TestImportPreview_NoFileField_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, body := srv.postJSON(t, "/api/imports", map[string]string{"file": "x"})

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, body, `"error"`)
}

func TestImportConfirm_NewAccount_InvalidAccountData_ReturnsBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      any
		wantError string
	}{
		{name: "no body", body: nil, wantError: "name"},
		{name: "empty name", body: newAccountBody("  ", "regular", "USD"), wantError: "name"},
		{name: "unknown type", body: newAccountBody("XTB USD", "savings", "USD"), wantError: `"savings"`},
		{name: "lowercase currency", body: newAccountBody("XTB USD", "IKE", "usd"), wantError: `"usd"`},
		{name: "long currency", body: newAccountBody("XTB USD", "IKZE", "USDT"), wantError: `"USDT"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			preview := srv.preview(t, usdExport())

			status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), tc.body)

			assert.Equal(t, http.StatusBadRequest, status)
			var got errorBody
			decode(t, body, &got)
			assert.Contains(t, got.Error, tc.wantError)
			assert.Empty(t, srv.imports(t))
		})
	}
}

func TestImportConfirm_UnknownID_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, id := range []string{"42", "abc"} {
		status, body := srv.postJSON(t, "/api/imports/"+id+"/confirm", newAccountBody("XTB USD", "regular", "USD"))

		assert.Equal(t, http.StatusNotFound, status, "id %s", id)
		assert.Contains(t, body, `"error"`)
	}
}

func TestImportConfirm_Twice_ReturnsConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, usdExport())
	first := srv.confirm(t, preview.ID, newAccountBody("XTB USD", "regular", "USD"))

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), nil)

	assert.Equal(t, http.StatusConflict, status)
	assert.Contains(t, body, `"error"`)
	assert.Equal(t, []importResponse{first}, srv.imports(t))
}

func TestImportConfirm_TwoPreviewsOfNewAccountAtSameTime_GoToOneAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	// The race is between the account lookup and the save. Many rounds make it occur.
	for round := range 50 {
		export := usdExport()
		export.AccountNumber = fmt.Sprintf("5700%04d", round)
		previews := []previewResponse{srv.preview(t, export), srv.preview(t, export)}

		results := make([]confirmResult, len(previews))
		var wg sync.WaitGroup
		for i, preview := range previews {
			wg.Go(func() {
				results[i] = srv.confirmAsync(t.Context(), preview.ID, newAccountBody(fmt.Sprintf("XTB USD %d", i), "regular", "USD"))
			})
		}
		wg.Wait()

		var saved []importResponse
		for _, res := range results {
			require.NoError(t, res.err, "round %d", round)
			require.Equal(t, http.StatusOK, res.status, "round %d, body %q", round, res.body)
			var got importResponse
			decode(t, res.body, &got)
			saved = append(saved, got)
		}
		assert.Equal(t, saved[0].AccountID, saved[1].AccountID, "round %d: both imports go to one account", round)
		assert.Equal(t, 3, saved[0].NewCashOperations+saved[1].NewCashOperations, "round %d", round)
	}
}

// confirmResult is the result of one confirm request in a goroutine.
type confirmResult struct {
	status int
	body   string
	err    error
}

// confirmAsync confirms the Import. It is safe in a goroutine because it does not stop the test.
func (s *testServer) confirmAsync(ctx context.Context, id int64, body any) confirmResult {
	data, err := json.Marshal(body)
	if err != nil {
		return confirmResult{err: err}
	}
	url := fmt.Sprintf("%s/api/imports/%d/confirm", s.URL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return confirmResult{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return confirmResult{err: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return confirmResult{status: resp.StatusCode, body: string(respBody), err: err}
}

// emptyWorkbook returns an xlsx file with one empty sheet.
func emptyWorkbook(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { assert.NoError(t, f.Close()) }()
	var buf bytes.Buffer
	require.NoError(t, f.Write(&buf))
	return buf.Bytes()
}

// importConflict is one Import Conflict in the preview and in the decision response.
type importConflict struct {
	ID       int64             `json:"id"`
	Kind     string            `json:"kind"`
	XTBID    string            `json:"xtbId"`
	Existing cashOperationData `json:"existing"`
	New      cashOperationData `json:"new"`
	Decision *string           `json:"decision"`
	// Check is null for the kind "changed-data".
	Check *checkData `json:"check"`
}

// cashOperationData is the data of a Cash Operation that an Import Conflict compares.
type cashOperationData struct {
	Type       string  `json:"type"`
	Time       string  `json:"time"`
	Amount     string  `json:"amount"`
	Comment    string  `json:"comment"`
	Ticker     string  `json:"ticker"`
	PositionID string  `json:"positionId"`
	Volume     *string `json:"volume"`
	Price      *string `json:"price"`
}

// deposit101 is the data of the Deposit 101 of usdExport.
func deposit101() cashOperationData {
	return cashOperationData{Type: "Deposit", Time: "2026-02-01T10:00:00Z", Amount: "1000.00", Comment: "Blik deposit"}
}

// purchase102 is the data of the purchase 102 of usdExport.
func purchase102() cashOperationData {
	return cashOperationData{
		Type: "Stock purchase", Time: "2026-02-02T10:00:00Z", Amount: "-109.40", Comment: "OPEN BUY 10 @ 10.94",
		Ticker: "FPS.US", PositionID: "2000000001", Volume: ptr("10"), Price: ptr("10.94"),
	}
}

// with returns d with the change applied.
func (d cashOperationData) with(change func(*cashOperationData)) cashOperationData {
	change(&d)
	return d
}

func ptr(s string) *string { return &s }

// summary returns the summary of the Account.
func (s *testServer) summary(t *testing.T, accountID int64) summaryResponse {
	t.Helper()

	var got summaryResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// holdings returns the Holdings of the Account.
func (s *testServer) holdings(t *testing.T, accountID int64) []holdingResponse {
	t.Helper()

	var got []holdingResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// decide records the decision for the Import Conflict. It stops the test if the status is not 200.
func (s *testServer) decide(t *testing.T, importID, conflictID int64, decision string) importConflict {
	t.Helper()

	status, body := s.postJSON(t, fmt.Sprintf("/api/imports/%d/conflicts/%d", importID, conflictID), map[string]string{"decision": decision})
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got importConflict
	decode(t, body, &got)
	return got
}

// withChange returns usdExport with the change applied to the row with the XTB ID.
func withChange(t *testing.T, xtbID string, change func(*xtbtest.CashOperation)) xtbtest.Export {
	t.Helper()

	export := usdExport()
	for i := range export.CashOperations {
		if export.CashOperations[i].ID == xtbID {
			change(&export.CashOperations[i])
			return export
		}
	}
	require.FailNow(t, "no row with the XTB ID", xtbID)
	return export
}

// usdSummary is the summary of the Account with the ID after the import of usdExport.
func usdSummary(accountID int64) summaryResponse {
	return noResultSummary(accountID, "774.02", "1000.00")
}

// noResultSummary is the summary of a USD Account with no sales, no income and no costs.
// Each part of the result and the total result are zero.
func noResultSummary(accountID int64, cash, netDeposits string) summaryResponse {
	return summaryResponse{
		AccountID: accountID, Currency: "USD", Cash: cash, NetDeposits: netDeposits,
		RealizedProfit: "0.00", NetDividends: "0.00", NetInterest: "0.00", Fees: "0.00", TotalResult: "0.00",
	}
}

// usdHoldings are the Holdings of the Account after the import of usdExport.
var usdHoldings = []holdingResponse{
	{Ticker: "FPS.US", Name: "FPS.US", Volume: "10", Cost: "109.40", AverageCost: "10.94"},
	{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
}

func TestImport_SameFileTwice_AddsNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")

	status, body := srv.upload(t, usdExport().FileName(), xtbtest.Build(t, usdExport()))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var preview previewResponse
	decode(t, body, &preview)

	assert.Equal(t, "preview", preview.State)
	assert.Contains(t, body, `"conflicts":[]`, "the conflicts are an empty array, not null")
	assert.Equal(t, 0, preview.NewCashOperations)
	assert.Equal(t, 3, preview.SkippedCashOperations)

	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 0, saved.NewCashOperations)
	assert.Equal(t, 3, saved.SkippedCashOperations)
	assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))
	assert.Equal(t, usdHoldings, srv.holdings(t, accountID))
}

func TestImport_OverlappingNewerFile_AddsOnlyNewCashOperations(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")

	// A full-history export, one month later: the three known rows and two new rows.
	newer := usdExport()
	newer.To = time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	newer.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "FPS.US", Time: opTime(5), Amount: "-21.88", ID: "105", Comment: "OPEN BUY 2 @ 10.94", PositionID: "2000000004"},
		{Type: "Deposit", Time: opTime(4), Amount: "500.00", ID: "104", Comment: "Blik deposit"},
	}, newer.CashOperations...)

	preview := srv.preview(t, newer)
	assert.Equal(t, "preview", preview.State)
	assert.Empty(t, preview.Conflicts)
	assert.Equal(t, 2, preview.NewCashOperations)
	assert.Equal(t, 3, preview.SkippedCashOperations)

	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 2, saved.NewCashOperations)
	assert.Equal(t, 3, saved.SkippedCashOperations)
	assert.Equal(t, noResultSummary(accountID, "1252.14", "1500.00"), srv.summary(t, accountID))
	assert.Equal(t, []holdingResponse{
		{Ticker: "FPS.US", Name: "FPS.US", Volume: "12", Cost: "131.28", AverageCost: "10.94"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportPreview_KnownIDWithChangedData_IsImportConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		xtbID    string
		change   func(*xtbtest.CashOperation)
		existing cashOperationData
		new      cashOperationData
	}{
		{
			name:     "amount",
			xtbID:    "101",
			change:   func(op *xtbtest.CashOperation) { op.Amount = "1200.00" },
			existing: deposit101(),
			new:      deposit101().with(func(d *cashOperationData) { d.Amount = "1200.00" }),
		},
		{
			name:     "time",
			xtbID:    "101",
			change:   func(op *xtbtest.CashOperation) { op.Time = op.Time.Add(time.Second) },
			existing: deposit101(),
			new:      deposit101().with(func(d *cashOperationData) { d.Time = "2026-02-01T10:00:01Z" }),
		},
		{
			name:  "type",
			xtbID: "102",
			change: func(op *xtbtest.CashOperation) {
				op.Type, op.Ticker, op.PositionID = "Deposit", "", ""
			},
			existing: purchase102(),
			new: purchase102().with(func(d *cashOperationData) {
				d.Type, d.Ticker, d.PositionID, d.Volume, d.Price = "Deposit", "", "", nil, nil
			}),
		},
		{
			name:     "comment",
			xtbID:    "102",
			change:   func(op *xtbtest.CashOperation) { op.Comment = "OPEN BUY 12 @ 9.1167" },
			existing: purchase102(),
			new: purchase102().with(func(d *cashOperationData) {
				d.Comment, d.Volume, d.Price = "OPEN BUY 12 @ 9.1167", ptr("12"), ptr("9.1167")
			}),
		},
		{
			name:     "ticker",
			xtbID:    "102",
			change:   func(op *xtbtest.CashOperation) { op.Ticker = "LSMC.DE" },
			existing: purchase102(),
			new:      purchase102().with(func(d *cashOperationData) { d.Ticker = "LSMC.DE" }),
		},
		{
			name:     "position ID",
			xtbID:    "102",
			change:   func(op *xtbtest.CashOperation) { op.PositionID = "2000000009" },
			existing: purchase102(),
			new:      purchase102().with(func(d *cashOperationData) { d.PositionID = "2000000009" }),
		},
		// The reader takes the volume and the price from the Comment. Thus a file cannot change only
		// the volume or only the price. TestCompare in the importer package covers these two fields.
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			srv.importAccount(t, usdExport(), "XTB USD")

			got := srv.preview(t, withChange(t, tc.xtbID, tc.change))

			assert.Equal(t, "needs-review", got.State)
			assert.Equal(t, 0, got.NewCashOperations)
			assert.Equal(t, 2, got.SkippedCashOperations, "the conflict is neither new nor skipped")
			require.Len(t, got.Conflicts, 1)
			conflict := got.Conflicts[0]
			assert.Positive(t, conflict.ID)
			assert.Equal(t, "changed-data", conflict.Kind)
			assert.Equal(t, tc.xtbID, conflict.XTBID)
			assert.Equal(t, tc.existing, conflict.Existing)
			assert.Equal(t, tc.new, conflict.New)
			assert.Nil(t, conflict.Decision)
		})
	}
}

func TestImportPreview_ImportConflict_HasContractJSON(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, usdExport(), "XTB USD")
	export := withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200" })

	status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got struct {
		State     string            `json:"state"`
		Conflicts []json.RawMessage `json:"conflicts"`
	}
	decode(t, body, &got)

	assert.Equal(t, "needs-review", got.State)
	require.Len(t, got.Conflicts, 1)
	var conflict importConflict
	decode(t, string(got.Conflicts[0]), &conflict)
	assert.JSONEq(t, fmt.Sprintf(`{
		"id": %d,
		"kind": "changed-data",
		"xtbId": "101",
		"existing": {"type": "Deposit", "time": "2026-02-01T10:00:00Z", "amount": "1000.00", "comment": "Blik deposit",
			"ticker": "", "positionId": "", "volume": null, "price": null},
		"new": {"type": "Deposit", "time": "2026-02-01T10:00:00Z", "amount": "1200.00", "comment": "Blik deposit",
			"ticker": "", "positionId": "", "volume": null, "price": null},
		"check": null,
		"decision": null
	}`, conflict.ID), string(got.Conflicts[0]))
}

func TestImportPreview_SameAmountWithOtherDecimalPlaces_IsSkipped(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, usdExport(), "XTB USD")

	got := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1000" }))

	assert.Equal(t, "preview", got.State)
	assert.Empty(t, got.Conflicts)
	assert.Equal(t, 3, got.SkippedCashOperations)
}

func TestImportConfirm_KeepExisting_LeavesLedgerUnchanged(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	require.Len(t, preview.Conflicts, 1)

	decided := srv.decide(t, preview.ID, preview.Conflicts[0].ID, "keep-existing")
	require.NotNil(t, decided.Decision)
	assert.Equal(t, "keep-existing", *decided.Decision)

	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 0, saved.NewCashOperations)
	assert.Equal(t, 3, saved.SkippedCashOperations)
	assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))
	assert.Equal(t, usdHoldings, srv.holdings(t, accountID))
}

func TestImportConfirm_UseNew_ReplacesStoredCashOperation(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 1, saved.NewCashOperations)
	assert.Equal(t, 2, saved.SkippedCashOperations)
	assert.Equal(t, noResultSummary(accountID, "974.02", "1200.00"), srv.summary(t, accountID))

	again := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	assert.Equal(t, "preview", again.State, "the stored Cash Operation has the new data")
	assert.Empty(t, again.Conflicts)
}

func TestImportConfirm_UseNew_PurchaseWithChangedVolume_ChangesHolding(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) { op.Comment = "OPEN BUY 12 @ 9.1167" }))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	srv.confirm(t, preview.ID, nil)

	assert.Equal(t, []holdingResponse{
		{Ticker: "FPS.US", Name: "FPS.US", Volume: "12", Cost: "109.40", AverageCost: "9.1166666666666667"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportConfirm_UseNew_PurchaseWithOtherInstrument_MovesFill(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) {
		op.Ticker, op.Comment, op.PositionID = "LSMC.DE", "OPEN BUY LSMC.DE 2 @ 54.70", "2000000005"
	}))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	srv.confirm(t, preview.ID, nil)

	assert.Equal(t, []holdingResponse{
		{Ticker: "LSMC.DE", Name: "LSMC.DE", Volume: "2", Cost: "109.40", AverageCost: "54.7"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportConfirm_UseNew_OnlyTickerColumnChanged_HoldingHasNewInstrument(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	// The Comment has no ticker, so only the Ticker column changes.
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) { op.Ticker = "LSMC.DE" }))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 1, saved.NewCashOperations)
	assert.Equal(t, []holdingResponse{
		{Ticker: "LSMC.DE", Name: "LSMC.DE", Volume: "10", Cost: "109.40", AverageCost: "10.94"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportConfirm_KeepExisting_OnlyTickerColumnChanged_HoldingStays(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) { op.Ticker = "LSMC.DE" }))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "keep-existing")
	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, 0, saved.NewCashOperations)
	assert.Equal(t, 3, saved.SkippedCashOperations)
	assert.Equal(t, usdHoldings, srv.holdings(t, accountID))
}

func TestImportConfirm_UseNew_SameLotWithOtherTicker_MovesLotToNewInstrument(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	// Only the ticker changes. The position ID, and thus the Lot, stays the same.
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) {
		op.Ticker, op.Comment = "LSMC.DE", "OPEN BUY LSMC.DE 10 @ 10.94"
	}))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	srv.confirm(t, preview.ID, nil)

	assert.Equal(t, []holdingResponse{
		{Ticker: "LSMC.DE", Name: "LSMC.DE", Volume: "10", Cost: "109.40", AverageCost: "10.94"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

// twoFillExport is usdExport with a second Fill 104 in the Lot 2000000001 of FPS.US.
func twoFillExport() xtbtest.Export {
	export := usdExport()
	export.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "FPS.US", Time: opTime(4), Amount: "-21.88", ID: "104", Comment: "OPEN BUY 2 @ 10.94", PositionID: "2000000001"},
	}, export.CashOperations...)
	return export
}

// toLSMC changes a Fill of the Lot 2000000001 to the Instrument LSMC.DE, with the same position ID.
func toLSMC(volume string) func(*xtbtest.CashOperation) {
	return func(op *xtbtest.CashOperation) {
		op.Ticker, op.Comment = "LSMC.DE", "OPEN BUY LSMC.DE "+volume+" @ 10.94"
	}
}

func TestImportConfirm_UseNew_AllFillsOfLotWithOtherTicker_MovesLotToNewInstrument(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, twoFillExport(), "XTB USD")
	export := twoFillExport()
	toLSMC("2")(&export.CashOperations[0])
	toLSMC("10")(&export.CashOperations[2])
	preview := srv.preview(t, export)
	require.Len(t, preview.Conflicts, 2)

	for _, conflict := range preview.Conflicts {
		srv.decide(t, preview.ID, conflict.ID, "use-new")
	}
	srv.confirm(t, preview.ID, nil)

	assert.Equal(t, []holdingResponse{
		{Ticker: "LSMC.DE", Name: "LSMC.DE", Volume: "12", Cost: "131.28", AverageCost: "10.94"},
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportConfirm_UseNew_LotWithFillsOfTwoInstruments_IsRejected(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, twoFillExport(), "XTB USD")
	before := srv.holdings(t, accountID)
	// Only the Fill 102 changes. The Fill 104 of the same Lot stays FPS.US.
	export := twoFillExport()
	toLSMC("10")(&export.CashOperations[2])
	preview := srv.preview(t, export)
	require.Len(t, preview.Conflicts, 1)
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), nil)

	assert.Equal(t, http.StatusBadRequest, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "2000000001")
	assert.Contains(t, got.Error, "more than one instrument")
	assert.Equal(t, before, srv.holdings(t, accountID))
	assert.Len(t, srv.imports(t), 1)
}

func TestImportConfirm_UseNew_PurchaseThatIsNoLongerAPurchase_RemovesFill(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "102", func(op *xtbtest.CashOperation) {
		op.Type, op.Ticker, op.PositionID = "Deposit", "", ""
	}))
	require.Len(t, preview.Conflicts, 1)

	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")
	srv.confirm(t, preview.ID, nil)

	assert.Equal(t, []holdingResponse{
		{Ticker: "PENG.US", Name: "PENG.US", Volume: "2", Cost: "116.58", AverageCost: "58.29"},
	}, srv.holdings(t, accountID))
}

func TestImportConfirm_OpenConflicts_ReturnsConflict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		decide    int
		wantError string
	}{
		{name: "no decisions", decide: 0, wantError: "2 import conflicts have no decision"},
		{name: "one decision", decide: 1, wantError: "1 import conflict has no decision"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			accountID := srv.importAccount(t, usdExport(), "XTB USD")
			export := withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" })
			export.CashOperations[0].Amount = "-120.00"
			preview := srv.preview(t, export)
			require.Len(t, preview.Conflicts, 2)
			for _, conflict := range preview.Conflicts[:tc.decide] {
				srv.decide(t, preview.ID, conflict.ID, "use-new")
			}

			status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), nil)

			assert.Equal(t, http.StatusConflict, status)
			var got errorBody
			decode(t, body, &got)
			assert.Contains(t, got.Error, tc.wantError)
			assert.Len(t, srv.imports(t), 1)
			assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))
		})
	}
}

func TestImportDecision_ChangedDecision_IsRecorded(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	preview := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	require.Len(t, preview.Conflicts, 1)
	conflict := preview.Conflicts[0]

	srv.decide(t, preview.ID, conflict.ID, "use-new")
	got := srv.decide(t, preview.ID, conflict.ID, "keep-existing")

	keep := "keep-existing"
	conflict.Decision = &keep
	assert.Equal(t, conflict, got)
	srv.confirm(t, preview.ID, nil)
	assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))
}

// decisionFixture is a server with one Import in needs-review, one other Import in needs-review
// and one saved Import. Each Import has one conflict on the Deposit 101.
type decisionFixture struct {
	srv                   *testServer
	preview, other, saved previewResponse
}

func newDecisionFixture(t *testing.T) decisionFixture {
	t.Helper()

	srv := startServer(t)
	srv.importAccount(t, usdExport(), "XTB USD")
	changed := withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" })
	f := decisionFixture{srv: srv, preview: srv.preview(t, changed), other: srv.preview(t, changed), saved: srv.preview(t, changed)}
	for _, p := range []previewResponse{f.preview, f.other, f.saved} {
		require.Len(t, p.Conflicts, 1)
	}
	srv.decide(t, f.saved.ID, f.saved.Conflicts[0].ID, "keep-existing")
	srv.confirm(t, f.saved.ID, nil)
	return f
}

func TestImportDecision_InvalidRequest_ReturnsError(t *testing.T) {
	t.Parallel()

	path := func(importID, conflictID int64) string {
		return fmt.Sprintf("/api/imports/%d/conflicts/%d", importID, conflictID)
	}
	tests := []struct {
		name       string
		path       func(f decisionFixture) string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "unknown decision",
			path:       func(f decisionFixture) string { return path(f.preview.ID, f.preview.Conflicts[0].ID) },
			body:       `{"decision":"drop"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  `"drop"`,
		},
		{
			name:       "accept for changed data",
			path:       func(f decisionFixture) string { return path(f.preview.ID, f.preview.Conflicts[0].ID) },
			body:       `{"decision":"accept"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  `"accept" is not valid for an import conflict of the kind "changed-data"`,
		},
		{
			name:       "no decision",
			path:       func(f decisionFixture) string { return path(f.preview.ID, f.preview.Conflicts[0].ID) },
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "decision",
		},
		{
			name:       "bad JSON",
			path:       func(f decisionFixture) string { return path(f.preview.ID, f.preview.Conflicts[0].ID) },
			body:       `{"decision":`,
			wantStatus: http.StatusBadRequest,
			wantError:  "JSON",
		},
		{
			name:       "unknown import",
			path:       func(f decisionFixture) string { return path(9999, f.preview.Conflicts[0].ID) },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusNotFound,
			wantError:  "import 9999: import not found",
		},
		{
			name:       "import ID not a number",
			path:       func(decisionFixture) string { return "/api/imports/abc/conflicts/1" },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusNotFound,
			wantError:  "not found",
		},
		{
			name:       "unknown conflict",
			path:       func(f decisionFixture) string { return path(f.preview.ID, 9999) },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusNotFound,
			wantError:  "import conflict 9999",
		},
		{
			name:       "conflict ID not a number",
			path:       func(f decisionFixture) string { return fmt.Sprintf("/api/imports/%d/conflicts/abc", f.preview.ID) },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusNotFound,
			wantError:  "not found",
		},
		{
			name:       "conflict of another import",
			path:       func(f decisionFixture) string { return path(f.preview.ID, f.other.Conflicts[0].ID) },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusNotFound,
			wantError:  "import conflict",
		},
		{
			name:       "saved import",
			path:       func(f decisionFixture) string { return path(f.saved.ID, f.saved.Conflicts[0].ID) },
			body:       `{"decision":"use-new"}`,
			wantStatus: http.StatusConflict,
			wantError:  "saved",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newDecisionFixture(t)

			status, _, body := f.srv.do(t, http.MethodPost, tc.path(f), strings.NewReader(tc.body))

			assert.Equal(t, tc.wantStatus, status)
			var got errorBody
			decode(t, body, &got)
			assert.Contains(t, got.Error, tc.wantError)
			for _, p := range []previewResponse{f.preview, f.other} {
				status, body := f.srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", p.ID), nil)
				assert.Equal(t, http.StatusConflict, status)
				assert.Contains(t, body, "1 import conflict has no decision", "the request changed no decision")
			}
		})
	}
}

func TestImportCancel_PreviewWithConflict_SavesNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	imports := srv.imports(t)
	preview := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	require.Len(t, preview.Conflicts, 1)
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/cancel", preview.ID), nil)

	require.Equal(t, http.StatusNoContent, status, "body %q", body)
	assert.Empty(t, body)
	assert.Equal(t, imports, srv.imports(t))
	assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))

	for _, path := range []string{
		fmt.Sprintf("/api/imports/%d/confirm", preview.ID),
		fmt.Sprintf("/api/imports/%d/cancel", preview.ID),
		fmt.Sprintf("/api/imports/%d/conflicts/%d", preview.ID, preview.Conflicts[0].ID),
	} {
		status, body := srv.postJSON(t, path, map[string]string{"decision": "keep-existing"})

		assert.Equal(t, http.StatusConflict, status, "path %s", path)
		var got errorBody
		decode(t, body, &got)
		assert.Contains(t, got.Error, "cancelled", "path %s", path)
	}
	assert.Equal(t, usdSummary(accountID), srv.summary(t, accountID))
}

func TestImportCancel_NewAccount_MakesNoAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, usdExport())

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/cancel", preview.ID), nil)

	require.Equal(t, http.StatusNoContent, status, "body %q", body)
	again := srv.preview(t, usdExport())
	assert.Nil(t, again.Account, "the account number is still new")
	assert.Empty(t, srv.imports(t))
}

func TestImportCancel_UnknownOrSavedImport_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		wantStatus int
		wantError  string
	}{
		{name: "unknown", id: "9999", wantStatus: http.StatusNotFound, wantError: "import 9999: import not found"},
		{name: "not a number", id: "abc", wantStatus: http.StatusNotFound, wantError: "not found"},
		// An empty id is the ID of the saved Import of the subtest.
		{name: "saved", id: "", wantStatus: http.StatusConflict, wantError: "saved"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			saved := srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))
			id := tc.id
			if id == "" {
				id = strconv.FormatInt(saved.ID, 10)
			}

			status, body := srv.postJSON(t, "/api/imports/"+id+"/cancel", nil)

			assert.Equal(t, tc.wantStatus, status)
			var got errorBody
			decode(t, body, &got)
			assert.Contains(t, got.Error, tc.wantError)
			assert.Equal(t, []importResponse{saved}, srv.imports(t))
		})
	}
}

func TestImportConfirm_LedgerChangedAfterDecision_ReturnsConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	first := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1200.00" }))
	second := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1300.00" }))
	require.Len(t, first.Conflicts, 1)
	require.Len(t, second.Conflicts, 1)
	srv.decide(t, first.ID, first.Conflicts[0].ID, "use-new")
	srv.decide(t, second.ID, second.Conflicts[0].ID, "use-new")
	srv.confirm(t, second.ID, nil)

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", first.ID), nil)

	assert.Equal(t, http.StatusConflict, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "upload the file again")
	assert.Equal(t, "1300.00", srv.summary(t, accountID).NetDeposits)
}

func TestImportConfirm_ConflictAppearsAfterPreview_ReturnsConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")
	// The first preview has no conflict. A different Import then changes the stored Deposit.
	first := srv.preview(t, usdExport())
	require.Empty(t, first.Conflicts)
	second := srv.preview(t, withChange(t, "101", func(op *xtbtest.CashOperation) { op.Amount = "1300.00" }))
	require.Len(t, second.Conflicts, 1)
	srv.decide(t, second.ID, second.Conflicts[0].ID, "use-new")
	srv.confirm(t, second.ID, nil)

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", first.ID), nil)

	assert.Equal(t, http.StatusConflict, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "upload the file again")
	assert.Equal(t, "1300.00", srv.summary(t, accountID).NetDeposits)
}
