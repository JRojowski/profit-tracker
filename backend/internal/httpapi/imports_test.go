package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

// accountResponse is an Account in an API response.
type accountResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
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
func usdExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000001",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "PENG.US", Amount: "-116.58", ID: "103", Comment: "OPEN BUY PENG.US 2 @ 58.29", PositionID: "2000000002"},
			{Type: "Stock purchase", Ticker: "FPS.US", Amount: "-109.40", ID: "102", Comment: "OPEN BUY 10 @ 10.94", PositionID: "2000000001"},
			{Type: "Deposit", Amount: "1000.00", ID: "101", Comment: "Blik deposit"},
		},
	}
}

// preview uploads the export and returns the preview. It stops the test if the status is not 200.
func (s *testServer) preview(t *testing.T, export xtbtest.Export) previewResponse {
	t.Helper()

	status, body := s.upload(t, "/api/imports", export.FileName(), xtbtest.Build(t, export))
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
	assert.Equal(t, accountResponse{ID: got.AccountID, Name: "XTB USD", Type: "regular", Currency: "USD"}, *again.Account)
}

func TestImportConfirm_KnownAccount_GoesToSameAccountWithNoBody(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))

	// A newer export of the same Account: the old rows and one new purchase.
	newer := usdExport()
	newer.To = time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	newer.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "LSMC.DE", Amount: "-126.35", ID: "104", Comment: "OPEN BUY 0.6169/1.6169 @ 47.755", PositionID: "2000000003"},
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
			data:      withRow(xtbtest.CashOperation{Type: "Stock sell", Ticker: "FPS.US", Amount: "120.00", ID: "999", Comment: "CLOSE BUY 10 @ 12.00", PositionID: "2000000001"}),
			wantError: []string{"unknown cash operation type", `"Stock sell"`, "row 9"},
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

			status, body := srv.upload(t, "/api/imports", "USD_56000001_2006-01-01_2026-10-22.xlsx", tc.data)

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

	comments := []string{
		"OPEN BUY 2 @ 58.29",
		"OPEN BUY PENG.US 2 @ 58.29",
		"OPEN BUY 0.6169/1.6169 @ 47.755",
		"OPEN BUY FPS.US 1/1.27 @ 39.24",
	}
	for _, comment := range comments {
		t.Run(comment, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			export := xtbtest.Export{CashOperations: []xtbtest.CashOperation{
				{Type: "Stock purchase", Ticker: "PENG.US", Amount: "-116.58", Comment: comment, PositionID: "2000000002"},
			}}

			got := srv.preview(t, export)

			assert.Equal(t, 1, got.NewCashOperations)
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
