package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// importAccount imports the export into a new Account with the name and returns the Account ID.
func (s *testServer) importAccount(t *testing.T, export xtbtest.Export, name string) int64 {
	t.Helper()

	currency := export.Currency
	if currency == "" {
		currency = xtbtest.DefaultCurrency
	}
	saved := s.confirm(t, s.preview(t, export).ID, newAccountBody(name, "regular", currency))
	return saved.AccountID
}

// patchJSON sends v as a JSON body with the method PATCH to path. It returns the status and the body.
func (s *testServer) patchJSON(t *testing.T, path string, v any) (status int, body string) {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)
	status, _, body = s.do(t, http.MethodPatch, path, bytes.NewReader(data))
	return status, body
}

// depositOnlyExport is an export with one Deposit and no purchases.
func depositOnlyExport(accountNumber, currency string) xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: accountNumber,
		Currency:      currency,
		CashOperations: []xtbtest.CashOperation{
			{Type: "Deposit", Amount: "250.00", ID: "901", Comment: "Blik deposit"},
		},
	}
}

// summaryResponse is the body of GET /api/accounts/{id}/summary.
type summaryResponse struct {
	AccountID      int64  `json:"accountId"`
	Currency       string `json:"currency"`
	Cash           string `json:"cash"`
	NetDeposits    string `json:"netDeposits"`
	RealizedProfit string `json:"realizedProfit"`
	NetDividends   string `json:"netDividends"`
	NetInterest    string `json:"netInterest"`
	Fees           string `json:"fees"`
	TotalResult    string `json:"totalResult"`
}

// holdingResponse is one row of GET /api/accounts/{id}/holdings.
type holdingResponse struct {
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	Volume      string `json:"volume"`
	Cost        string `json:"cost"`
	AverageCost string `json:"averageCost"`
}

// dashboardExport is an export with two Deposits and purchases of two Instruments.
// AAPL.US is in two Lots, so its average cost comes from both Lots.
// The API does not round the average cost. The division has 16 decimal places, the default of the decimal library.
//
//	Cash:         5000.00 + 1500.50 - 340.00 - 272.30 - 124.55 = 5763.65
//	Net Deposits: 5000.00 + 1500.50                            = 6500.50
//	AAPL.US:      volume 2 + 1.5 = 3.5, cost 340.00 + 272.30 = 612.30, average 174.9428571428571429
//	MSFT.US:      volume 0.3, cost 124.55, average 415.1666666666666667
func dashboardExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000001",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "MSFT.US", Instrument: "Microsoft", Amount: "-124.55", ID: "205", Comment: "OPEN BUY 0.3 @ 415.17", PositionID: "3000000003"},
			{Type: "Stock purchase", Ticker: "AAPL.US", Instrument: "Apple", Amount: "-272.30", ID: "204", Comment: "OPEN BUY 1.5 @ 181.53", PositionID: "3000000002"},
			{Type: "Deposit", Amount: "1500.50", ID: "203", Comment: "Blik deposit"},
			{Type: "Stock purchase", Ticker: "AAPL.US", Instrument: "Apple", Amount: "-340.00", ID: "202", Comment: "OPEN BUY AAPL.US 2 @ 170.00", PositionID: "3000000001"},
			{Type: "Deposit", Amount: "5000.00", ID: "201", Comment: "Blik deposit"},
		},
	}
}

func TestAccountSummary_DepositsAndPurchases_ReturnsCashAndNetDeposits(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, dashboardExport(), "XTB USD")
	// A second Account must not change the values of the first.
	srv.importAccount(t, depositOnlyExport("56000002", "PLN"), "XTB PLN")

	var got summaryResponse
	status := srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", id), &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, noResultSummary(id, "5763.65", "6500.50"), got)
}

// incomeAndCostsExport is an export with each Cash Operation type of the total result:
// a Deposit, a purchase, a sale, a Withdrawal, and the income and cost types.
// The Dividend and the Withholding tax have a ticker, but no Position ID, like in a real export.
//
//	Cash:            2000.00 - 500.00 + 320.00 + 12.50 - 1.88 + 3.40 - 0.65 - 0.03 - 0.10 - 300.00 = 1533.24
//	Net Deposits:    2000.00 - 300.00                                                              = 1700.00
//	Realized Profit: 320.00 - 500.00 × 4 / 10                                                      =  120.00
//	Net dividends:   12.50 - 1.88                                                                  =   10.62
//	Net interest:    3.40 - 0.65                                                                   =    2.75
//	Fees:            -0.03 - 0.10                                                                  =   -0.13
//	Total result:    120.00 + 10.62 + 2.75 - 0.13                                                  =  133.24
func incomeAndCostsExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000003",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Withdrawal", Time: opTime(9), Amount: "-300.00", ID: "510", Comment: "Withdrawal to bank account"},
			{Type: "Correction", Time: opTime(7), Amount: "-0.10", ID: "509", Comment: "Correction of interest"},
			{Type: "SEC fee", Ticker: "ABC.US", Instrument: "ABC", Time: opTime(3).Add(time.Second), Amount: "-0.03", ID: "508", Comment: "SEC fee ABC.US"},
			{Type: "Free funds interest tax", Time: opTime(5).Add(time.Second), Amount: "-0.65", ID: "507", Comment: "Free-funds Interest Tax 2026-02"},
			{Type: "Free funds interest", Time: opTime(5), Amount: "3.40", ID: "506", Comment: "Free-funds Interest 2026-02"},
			{Type: "Withholding tax", Ticker: "ABC.US", Instrument: "ABC", Time: opTime(4).Add(time.Second), Amount: "-1.88", ID: "505", Comment: "ABC.US USD WHT 15%"},
			{Type: "Dividend", Ticker: "ABC.US", Instrument: "ABC", Time: opTime(4), Amount: "12.50", ID: "504", Comment: "ABC.US USD 2.0833/ SHR"},
			{Type: "Stock sell", Ticker: "ABC.US", Instrument: "ABC", Time: opTime(3), Amount: "320.00", ID: "503", Comment: "CLOSE BUY 4/10 @ 80.00", PositionID: "4000000001"},
			{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Time: opTime(2), Amount: "-500.00", ID: "502", Comment: "OPEN BUY 10 @ 50.00", PositionID: "4000000001"},
			{Type: "Deposit", Time: opTime(1), Amount: "2000.00", ID: "501", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "ABC.US", Volume: "4", OpenTime: opTime(2), CloseTime: opTime(3), ProfitLoss: "120.00", PositionID: "4000000001"},
		},
	}
}

func TestAccountSummary_IncomeAndCosts_ReturnsEachPartAndTotalResult(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, incomeAndCostsExport(), "XTB USD")

	var got summaryResponse
	status := srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", id), &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, incomeAndCostsSummary(id), got)
}

func TestImport_IncomeAndCosts_SameFileAgain_GivesNoNewOperationsAndNoConflicts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, incomeAndCostsExport(), "XTB USD")

	again := srv.preview(t, incomeAndCostsExport())

	assert.Equal(t, 0, again.NewCashOperations)
	assert.Equal(t, 10, again.SkippedCashOperations)
	assert.Empty(t, again.Conflicts, "a Dividend with an empty Position ID is the same operation again")
	srv.confirm(t, again.ID, nil)
	assert.Equal(t, incomeAndCostsSummary(id), srv.summary(t, id))
}

// incomeAndCostsSummary is the summary of the Account with the ID after the import of incomeAndCostsExport.
func incomeAndCostsSummary(accountID int64) summaryResponse {
	return summaryResponse{
		AccountID:      accountID,
		Currency:       "USD",
		Cash:           "1533.24",
		NetDeposits:    "1700.00",
		RealizedProfit: "120.00",
		NetDividends:   "10.62",
		NetInterest:    "2.75",
		Fees:           "-0.13",
		TotalResult:    "133.24",
	}
}

func TestAccountHoldings_TwoInstruments_ReturnsOpenVolumeAndAverageCost(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, dashboardExport(), "XTB USD")

	var got []holdingResponse
	status := srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", id), &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []holdingResponse{
		{Ticker: "AAPL.US", Name: "Apple", Volume: "3.5", Cost: "612.30", AverageCost: "174.9428571428571429"},
		{Ticker: "MSFT.US", Name: "Microsoft", Volume: "0.3", Cost: "124.55", AverageCost: "415.1666666666666667"},
	}, got)
}

func TestAccountHoldings_NoPurchases_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, depositOnlyExport("56000001", "USD"), "XTB USD")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/holdings", id))

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, "[]", body)
}

func TestAccountDashboard_UnknownAccount_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{
		"/api/accounts/999/summary",
		"/api/accounts/999/holdings",
		"/api/accounts/abc/summary",
		"/api/accounts/abc/holdings",
		"/api/accounts/999/sales",
		"/api/accounts/abc/sales",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			var got errorBody
			status := srv.getJSON(t, path, &got)

			assert.Equal(t, http.StatusNotFound, status)
			assert.NotEmpty(t, got.Error)
		})
	}
}

func TestListAccounts_NoAccounts_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, contentType, body := srv.get(t, "/api/accounts")

	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, contentType, "application/json")
	assert.JSONEq(t, "[]", body)
}

func TestListAccounts_TwoAccounts_OrderedByName(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	usdID := srv.importAccount(t, depositOnlyExport("56000001", "USD"), "XTB USD")
	plnID := srv.importAccount(t, depositOnlyExport("56000002", "PLN"), "XTB PLN")

	var got []accountResponse
	status := srv.getJSON(t, "/api/accounts", &got)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []accountResponse{
		{ID: plnID, AccountNumber: "56000002", Name: "XTB PLN", Type: "regular", Currency: "PLN"},
		{ID: usdID, AccountNumber: "56000001", Name: "XTB USD", Type: "regular", Currency: "USD"},
	}, got)
}

func TestRenameAccount_NewName_ReturnsAndKeepsTrimmedName(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, depositOnlyExport("56000001", "USD"), "XTB USD")

	status, body := srv.patchJSON(t, fmt.Sprintf("/api/accounts/%d", id), map[string]string{"name": "  Main USD  "})

	require.Equal(t, http.StatusOK, status, "body %q", body)
	want := accountResponse{ID: id, AccountNumber: "56000001", Name: "Main USD", Type: "regular", Currency: "USD"}
	var got accountResponse
	decode(t, body, &got)
	assert.Equal(t, want, got)

	var listed []accountResponse
	srv.getJSON(t, "/api/accounts", &listed)
	assert.Equal(t, []accountResponse{want}, listed)
}

func TestRenameAccount_InvalidRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, depositOnlyExport("56000001", "USD"), "XTB USD")

	tests := []struct {
		name       string
		path       string
		body       any
		wantStatus int
	}{
		{name: "empty name", path: fmt.Sprintf("/api/accounts/%d", id), body: map[string]string{"name": "   "}, wantStatus: http.StatusBadRequest},
		{name: "no name", path: fmt.Sprintf("/api/accounts/%d", id), body: map[string]string{}, wantStatus: http.StatusBadRequest},
		{name: "body is not an object", path: fmt.Sprintf("/api/accounts/%d", id), body: []string{"Main"}, wantStatus: http.StatusBadRequest},
		{name: "unknown account", path: fmt.Sprintf("/api/accounts/%d", id+100), body: map[string]string{"name": "Main"}, wantStatus: http.StatusNotFound},
		{name: "account ID is not a number", path: "/api/accounts/abc", body: map[string]string{"name": "Main"}, wantStatus: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// No case renames the Account, so the cases can share it and run at the same time.
			t.Parallel()
			status, body := srv.patchJSON(t, tc.path, tc.body)

			assert.Equal(t, tc.wantStatus, status, "body %q", body)
			var got errorBody
			decode(t, body, &got)
			assert.NotEmpty(t, got.Error)

			var listed []accountResponse
			srv.getJSON(t, "/api/accounts", &listed)
			require.Len(t, listed, 1)
			assert.Equal(t, "XTB USD", listed[0].Name, "a rejected rename changes nothing")
		})
	}
}
