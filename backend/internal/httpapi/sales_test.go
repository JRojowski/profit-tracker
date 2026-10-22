package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// saleResponse is one row of GET /api/accounts/{id}/sales.
type saleResponse struct {
	XTBID          string `json:"xtbId"`
	Time           string `json:"time"`
	Ticker         string `json:"ticker"`
	Name           string `json:"name"`
	PositionID     string `json:"positionId"`
	Volume         string `json:"volume"`
	Amount         string `json:"amount"`
	Cost           string `json:"cost"`
	RealizedProfit string `json:"realizedProfit"`
}

// saleTime returns the time of a sale on day n of March 2026, at 15:30 UTC.
func saleTime(day int) time.Time {
	return time.Date(2026, 3, day, 15, 30, 0, 0, time.UTC)
}

// adrExample1 is ADR 0001 Example 1: one Lot, closed in two parts on the same day.
// closedProfits are the Profit/Loss values in Closed Positions for the two sales.
//
//	Sale 1: cost = 109.40 × 9 / 10 = 98.46, profit = 92.34 − 98.46 = −6.12
//	Sale 2: cost = 109.40 × 1 / 10 = 10.94, profit = 10.28 − 10.94 = −0.66
//	The Lot is closed.
func adrExample1(closedProfit1, closedProfit2 string) xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "50000001",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: saleTime(2).Add(time.Minute), Amount: "10.28", ID: "304", Comment: "CLOSE BUY 1/10 @ 10.28", PositionID: "2636532207"},
			{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: saleTime(2), Amount: "92.34", ID: "303", Comment: "CLOSE BUY 9/10 @ 10.26", PositionID: "2636532207"},
			{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(2), Amount: "-109.40", ID: "302", Comment: "OPEN BUY 10 @ 10.94", PositionID: "2636532207"},
			{Type: "Deposit", Time: opTime(1), Amount: "200.00", ID: "301", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "ABC.PL", Volume: "1", OpenTime: opTime(2), CloseTime: saleTime(2).Add(time.Minute), ProfitLoss: closedProfit2, PositionID: "2636532207"},
			{Ticker: "ABC.PL", Volume: "9", OpenTime: opTime(2), CloseTime: saleTime(2), ProfitLoss: closedProfit1, PositionID: "2636532207"},
		},
	}
}

// adrExample2 is ADR 0001 Example 2: one Lot, closed in part, the rest stays open.
//
//	Cost   = 185.31 × 0.1504 / 0.2638 ≈ 105.65
//	Profit = 162.73 − 105.65          ≈ 57.08
//	Open:  volume 0.1134, cost ≈ 79.66
func adrExample2() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "50000002",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "XYZ.US", Instrument: "XYZ", Time: saleTime(5), Amount: "162.73", ID: "403", Comment: "CLOSE BUY 0.1504/0.426 @ 256.60", PositionID: "1349972184"},
			{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Time: opTime(2), Amount: "-185.31", ID: "402", Comment: "OPEN BUY 0.2638 @ 161.92", PositionID: "1349972184"},
			{Type: "Deposit", Time: opTime(1), Amount: "200.00", ID: "401", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "XYZ.US", Volume: "0.1504", OpenTime: opTime(2), CloseTime: saleTime(5), ProfitLoss: "57.08", PositionID: "1349972184"},
		},
	}
}

// sales returns the sales of the Account.
func (s *testServer) sales(t *testing.T, accountID int64) []saleResponse {
	t.Helper()

	var got []saleResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/sales", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// cents rounds the decimal text to 2 decimal places. ADR 0001: the tool rounds only for display.
func cents(t *testing.T, text string) string {
	t.Helper()

	d, err := decimal.NewFromString(text)
	require.NoError(t, err, "%q is not a decimal", text)
	return d.StringFixed(2)
}

func TestSales_ADR0001Example1_LotClosedInTwoParts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.66")
	preview := srv.preview(t, export)
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	got := srv.sales(t, accountID)

	assert.Equal(t, []saleResponse{
		{
			XTBID: "303", Time: "2026-03-02T15:30:00Z", Ticker: "ABC.PL", Name: "ABC", PositionID: "2636532207",
			Volume: "9", Amount: "92.34", Cost: "98.46", RealizedProfit: "-6.12",
		},
		{
			XTBID: "304", Time: "2026-03-02T15:31:00Z", Ticker: "ABC.PL", Name: "ABC", PositionID: "2636532207",
			Volume: "1", Amount: "10.28", Cost: "10.94", RealizedProfit: "-0.66",
		},
	}, got)
	assert.Empty(t, srv.holdings(t, accountID), "the Lot is closed")
	assert.Equal(t, "193.22", srv.summary(t, accountID).Cash)
}

func TestSales_ADR0001Example2_LotClosedInPart(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adrExample2())
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	sales := srv.sales(t, accountID)
	holdings := srv.holdings(t, accountID)

	require.Len(t, sales, 1)
	sale := sales[0]
	assert.Equal(t, "403", sale.XTBID)
	assert.Equal(t, "0.1504", sale.Volume)
	assert.Equal(t, "162.73", sale.Amount)
	assert.Equal(t, "105.65", cents(t, sale.Cost))
	assert.Equal(t, "57.08", cents(t, sale.RealizedProfit))
	require.Len(t, holdings, 1)
	assert.Equal(t, "XYZ.US", holdings[0].Ticker)
	assert.Equal(t, "0.1134", holdings[0].Volume)
	assert.Equal(t, "79.66", cents(t, holdings[0].Cost))
	// The cost of the closed part plus the open cost is the purchase amount.
	assert.Equal(t, "185.31", cents(t, decimal.RequireFromString(sale.Cost).Add(decimal.RequireFromString(holdings[0].Cost)).String()))
}

func TestSales_NoSales_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/sales", accountID))

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, "[]", body)
}

func TestImportPreview_SaleOfUnknownLot_IsRejected(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := usdExport()
	export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{
		Type: "Stock sell", Ticker: "FPS.US", Amount: "12.00", ID: "999", Comment: "CLOSE BUY 1 @ 12.00", PositionID: "2999999999",
	})

	status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))

	assert.Equal(t, http.StatusBadRequest, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "999")
	assert.Contains(t, got.Error, "2999999999")
	assert.Empty(t, srv.imports(t))
}

// checkData is the Realized Profit check of an Import Conflict of the kind "check-mismatch".
type checkData struct {
	Calculated      string `json:"calculated"`
	ClosedPositions string `json:"closedPositions"`
}

// checkConflict is an Import Conflict with the check field.
type checkConflict struct {
	ID       int64              `json:"id"`
	Kind     string             `json:"kind"`
	XTBID    string             `json:"xtbId"`
	Existing *cashOperationData `json:"existing"`
	New      *cashOperationData `json:"new"`
	Check    *checkData         `json:"check"`
	Decision *string            `json:"decision"`
}

// checkPreview is the body of POST /api/imports with the check field in the conflicts.
type checkPreview struct {
	ID                int64           `json:"id"`
	State             string          `json:"state"`
	NewCashOperations int             `json:"newCashOperations"`
	Conflicts         []checkConflict `json:"conflicts"`
}

func TestImportPreview_RealizedProfitCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		closedProfit1 string
		wantConflict  bool
	}{
		{name: "same value", closedProfit1: "-6.12", wantConflict: false},
		{name: "difference of 0.01", closedProfit1: "-6.13", wantConflict: false},
		{name: "difference above 0.01", closedProfit1: "-6.1301", wantConflict: true},
		{name: "wrong value", closedProfit1: "-6.20", wantConflict: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			export := adrExample1(tc.closedProfit1, "-0.66")

			status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))

			require.Equal(t, http.StatusOK, status, "body %q", body)
			var got checkPreview
			decode(t, body, &got)
			assert.Equal(t, 4, got.NewCashOperations)
			if !tc.wantConflict {
				assert.Equal(t, "preview", got.State)
				assert.Empty(t, got.Conflicts)
				return
			}
			assert.Equal(t, "needs-review", got.State)
			require.Len(t, got.Conflicts, 1)
			c := got.Conflicts[0]
			assert.Equal(t, "check-mismatch", c.Kind)
			assert.Equal(t, "303", c.XTBID)
			assert.Nil(t, c.Existing)
			assert.Equal(t, &cashOperationData{
				Type: "Stock sell", Time: "2026-03-02T15:30:00Z", Amount: "92.34", Comment: "CLOSE BUY 9/10 @ 10.26",
				Ticker: "ABC.PL", PositionID: "2636532207", Volume: ptr("9"), Price: ptr("10.26"),
			}, c.New)
			assert.Equal(t, &checkData{Calculated: "-6.12", ClosedPositions: cents(t, tc.closedProfit1)}, roundedCheck(t, c.Check))
			assert.Nil(t, c.Decision)
		})
	}
}

// roundedCheck returns the check with the Closed Positions value rounded to cents, for a short table.
func roundedCheck(t *testing.T, c *checkData) *checkData {
	t.Helper()
	if c == nil {
		return nil
	}
	return &checkData{Calculated: c.Calculated, ClosedPositions: cents(t, c.ClosedPositions)}
}

func TestImport_CheckMismatchAccepted_SavesTheLedgerValue(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.20", "-0.66")
	status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var preview checkPreview
	decode(t, body, &preview)
	require.Len(t, preview.Conflicts, 1)
	conflict := preview.Conflicts[0]
	assert.Equal(t, &checkData{Calculated: "-6.12", ClosedPositions: "-6.20"}, conflict.Check)

	confirmPath := fmt.Sprintf("/api/imports/%d/confirm", preview.ID)
	status, body = srv.postJSON(t, confirmPath, newAccountBody("XTB PLN", "regular", "PLN"))
	require.Equal(t, http.StatusConflict, status, "the conflict has no decision: %s", body)
	assert.Contains(t, body, "1 import conflict has no decision")

	decidePath := fmt.Sprintf("/api/imports/%d/conflicts/%d", preview.ID, conflict.ID)
	for _, decision := range []string{"keep-existing", "use-new"} {
		status, body := srv.postJSON(t, decidePath, map[string]string{"decision": decision})
		assert.Equal(t, http.StatusBadRequest, status, "decision %s", decision)
		var got errorBody
		decode(t, body, &got)
		assert.Contains(t, got.Error, decision)
		assert.Contains(t, got.Error, "check-mismatch")
		assert.Contains(t, got.Error, `"accept"`)
	}

	status, body = srv.postJSON(t, decidePath, map[string]string{"decision": "accept"})
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var decided checkConflict
	decode(t, body, &decided)
	accept := "accept"
	conflict.Decision = &accept
	assert.Equal(t, conflict, decided)

	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	sales := srv.sales(t, accountID)
	require.Len(t, sales, 2)
	assert.Equal(t, "-6.12", sales[0].RealizedProfit, "the Ledger is the source of truth")
	assert.Equal(t, "-0.66", sales[1].RealizedProfit)
}

func TestImport_KnownSaleWithWrongClosedPositions_GivesNoConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")

	// The check runs only for the sales that are new in the Import.
	preview := srv.preview(t, adrExample1("-6.20", "-0.70"))

	assert.Equal(t, "preview", preview.State)
	assert.Empty(t, preview.Conflicts)
}

func TestImport_SaleInLaterImport_ClosesLotOfEarlierImport(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	full := adrExample1("-6.12", "-0.66")
	first := full
	first.CashOperations = full.CashOperations[2:]
	first.ClosedPositions = nil
	accountID := srv.importAccount(t, first, "XTB PLN")
	require.Len(t, srv.holdings(t, accountID), 1)

	wrong := adrExample1("-6.12", "-0.75")
	preview := srv.preview(t, wrong)

	require.Len(t, preview.Conflicts, 1, "the check uses the stored Lot")
	assert.Equal(t, "304", preview.Conflicts[0].XTBID)
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "accept")
	srv.confirm(t, preview.ID, nil)
	assert.Empty(t, srv.holdings(t, accountID))
	assert.Len(t, srv.sales(t, accountID), 2)
}

func TestImportPreview_CheckMismatchConflict_HasContractJSON(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.70")

	status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got struct {
		State     string            `json:"state"`
		Conflicts []json.RawMessage `json:"conflicts"`
	}
	decode(t, body, &got)

	assert.Equal(t, "needs-review", got.State)
	require.Len(t, got.Conflicts, 1)
	var conflict checkConflict
	decode(t, string(got.Conflicts[0]), &conflict)
	assert.JSONEq(t, fmt.Sprintf(`{
		"id": %d,
		"kind": "check-mismatch",
		"xtbId": "304",
		"existing": null,
		"new": {"type": "Stock sell", "time": "2026-03-02T15:31:00Z", "amount": "10.28", "comment": "CLOSE BUY 1/10 @ 10.28",
			"ticker": "ABC.PL", "positionId": "2636532207", "volume": "1", "price": "10.28"},
		"check": {"calculated": "-0.66", "closedPositions": "-0.70"},
		"decision": null
	}`, conflict.ID), string(got.Conflicts[0]))
}

func TestSales_SaleTimeOrder_ThenXTBID(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.66")
	// Both sales at the same time: the XTB ID decides the order.
	export.CashOperations[0].Time = saleTime(2)
	export.ClosedPositions[0].CloseTime = saleTime(2)
	accountID := srv.importAccount(t, export, "XTB PLN")

	got := srv.sales(t, accountID)

	require.Len(t, got, 2)
	assert.Equal(t, "303", got[0].XTBID)
	assert.Equal(t, "304", got[1].XTBID)
}

func TestImportConfirm_UseNewThatOvercloses_IsRejectedAndSavesNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")
	summary, holdings, sales := srv.summary(t, accountID), srv.holdings(t, accountID), srv.sales(t, accountID)

	// The purchase 302 now has volume 5. The stored sales close 9 + 1 of the Lot.
	smaller := adrExample1("-6.12", "-0.66")
	smaller.CashOperations[2].Comment = "OPEN BUY 5 @ 10.94"
	preview := srv.preview(t, smaller)
	require.Len(t, preview.Conflicts, 1)
	require.Equal(t, "302", preview.Conflicts[0].XTBID)
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "use-new")

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), nil)

	assert.Equal(t, http.StatusBadRequest, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "sale 303")
	assert.Contains(t, got.Error, "lot 2636532207")
	assert.Equal(t, summary, srv.summary(t, accountID))
	assert.Equal(t, holdings, srv.holdings(t, accountID))
	assert.Equal(t, sales, srv.sales(t, accountID))
	assert.Len(t, srv.imports(t), 1, "the import is not saved")

	// The Import stays open: a different decision lets the confirm succeed.
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "keep-existing")
	srv.confirm(t, preview.ID, nil)
	assert.Equal(t, sales, srv.sales(t, accountID))
}

func TestImportPreview_ZeroVolumeSale_IsRejected(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.66")
	export.CashOperations[0].Comment = "CLOSE BUY 0 @ 10.28"

	status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))

	assert.Equal(t, http.StatusBadRequest, status)
	var got errorBody
	decode(t, body, &got)
	assert.Contains(t, got.Error, "row 6")
	assert.Contains(t, got.Error, `"CLOSE BUY 0 @ 10.28"`)
	assert.Empty(t, srv.imports(t))
}
