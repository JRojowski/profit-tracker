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

// closedPositionResponse is one row of GET /api/accounts/{id}/Closed Positions.
type closedPositionResponse struct {
	XTBID          string  `json:"xtbId"`
	Time           string  `json:"time"`
	Ticker         string  `json:"ticker"`
	Name           string  `json:"name"`
	PositionID     string  `json:"positionId"`
	Volume         string  `json:"volume"`
	Amount         string  `json:"amount"`
	Cost           string  `json:"cost"`
	RealizedProfit string  `json:"realizedProfit"`
	OpenTime       string  `json:"openTime"`
	OpenPrice      *string `json:"openPrice"`
	ClosePrice     *string `json:"closePrice"`
}

// closeTime returns the time of a Closed Position on day n of March 2026, at 15:30 UTC.
func closeTime(day int) time.Time {
	return time.Date(2026, 3, day, 15, 30, 0, 0, time.UTC)
}

// adrExample1 is ADR 0001 Example 1: one Lot, closed in two parts on the same day.
// closedProfits are the Profit/Loss values in Closed Positions for the two Closed Positions.
//
//	Closed Position 1: cost = 109.40 × 9 / 10 = 98.46, profit = 92.34 − 98.46 = −6.12
//	Closed Position 2: cost = 109.40 × 1 / 10 = 10.94, profit = 10.28 − 10.94 = −0.66
//	The Lot is closed.
func adrExample1(closedProfit1, closedProfit2 string) xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "50000001",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(2).Add(time.Minute), Amount: "10.28", ID: "304", Comment: "CLOSE BUY 1/10 @ 10.28", PositionID: "2636532207"},
			{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(2), Amount: "92.34", ID: "303", Comment: "CLOSE BUY 9/10 @ 10.26", PositionID: "2636532207"},
			{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(2), Amount: "-109.40", ID: "302", Comment: "OPEN BUY 10 @ 10.94", PositionID: "2636532207"},
			{Type: "Deposit", Time: opTime(1), Amount: "200.00", ID: "301", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "ABC.PL", Volume: "1", OpenTime: opTime(2), CloseTime: closeTime(2).Add(time.Minute), ProfitLoss: closedProfit2, PositionID: "2636532207"},
			{Ticker: "ABC.PL", Volume: "9", OpenTime: opTime(2), CloseTime: closeTime(2), ProfitLoss: closedProfit1, PositionID: "2636532207"},
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
			{Type: "Stock sell", Ticker: "XYZ.US", Instrument: "XYZ", Time: closeTime(5), Amount: "162.73", ID: "403", Comment: "CLOSE BUY 0.1504/0.426 @ 256.60", PositionID: "1349972184"},
			{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Time: opTime(2), Amount: "-185.31", ID: "402", Comment: "OPEN BUY 0.2638 @ 161.92", PositionID: "1349972184"},
			{Type: "Deposit", Time: opTime(1), Amount: "200.00", ID: "401", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "XYZ.US", Volume: "0.1504", OpenTime: opTime(2), CloseTime: closeTime(5), ProfitLoss: "57.08", PositionID: "1349972184"},
		},
	}
}

// Closed Positions returns the Closed Positions of the Account.
func (s *testServer) closedPositions(t *testing.T, accountID int64) []closedPositionResponse {
	t.Helper()

	var got []closedPositionResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/closed-positions", accountID), &got)
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

// centsPointer rounds the decimal text of a pointer to 2 decimal places. A nil pointer stays nil.
func centsPointer(t *testing.T, text *string) *string {
	t.Helper()

	if text == nil {
		return nil
	}
	return new(cents(t, *text))
}

func TestClosedPositions_ADR0001Example1_LotClosedInTwoParts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.66")
	preview := srv.preview(t, export)
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	got := srv.closedPositions(t, accountID)

	assert.Equal(t, []closedPositionResponse{
		{
			XTBID: "303", Time: "2026-03-02T15:30:00Z", Ticker: "ABC.PL", Name: "ABC", PositionID: "2636532207",
			Volume: "9", Amount: "92.34", Cost: "98.46", RealizedProfit: "-6.12",
			OpenTime: "2026-02-02T10:00:00Z", OpenPrice: ptr("10.94"), ClosePrice: ptr("10.26"),
		},
		{
			XTBID: "304", Time: "2026-03-02T15:31:00Z", Ticker: "ABC.PL", Name: "ABC", PositionID: "2636532207",
			Volume: "1", Amount: "10.28", Cost: "10.94", RealizedProfit: "-0.66",
			OpenTime: "2026-02-02T10:00:00Z", OpenPrice: ptr("10.94"), ClosePrice: ptr("10.28"),
		},
	}, got)
	assert.Empty(t, srv.holdings(t, accountID), "the Lot is closed")
	assert.Equal(t, "193.22", srv.summary(t, accountID).Cash)
}

func TestClosedPositions_ADR0001Example2_LotClosedInPart(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adrExample2())
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	closedPositions := srv.closedPositions(t, accountID)
	holdings := srv.holdings(t, accountID)

	require.Len(t, closedPositions, 1)
	closedPosition := closedPositions[0]
	assert.Equal(t, "403", closedPosition.XTBID)
	assert.Equal(t, "0.1504", closedPosition.Volume)
	assert.Equal(t, "162.73", closedPosition.Amount)
	assert.Equal(t, "105.65", cents(t, closedPosition.Cost))
	assert.Equal(t, "57.08", cents(t, closedPosition.RealizedProfit))
	require.Len(t, holdings, 1)
	assert.Equal(t, "XYZ.US", holdings[0].Ticker)
	assert.Equal(t, "0.1134", holdings[0].Volume)
	assert.Equal(t, "79.66", cents(t, holdings[0].Cost))
	// The cost of the closed part plus the open cost is the purchase amount.
	assert.Equal(t, "185.31", cents(t, decimal.RequireFromString(closedPosition.Cost).Add(decimal.RequireFromString(holdings[0].Cost)).String()))
}

func TestClosedPositions_NoClosedPositions_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, usdExport(), "XTB USD")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/closed-positions", accountID))

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, "[]", body)
}

func TestImportPreview_ClosedPositionOfUnknownLot_IsRejected(t *testing.T) {
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

// TestImportPreview_RealizedProfitCheck_SameTimeAndVolume_ClosePriceDecides covers two Closed Positions of one Lot
// with the same time and the same volume. Only the Close Price tells their Closed Positions rows apart.
//
//	Lot: OPEN BUY 4 @ 23.00, amount -92.00. The cost of 2 is 46.00.
//	Closed Position 402: CLOSE BUY 2/8 @ 18.00, amount 36.00, profit 36.00 − 46.00 = −10.00
//	Closed Position 403: CLOSE BUY 2/8 @ 17.95, amount 35.90, profit 35.90 − 46.00 = −10.10
func TestImportPreview_RealizedProfitCheck_SameTimeAndVolume_ClosePriceDecides(t *testing.T) {
	t.Parallel()

	row402 := xtbtest.ClosedPosition{
		Ticker: "DEF.PL", Volume: "2", OpenTime: opTime(3), ClosePrice: "18.00", CloseTime: closeTime(4),
		ProfitLoss: "-10.00", PositionID: "2700000001",
	}
	// The Excel cell of the Close Price can carry float noise.
	row403 := xtbtest.ClosedPosition{
		Ticker: "DEF.PL", Volume: "2", OpenTime: opTime(3), ClosePrice: "17.950000000000003", CloseTime: closeTime(4),
		ProfitLoss: "-10.10", PositionID: "2700000001",
	}
	tests := []struct {
		name string
		rows []xtbtest.ClosedPosition
	}{
		{name: "row of the second Closed Position first", rows: []xtbtest.ClosedPosition{row403, row402}},
		{name: "row of the first Closed Position first", rows: []xtbtest.ClosedPosition{row402, row403}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			export := xtbtest.Export{
				AccountNumber: "50000002",
				Currency:      "PLN",
				CashOperations: []xtbtest.CashOperation{
					{Type: "Stock sell", Ticker: "DEF.PL", Instrument: "DEF", Time: closeTime(4), Amount: "35.90", ID: "403", Comment: "CLOSE BUY 2/8 @ 17.95", PositionID: "2700000001"},
					{Type: "Stock sell", Ticker: "DEF.PL", Instrument: "DEF", Time: closeTime(4), Amount: "36.00", ID: "402", Comment: "CLOSE BUY 2/8 @ 18.00", PositionID: "2700000001"},
					{Type: "Stock purchase", Ticker: "DEF.PL", Instrument: "DEF", Time: opTime(3), Amount: "-92.00", ID: "401", Comment: "OPEN BUY 4 @ 23.00", PositionID: "2700000001"},
					{Type: "Deposit", Time: opTime(1), Amount: "100.00", ID: "400", Comment: "Blik deposit"},
				},
				ClosedPositions: tc.rows,
			}

			status, body := srv.upload(t, export.FileName(), xtbtest.Build(t, export))

			require.Equal(t, http.StatusOK, status, "body %q", body)
			var got checkPreview
			decode(t, body, &got)
			assert.Equal(t, 4, got.NewCashOperations)
			assert.Equal(t, "preview", got.State)
			assert.Empty(t, got.Conflicts)
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

	closedPositions := srv.closedPositions(t, accountID)
	require.Len(t, closedPositions, 2)
	assert.Equal(t, "-6.12", closedPositions[0].RealizedProfit, "the Ledger is the source of truth")
	assert.Equal(t, "-0.66", closedPositions[1].RealizedProfit)
}

func TestImport_KnownClosedPositionWithWrongClosedPositions_GivesNoConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")

	// The check runs only for the Closed Positions that are new in the Import.
	preview := srv.preview(t, adrExample1("-6.20", "-0.70"))

	assert.Equal(t, "preview", preview.State)
	assert.Empty(t, preview.Conflicts)
}

func TestImport_ClosedPositionInLaterImport_ClosesLotOfEarlierImport(t *testing.T) {
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
	assert.Len(t, srv.closedPositions(t, accountID), 2)
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

func TestClosedPositions_CloseTimeOrder_ThenXTBID(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adrExample1("-6.12", "-0.66")
	// Both Closed Positions at the same time: the XTB ID decides the order.
	export.CashOperations[0].Time = closeTime(2)
	export.ClosedPositions[0].CloseTime = closeTime(2)
	accountID := srv.importAccount(t, export, "XTB PLN")

	got := srv.closedPositions(t, accountID)

	require.Len(t, got, 2)
	assert.Equal(t, "303", got[0].XTBID)
	assert.Equal(t, "304", got[1].XTBID)
}

func TestImportConfirm_UseNewThatOvercloses_IsRejectedAndSavesNothing(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")
	summary, holdings, closedPositions := srv.summary(t, accountID), srv.holdings(t, accountID), srv.closedPositions(t, accountID)

	// The purchase 302 now has volume 5. The stored Closed Positions close 9 + 1 of the Lot.
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
	assert.Contains(t, got.Error, "closed position 303")
	assert.Contains(t, got.Error, "lot 2636532207")
	assert.Equal(t, summary, srv.summary(t, accountID))
	assert.Equal(t, holdings, srv.holdings(t, accountID))
	assert.Equal(t, closedPositions, srv.closedPositions(t, accountID))
	assert.Len(t, srv.imports(t), 1, "the import is not saved")

	// The Import stays open: a different decision lets the confirm succeed.
	srv.decide(t, preview.ID, preview.Conflicts[0].ID, "keep-existing")
	srv.confirm(t, preview.ID, nil)
	assert.Equal(t, closedPositions, srv.closedPositions(t, accountID))
}

func TestImportPreview_ZeroVolumeClosedPosition_IsRejected(t *testing.T) {
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

func TestClosedPositions_OldSalesPath_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adrExample2())
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	status, _, _ := srv.get(t, fmt.Sprintf("/api/accounts/%d/sales", accountID))

	assert.Equal(t, http.StatusNotFound, status)
}

// TestClosedPositions_OpenTimeOpenPriceAndClosePrice covers the Fill and sale prices of the Closed Positions.
func TestClosedPositions_OpenTimeOpenPriceAndClosePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ops  []xtbtest.CashOperation
		want []closedPositionResponse
	}{
		{
			name: "Lot with one Fill",
			ops: []xtbtest.CashOperation{
				{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(3), Amount: "-50.00", ID: "501", Comment: "OPEN BUY 5 @ 10.00", PositionID: "2800000001"},
				{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(4), Amount: "60.00", ID: "502", Comment: "CLOSE BUY 5 @ 12.00", PositionID: "2800000001"},
			},
			want: []closedPositionResponse{
				{OpenTime: "2026-02-03T10:00:00Z", OpenPrice: ptr("10"), ClosePrice: ptr("12")},
			},
		},
		{
			name: "Lot with two Fills at different prices and volumes, weighted average",
			ops: []xtbtest.CashOperation{
				// The later Fill comes first in the list. The open time is the earliest Fill time.
				{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(5), Amount: "-90.00", ID: "512", Comment: "OPEN BUY 3 @ 30.00", PositionID: "2800000002"},
				{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(3), Amount: "-10.00", ID: "511", Comment: "OPEN BUY 1 @ 10.00", PositionID: "2800000002"},
				{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(6), Amount: "120.00", ID: "513", Comment: "CLOSE BUY 4 @ 30.00", PositionID: "2800000002"},
			},
			// Plain average (10 + 30) / 2 = 20. Weighted (10 × 1 + 30 × 3) / 4 = 25.
			want: []closedPositionResponse{
				{OpenTime: "2026-02-03T10:00:00Z", OpenPrice: ptr("25"), ClosePrice: ptr("30")},
			},
		},
		{
			name: "Lot closed in two parts",
			ops: []xtbtest.CashOperation{
				{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(3), Amount: "-10.00", ID: "521", Comment: "OPEN BUY 1 @ 10.00", PositionID: "2800000003"},
				{Type: "Stock purchase", Ticker: "ABC.PL", Instrument: "ABC", Time: opTime(5), Amount: "-90.00", ID: "522", Comment: "OPEN BUY 3 @ 30.00", PositionID: "2800000003"},
				{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(6), Amount: "62.00", ID: "523", Comment: "CLOSE BUY 2/4 @ 31.00", PositionID: "2800000003"},
				{Type: "Stock sell", Ticker: "ABC.PL", Instrument: "ABC", Time: closeTime(7), Amount: "66.00", ID: "524", Comment: "CLOSE BUY 2/4 @ 33.00", PositionID: "2800000003"},
			},
			want: []closedPositionResponse{
				{OpenTime: "2026-02-03T10:00:00Z", OpenPrice: ptr("25"), ClosePrice: ptr("31")},
				{OpenTime: "2026-02-03T10:00:00Z", OpenPrice: ptr("25"), ClosePrice: ptr("33")},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			export := xtbtest.Export{
				AccountNumber:  "50000003",
				Currency:       "PLN",
				CashOperations: append(tc.ops, xtbtest.CashOperation{Type: "Deposit", Time: opTime(1), Amount: "500.00", ID: "500", Comment: "Blik deposit"}),
			}
			accountID := srv.importAccount(t, export, "XTB PLN")

			got := srv.closedPositions(t, accountID)

			require.Len(t, got, len(tc.want))
			for i, want := range tc.want {
				assert.Equal(t, want.OpenTime, got[i].OpenTime, "row %d open time", i)
				assert.Equal(t, want.OpenPrice, got[i].OpenPrice, "row %d open price", i)
				assert.Equal(t, want.ClosePrice, got[i].ClosePrice, "row %d close price", i)
			}
		})
	}
}

func TestClosedPositions_HasContractJSONFields(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/closed-positions", accountID))

	require.Equal(t, http.StatusOK, status)
	var rows []map[string]json.RawMessage
	decode(t, body, &rows)
	require.NotEmpty(t, rows)
	assert.JSONEq(t, `"2026-02-02T10:00:00Z"`, string(rows[0]["openTime"]))
	assert.JSONEq(t, `"10.94"`, string(rows[0]["openPrice"]))
	assert.JSONEq(t, `"10.26"`, string(rows[0]["closePrice"]))
}
