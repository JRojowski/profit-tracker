package httpapi_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// importCashOperations returns the Cash Operations that the Import added.
func (s *testServer) importCashOperations(t *testing.T, id int64) []importCashOperationResponse {
	t.Helper()

	status, _, body := s.get(t, fmt.Sprintf("/api/imports/%d/cash-operations", id))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got []importCashOperationResponse
	decode(t, body, &got)
	return got
}

func TestImportList_CarriesThePeriodOfItsCashOperations(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	first := srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))
	// The same file again: every Cash Operation is skipped, so the Import adds none.
	second := srv.confirm(t, srv.preview(t, usdExport()).ID, nil)

	require.NotNil(t, first.PeriodFrom)
	require.NotNil(t, first.PeriodTo)
	assert.Equal(t, "2026-02-01T10:00:00Z", *first.PeriodFrom)
	assert.Equal(t, "2026-02-03T10:00:00Z", *first.PeriodTo)
	assert.Nil(t, second.PeriodFrom)
	assert.Nil(t, second.PeriodTo)
	assert.Equal(t, []importResponse{second, first}, srv.imports(t))
}

func TestImportList_PeriodKeepsTheExactUTCTimeNearMidnight(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// 2026-01-01 00:30 in Poland is 2025-12-31 23:30 UTC. The Period must not be cut to a date.
	export := xtbtest.Export{
		AccountNumber: "56000002",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Deposit", Time: time.Date(2025, 12, 31, 23, 30, 0, 0, time.UTC), Amount: "10.00", ID: "201", Comment: "Blik deposit"},
		},
	}

	got := srv.confirm(t, srv.preview(t, export).ID, newAccountBody("XTB USD", "regular", "USD"))

	require.NotNil(t, got.PeriodFrom)
	require.NotNil(t, got.PeriodTo)
	assert.Equal(t, "2025-12-31T23:30:00Z", *got.PeriodFrom)
	assert.Equal(t, "2025-12-31T23:30:00Z", *got.PeriodTo)
}

func TestImportList_ImportThatAddedNothing_HasNullPeriodInJSON(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))
	srv.confirm(t, srv.preview(t, usdExport()).ID, nil)

	_, _, body := srv.get(t, "/api/imports")

	assert.Contains(t, body, `"periodFrom":null`)
	assert.Contains(t, body, `"periodTo":null`)
}

func TestImportCashOperations_ReturnsTheAddedCashOperationsInTimeOrder(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.confirm(t, srv.preview(t, usdExport()).ID, newAccountBody("XTB USD", "regular", "USD"))
	second := srv.confirm(t, srv.preview(t, usdExport()).ID, nil)

	got := srv.importCashOperations(t, first.ID)

	require.Len(t, got, 3)
	assert.Equal(t, "101", got[0].XTBID)
	assert.Equal(t, "Deposit", got[0].Type)
	assert.Equal(t, "2026-02-01T10:00:00Z", got[0].Time)
	assert.Equal(t, "1000.00", got[0].Amount)
	assert.Equal(t, "Blik deposit", got[0].Comment)
	assert.Nil(t, got[0].Volume)
	assert.Equal(t, "102", got[1].XTBID)
	assert.Equal(t, "FPS.US", got[1].Ticker)
	assert.Equal(t, "2000000001", got[1].PositionID)
	assert.Equal(t, "103", got[2].XTBID)
	assert.Equal(t, "2026-02-03T10:00:00Z", got[2].Time)
	assert.NotNil(t, got[2].Volume)
	assert.Equal(t, []importCashOperationResponse{}, srv.importCashOperations(t, second.ID), "an Import that added nothing has an empty list")
}

func TestImportCashOperations_UnknownImport_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, usdExport())

	for _, id := range []string{"42", "abc", strconv.FormatInt(preview.ID, 10)} {
		status, _, body := srv.get(t, "/api/imports/"+id+"/cash-operations")

		assert.Equal(t, http.StatusNotFound, status, "id %s", id)
		assert.Contains(t, body, `"error"`)
	}
}
