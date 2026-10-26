package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// ikzeLimitResponse is the body of GET /api/accounts/{id}/ikze-limit.
type ikzeLimitResponse struct {
	CurrentYear int            `json:"currentYear"`
	Rows        []ikzeLimitRow `json:"rows"`
}

// ikzeLimitRow is one year of the IKZE Limit response. The limit fields are nil for a year with no IKZE Limit.
type ikzeLimitRow struct {
	Year          int     `json:"year"`
	Contributions string  `json:"contributions"`
	Limit         *string `json:"limit"`
	Unused        *string `json:"unused"`
	OverLimit     *string `json:"overLimit"`
}

// ikzeLimit returns the IKZE Limit of the Account. It stops the test if the status is not 200.
func (s *testServer) ikzeLimit(t *testing.T, accountID int64) ikzeLimitResponse {
	t.Helper()

	var got ikzeLimitResponse
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/ikze-limit", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// importIKZE imports the Cash Operations into a new PLN Account of the type IKZE and returns the Account ID.
func (s *testServer) importIKZE(t *testing.T, ops ...xtbtest.CashOperation) int64 {
	t.Helper()

	id, _ := s.importWithWarnings(t, xtbtest.Export{AccountNumber: "57000001", Currency: "PLN", CashOperations: ops}, "XTB IKZE", "IKZE")
	return id
}

// transferIn is a Transfer In of the amount at the time. See ADR 0003.
func transferIn(id, amount string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{Type: "IKZE deposit", Time: at, Amount: amount, ID: id, Comment: "Transfer in operation on account with id 57000001"}
}

// ikzeDeposit is a Deposit of the amount at the time.
func ikzeDeposit(id, amount string, at time.Time) xtbtest.CashOperation {
	return xtbtest.CashOperation{Type: "Deposit", Time: at, Amount: amount, ID: id, Comment: "Blik deposit"}
}

// limitRow is a row of a year with an IKZE Limit.
func limitRow(year int, contributions, limit, unused, overLimit string) ikzeLimitRow {
	return ikzeLimitRow{Year: year, Contributions: contributions, Limit: &limit, Unused: &unused, OverLimit: &overLimit}
}

func TestIKZELimit_TransferInAtNewYear_CountsInTheYearInPolishTime(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// 2025-12-31 22:30 UTC is 23:30 in Poland, still 2025. 2025-12-31 23:30 UTC is 2026-01-01 00:30 in Poland.
	id := srv.importIKZE(t,
		transferIn("8002", "2000.00", time.Date(2025, 12, 31, 23, 30, 0, 0, time.UTC)),
		transferIn("8001", "1000.00", time.Date(2025, 12, 31, 22, 30, 0, 0, time.UTC)),
	)

	got := srv.ikzeLimit(t, id)

	assert.Equal(t, ikzeLimitResponse{CurrentYear: 2026, Rows: []ikzeLimitRow{
		limitRow(2026, "2000.00", "16956.00", "14956.00", "0.00"),
		limitRow(2025, "1000.00", "15611.40", "14611.40", "0.00"),
	}}, got)
}

func TestIKZELimit_ContributionsAgainstTheLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ops  []xtbtest.CashOperation
		want []ikzeLimitRow
	}{
		{
			name: "contributions equal to the limit leave 0 not used",
			ops: []xtbtest.CashOperation{
				transferIn("8012", "6956.00", time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)),
				ikzeDeposit("8011", "10000.00", time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)),
			},
			want: []ikzeLimitRow{limitRow(2026, "16956.00", "16956.00", "0.00", "0.00")},
		},
		{
			name: "contributions above the limit give the amount above the limit",
			ops: []xtbtest.CashOperation{
				ikzeDeposit("8021", "16000.00", time.Date(2025, 3, 3, 10, 0, 0, 0, time.UTC)),
			},
			want: []ikzeLimitRow{
				limitRow(2026, "0.00", "16956.00", "16956.00", "0.00"),
				limitRow(2025, "16000.00", "15611.40", "0.00", "388.60"),
			},
		},
		{
			name: "a withdrawal and a Transfer Out do not reduce the contributions",
			ops: []xtbtest.CashOperation{
				transferIn("8034", "-1000.00", time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)),
				{Type: "Withdrawal", Time: time.Date(2025, 8, 1, 10, 0, 0, 0, time.UTC), Amount: "-5000.00", ID: "8033", Comment: "Withdrawal to bank account"},
				ikzeDeposit("8032", "3000.00", time.Date(2025, 6, 2, 10, 0, 0, 0, time.UTC)),
				transferIn("8031", "10000.00", time.Date(2025, 2, 6, 16, 0, 43, 0, time.UTC)),
			},
			want: []ikzeLimitRow{
				limitRow(2026, "0.00", "16956.00", "16956.00", "0.00"),
				limitRow(2025, "13000.00", "15611.40", "2611.40", "0.00"),
			},
		},
		{
			name: "a year with no contribution between the first year and today has a row",
			ops: []xtbtest.CashOperation{
				ikzeDeposit("8041", "5000.00", time.Date(2024, 4, 2, 10, 0, 0, 0, time.UTC)),
			},
			want: []ikzeLimitRow{
				limitRow(2026, "0.00", "16956.00", "16956.00", "0.00"),
				limitRow(2025, "0.00", "15611.40", "15611.40", "0.00"),
				{Year: 2024, Contributions: "5000.00"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			id := srv.importIKZE(t, tc.ops...)

			got := srv.ikzeLimit(t, id)

			assert.Equal(t, ikzeLimitResponse{CurrentYear: 2026, Rows: tc.want}, got)
		})
	}
}

func TestIKZELimit_YearWithNoLimit_LimitUnknown(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importIKZE(t, ikzeDeposit("8051", "16956.00", time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)))
	srv.clock.set(time.Date(2027, 1, 15, 9, 0, 0, 0, time.UTC))

	got := srv.ikzeLimit(t, id)

	assert.Equal(t, ikzeLimitResponse{CurrentYear: 2027, Rows: []ikzeLimitRow{
		{Year: 2027, Contributions: "0.00"},
		limitRow(2026, "16956.00", "16956.00", "0.00", "0.00"),
	}}, got)
}

func TestIKZELimit_NoContribution_EmptyRows(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importIKZE(t, xtbtest.CashOperation{
		Type: "Free funds interest", Time: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC), Amount: "1.00", ID: "8061", Comment: "Free-funds Interest",
	})

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/ikze-limit", id))

	require.Equal(t, http.StatusOK, status, "body %q", body)
	assert.JSONEq(t, `{"currentYear": 2026, "rows": []}`, body)
}

func TestIKZELimit_AccountNotIKZE_BadRequest(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, depositOnlyExport("56000001", "PLN"), "XTB PLN")

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/ikze-limit", id))

	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, fmt.Sprintf(`{"error": "account %d is of the type regular, not IKZE"}`, id), body)
}

func TestIKZELimit_UnknownAccount_NotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, _, body := srv.get(t, "/api/accounts/999/ikze-limit")

	assert.Equal(t, http.StatusNotFound, status, "body %q", body)
}

func TestIKZELimit_IKZEAccountNotInPLN_BadRequest(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := xtbtest.Export{AccountNumber: "57000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		ikzeDeposit("8071", "1000.00", time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)),
	}}
	id := srv.confirm(t, srv.preview(t, export).ID, newAccountBody("XTB IKZE USD", "IKZE", "USD")).AccountID

	status, _, body := srv.get(t, fmt.Sprintf("/api/accounts/%d/ikze-limit", id))

	assert.Equal(t, http.StatusBadRequest, status)
	assert.JSONEq(t, fmt.Sprintf(`{"error": "account %d has the currency USD. The IKZE Limit needs an Account in PLN."}`, id), body)
}

// TestIKZELimit_ContributionAfterTheCurrentYear_NoRow checks that the rows stop at the current year.
// The clock is before the contribution of 2026, so that contribution gets no row.
// With the clock before each contribution, there is no row.
func TestIKZELimit_ContributionAfterTheCurrentYear_NoRow(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importIKZE(t,
		ikzeDeposit("8082", "2000.00", time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)),
		ikzeDeposit("8081", "1000.00", time.Date(2025, 3, 3, 10, 0, 0, 0, time.UTC)),
	)
	srv.clock.set(time.Date(2025, 12, 1, 9, 0, 0, 0, time.UTC))

	got := srv.ikzeLimit(t, id)

	assert.Equal(t, ikzeLimitResponse{CurrentYear: 2025, Rows: []ikzeLimitRow{
		limitRow(2025, "1000.00", "15611.40", "14611.40", "0.00"),
	}}, got)

	srv.clock.set(time.Date(2023, 12, 1, 9, 0, 0, 0, time.UTC))

	assert.Equal(t, ikzeLimitResponse{CurrentYear: 2023, Rows: []ikzeLimitRow{}}, srv.ikzeLimit(t, id))
}
