package nbp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange/nbp"
)

// usdAnswer is a recorded NBP answer for USD from 2026-10-20 to 2026-10-21, with a mid that float64 cannot keep.
const usdAnswer = `{"table":"A","currency":"dolar amerykański","code":"USD","rates":[` +
	`{"no":"203/A/NBP/2026","effectiveDate":"2026-10-20","mid":3.6404},` +
	`{"no":"204/A/NBP/2026","effectiveDate":"2026-10-21","mid":3.65120000000000001}]}`

// notFoundAnswer is the recorded NBP answer for a range with no table.
const notFoundAnswer = "404 NotFound - Not Found - Brak danych"

// request is one request that the test server got.
type request struct {
	path, rawQuery, accept, userAgent string
}

// recorder keeps the requests that the test server got. It is safe for concurrent use.
type recorder struct {
	mu       sync.Mutex
	requests []request
}

func (r *recorder) add(req request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

func (r *recorder) all() []request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]request(nil), r.requests...)
}

func (r *recorder) paths() []string {
	var paths []string
	for _, req := range r.all() {
		paths = append(paths, req.path)
	}
	return paths
}

// startNBP starts a server that answers each request with answer. It returns the provider for the server
// and the recorder of the requests.
func startNBP(t *testing.T, answer http.HandlerFunc) (*nbp.Provider, *recorder) {
	t.Helper()
	got := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.add(request{
			path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, accept: r.Header.Get("Accept"), userAgent: r.UserAgent(),
		})
		answer(w, r)
	}))
	t.Cleanup(srv.Close)
	return &nbp.Provider{Client: srv.Client(), BaseURL: srv.URL}, got
}

// fixed returns a handler that answers with the status and the body.
func fixed(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// day returns the day as YYYY-MM-DD at midnight UTC.
func day(t *testing.T, date string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, date)
	require.NoError(t, err)
	return d
}

func TestRates_NormalAnswer_ReturnsExactRatesWithTableDateAndNumber(t *testing.T) {
	t.Parallel()
	provider, requests := startNBP(t, fixed(http.StatusOK, usdAnswer))

	got, err := provider.Rates(t.Context(), "USD", day(t, "2026-10-20"), day(t, "2026-10-21"))

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, day(t, "2026-10-20"), got[0].Date)
	assert.Equal(t, "3.6404", got[0].Mid.String())
	assert.Equal(t, "203/A/NBP/2026", got[0].Table)
	assert.Equal(t, day(t, "2026-10-21"), got[1].Date)
	assert.Equal(t, "3.65120000000000001", got[1].Mid.String(), "the rate must not go through float64")
	assert.Equal(t, "204/A/NBP/2026", got[1].Table)
	all := requests.all()
	require.Len(t, all, 1)
	req := all[0]
	assert.Equal(t, "/api/exchangerates/rates/a/usd/2026-10-20/2026-10-21/", req.path)
	assert.Equal(t, "format=json", req.rawQuery)
	assert.Equal(t, "application/json", req.accept)
	assert.NotEmpty(t, req.userAgent)
	assert.NotContains(t, req.userAgent, "Go-http-client", "the NBP answers 403 to the Go default User-Agent")
}

func TestRates_NotFound_ReturnsEmptyListAndNoError(t *testing.T) {
	t.Parallel()
	provider, _ := startNBP(t, fixed(http.StatusNotFound, notFoundAnswer))

	// A Saturday and a Sunday have no table.
	got, err := provider.Rates(t.Context(), "USD", day(t, "2026-10-24"), day(t, "2026-10-25"))

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRates_EmptyRange_SendsNoRequest(t *testing.T) {
	t.Parallel()
	provider, requests := startNBP(t, fixed(http.StatusOK, usdAnswer))

	got, err := provider.Rates(t.Context(), "USD", day(t, "2026-10-21"), day(t, "2026-10-20"))

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, requests.all())
}

// TestRates_LongRange_SplitsIntoPartsOfAtMost367Days checks the split. The NBP answers 400 for a longer range.
//
//	2024-01-01 + 366 days = 2025-01-01 (2024 has 366 days)
//	2025-01-02 + 366 days = 2026-01-03
//	2026-01-04 to 2026-01-10
func TestRates_LongRange_SplitsIntoPartsOfAtMost367Days(t *testing.T) {
	t.Parallel()
	// The server answers each part with one rate on the first day of the part.
	provider, requests := startNBP(t, func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		from := parts[len(parts)-2]
		fixed(http.StatusOK, fmt.Sprintf(`{"table":"A","code":"USD","rates":[{"no":"1/A/NBP/%s","effectiveDate":%q,"mid":4.01}]}`,
			from[:4], from))(w, r)
	})

	got, err := provider.Rates(t.Context(), "USD", day(t, "2024-01-01"), day(t, "2026-01-10"))

	require.NoError(t, err)
	assert.Equal(t, []string{
		"/api/exchangerates/rates/a/usd/2024-01-01/2025-01-01/",
		"/api/exchangerates/rates/a/usd/2025-01-02/2026-01-03/",
		"/api/exchangerates/rates/a/usd/2026-01-04/2026-01-10/",
	}, requests.paths())
	want := []exchange.Rate{
		{Date: day(t, "2024-01-01"), Table: "1/A/NBP/2024"},
		{Date: day(t, "2025-01-02"), Table: "1/A/NBP/2025"},
		{Date: day(t, "2026-01-04"), Table: "1/A/NBP/2026"},
	}
	require.Len(t, got, len(want))
	for i, w := range want {
		assert.Equal(t, w.Date, got[i].Date)
		assert.Equal(t, w.Table, got[i].Table)
		assert.Equal(t, "4.01", got[i].Mid.String())
	}
}

func TestRates_BadAnswers_ReturnErrorThatNamesTheCurrency(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "range too long", status: http.StatusBadRequest, body: "400 BadRequest - Przekroczony limit 367 dni / Limit of 367 days has been exceeded"},
		{name: "server error", status: http.StatusInternalServerError, body: "<html>error</html>"},
		{name: "not JSON", status: http.StatusOK, body: "not json"},
		{name: "no mid", status: http.StatusOK, body: `{"code":"USD","rates":[{"no":"1/A/NBP/2026","effectiveDate":"2026-10-21"}]}`},
		{name: "mid is text", status: http.StatusOK, body: `{"code":"USD","rates":[{"no":"1/A/NBP/2026","effectiveDate":"2026-10-21","mid":"x"}]}`},
		{name: "bad date", status: http.StatusOK, body: `{"code":"USD","rates":[{"no":"1/A/NBP/2026","effectiveDate":"21.10.2026","mid":3.6}]}`},
		{name: "other currency", status: http.StatusOK, body: `{"code":"EUR","rates":[{"no":"1/A/NBP/2026","effectiveDate":"2026-10-21","mid":4.2}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, _ := startNBP(t, fixed(tc.status, tc.body))

			_, err := provider.Rates(t.Context(), "USD", day(t, "2026-10-20"), day(t, "2026-10-21"))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "USD", "the error must name the currency")
		})
	}
}
