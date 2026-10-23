package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// warningData is one warning of the Import preview.
type warningData struct {
	Kind  string `json:"kind"`
	XTBID string `json:"xtbId"`
	Time  string `json:"time"`
	// Direction is "in" for a Transfer In and "out" for a Transfer Out.
	Direction string `json:"direction"`
	Amount    string `json:"amount"`
}

// transferPreviewResponse is the part of the preview body that the Transfer tests read.
type transferPreviewResponse struct {
	ID int64 `json:"id"`
	// Warnings is nil if the body has null or no "warnings". An empty JSON array gives an empty, non-nil slice.
	Warnings []warningData `json:"warnings"`
}

// Invented account numbers of the ADR 0003 example.
const (
	plnAccountNumber  = "56000010"
	ikzeAccountNumber = "56000020"
)

// adr0003TransferOutTime and adr0003TransferInTime are the times of the two sides in ADR 0003, 15 ms apart.
var (
	adr0003TransferOutTime = time.Date(2026, 2, 6, 16, 0, 43, 157_000_000, time.UTC)
	adr0003TransferInTime  = time.Date(2026, 2, 6, 16, 0, 43, 172_000_000, time.UTC)
)

// adr0003PLNExport is the PLN Account of the ADR 0003 example: a Deposit and a Transfer Out.
//
//	Net Deposits: 10000 - 10000 = 0
func adr0003PLNExport() xtbtest.Export {
	return plnExportWithTransferOut(adr0003TransferOutTime)
}

// plnExportWithTransferOut is the PLN Account of the ADR 0003 example with the Transfer Out at the time.
func plnExportWithTransferOut(at time.Time) xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: plnAccountNumber,
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "IKZE deposit", Time: at, Amount: "-10000.00", ID: "3001", Comment: "Transfer out operation on account with id " + plnAccountNumber},
			{Type: "Deposit", Time: time.Date(2026, 2, 6, 15, 0, 0, 0, time.UTC), Amount: "10000.00", ID: "3000", Comment: "Blik deposit"},
		},
	}
}

// adr0003IKZEExport is the IKZE Account of the ADR 0003 example: a Transfer In.
//
//	Net Deposits: 10000
func adr0003IKZEExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: ikzeAccountNumber,
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "IKZE deposit", Time: adr0003TransferInTime, Amount: "10000.00", ID: "4001", Comment: "Transfer in operation on account with id " + ikzeAccountNumber},
		},
	}
}

// previewWarnings uploads the export and returns the preview ID and its warnings.
// It stops the test if the status is not 200 or if "warnings" is not a JSON array.
func (s *testServer) previewWarnings(t *testing.T, export xtbtest.Export) (int64, []warningData) {
	t.Helper()

	status, body := s.upload(t, export.FileName(), xtbtest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got transferPreviewResponse
	decode(t, body, &got)
	require.NotNil(t, got.Warnings, "warnings must be an array, never null or missing: %s", body)
	return got.ID, got.Warnings
}

// importWithWarnings imports the export into a new PLN Account of the type and returns the Account ID
// and the warnings of the preview.
func (s *testServer) importWithWarnings(t *testing.T, export xtbtest.Export, name, typ string) (int64, []warningData) {
	t.Helper()

	id, warnings := s.previewWarnings(t, export)
	saved := s.confirm(t, id, newAccountBody(name, typ, "PLN"))
	return saved.AccountID, warnings
}

func TestTransfers_ADR0003Example_NetDepositsPerAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	plnID, _ := srv.importWithWarnings(t, adr0003PLNExport(), "XTB PLN", "regular")
	ikzeID, _ := srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")

	pln := srv.summary(t, plnID)
	assert.Equal(t, "0.00", pln.NetDeposits)
	assert.Equal(t, "0.00", pln.Cash, "a Transfer Out changes the cash balance")
	ikze := srv.summary(t, ikzeID)
	assert.Equal(t, "10000.00", ikze.NetDeposits)
	assert.Equal(t, "10000.00", ikze.Cash, "a Transfer In changes the cash balance")
}

func TestImportPreview_TransferWithoutOtherSide_GivesWarningAndConfirmSucceeds(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	id, warnings := srv.previewWarnings(t, adr0003PLNExport())

	assert.Equal(t, []warningData{{
		Kind:      "transfer-without-other-side",
		XTBID:     "3001",
		Time:      "2026-02-06T16:00:43Z",
		Direction: "out",
		Amount:    "-10000.00",
	}}, warnings)
	saved := srv.confirm(t, id, newAccountBody("XTB PLN", "regular", "PLN"))
	assert.Equal(t, "0.00", srv.summary(t, saved.AccountID).NetDeposits)
}

func TestImportPreview_TransferWithOtherSideInOtherAccount_GivesNoWarning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		first, second xtbtest.Export
	}{
		{name: "PLN first, then IKZE", first: adr0003PLNExport(), second: adr0003IKZEExport()},
		{name: "IKZE first, then PLN", first: adr0003IKZEExport(), second: adr0003PLNExport()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			_, firstWarnings := srv.importWithWarnings(t, tc.first, "first", "regular")
			require.Len(t, firstWarnings, 1, "the first Account has no other side yet")

			_, warnings := srv.previewWarnings(t, tc.second)

			assert.Empty(t, warnings)
		})
	}
}

func TestImportPreview_TransferWindow_IsFiveSecondsInclusive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		outTime     time.Time
		wantWarning bool
	}{
		{name: "5 seconds before the other side", outTime: time.Date(2026, 2, 6, 16, 0, 38, 0, time.UTC), wantWarning: false},
		{name: "5 seconds after the other side", outTime: time.Date(2026, 2, 6, 16, 0, 48, 0, time.UTC), wantWarning: false},
		{name: "6 seconds before the other side", outTime: time.Date(2026, 2, 6, 16, 0, 37, 0, time.UTC), wantWarning: true},
		{name: "6 seconds after the other side", outTime: time.Date(2026, 2, 6, 16, 0, 49, 0, time.UTC), wantWarning: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			// The reader rounds the Transfer In time to 16:00:43.
			srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")

			_, warnings := srv.previewWarnings(t, plnExportWithTransferOut(tc.outTime))

			if !tc.wantWarning {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, "3001", warnings[0].XTBID)
			assert.Equal(t, tc.outTime.Format(time.RFC3339), warnings[0].Time)
		})
	}
}

func TestImportPreview_OppositeTransferOnSameAccount_IsNotOtherSide(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	at := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	transferIn := xtbtest.CashOperation{Type: "IKZE deposit", Time: at, Amount: "500.00", ID: "5001", Comment: "Transfer in operation on account with id " + ikzeAccountNumber}
	transferOut := xtbtest.CashOperation{Type: "IKZE deposit", Time: at.Add(time.Second), Amount: "-500.00", ID: "5002", Comment: "Transfer out operation on account with id " + ikzeAccountNumber}
	srv.importWithWarnings(t, xtbtest.Export{AccountNumber: ikzeAccountNumber, Currency: "PLN", CashOperations: []xtbtest.CashOperation{transferIn}}, "XTB IKZE", "IKZE")

	// A newer export of the same Account has the stored Transfer In and a new Transfer Out with the opposite amount.
	_, warnings := srv.previewWarnings(t, xtbtest.Export{
		AccountNumber:  ikzeAccountNumber,
		Currency:       "PLN",
		CashOperations: []xtbtest.CashOperation{transferOut, transferIn},
	})

	assert.ElementsMatch(t, []warningData{
		{Kind: "transfer-without-other-side", XTBID: "5001", Time: "2026-03-02T12:00:00Z", Direction: "in", Amount: "500.00"},
		{Kind: "transfer-without-other-side", XTBID: "5002", Time: "2026-03-02T12:00:01Z", Direction: "out", Amount: "-500.00"},
	}, warnings)
}

func TestImportPreview_OneStoredTransferIsOtherSideOfOneTransferOnly(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importWithWarnings(t, adr0003IKZEExport(), "XTB IKZE", "IKZE")
	// Two Transfers Out within the window of the one stored Transfer In. The file order is not the time order.
	export := adr0003PLNExport()
	export.CashOperations = append([]xtbtest.CashOperation{
		{Type: "IKZE deposit", Time: adr0003TransferOutTime.Add(time.Second), Amount: "-10000.00", ID: "3002", Comment: "Transfer out operation on account with id " + plnAccountNumber},
	}, export.CashOperations...)
	export.CashOperations = append(export.CashOperations,
		xtbtest.CashOperation{Type: "Deposit", Time: time.Date(2026, 2, 6, 15, 30, 0, 0, time.UTC), Amount: "10000.00", ID: "2999", Comment: "Blik deposit"})

	_, warnings := srv.previewWarnings(t, export)

	// The Transfers of the file take the other sides in time order, so the later Transfer Out has none.
	assert.Equal(t, []warningData{{
		Kind:      "transfer-without-other-side",
		XTBID:     "3002",
		Time:      "2026-02-06T16:00:44Z",
		Direction: "out",
		Amount:    "-10000.00",
	}}, warnings)
}

func TestImportPreview_NoTransfers_GivesEmptyWarningsArray(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	_, warnings := srv.previewWarnings(t, usdExport())

	assert.Empty(t, warnings)
}
