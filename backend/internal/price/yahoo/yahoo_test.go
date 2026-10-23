package yahoo_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/price/yahoo"
)

// chartBody returns a chart response with the price, the currency, the market time and the GMT offset in seconds.
func chartBody(closePrice, currency string, marketTime time.Time, gmtOffset int) string {
	return fmt.Sprintf(`{"chart":{"result":[{"meta":{"currency":%q,"symbol":"X","regularMarketPrice":%s,`+
		`"regularMarketTime":%d,"gmtoffset":%d,"exchangeTimezoneName":"X"},"timestamp":[1],"indicators":{}}],"error":null}}`,
		currency, closePrice, marketTime.Unix(), gmtOffset)
}

// request is one request that the test server got.
type request struct {
	path, rawQuery, userAgent string
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

// startYahoo starts a server that answers each request with the status and the body.
// It returns the provider for the server and the recorder of the requests.
func startYahoo(t *testing.T, status int, body string) (*yahoo.Provider, *recorder) {
	t.Helper()
	got := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.add(request{path: r.URL.EscapedPath(), rawQuery: r.URL.RawQuery, userAgent: r.UserAgent()})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &yahoo.Provider{Client: srv.Client(), BaseURL: srv.URL}, got
}

func TestLastClose_USDPrice_ReturnsExactPriceAndDateInExchangeTime(t *testing.T) {
	t.Parallel()
	// 02:00 UTC on 23 October is 22:00 on 22 October in New York (UTC-4).
	marketTime := time.Date(2026, 10, 23, 2, 0, 0, 0, time.UTC)
	provider, requests := startYahoo(t, http.StatusOK, chartBody("271.4900000000001", "USD", marketTime, -14400))

	got, err := provider.LastClose(t.Context(), "AAPL")

	require.NoError(t, err)
	assert.Equal(t, "271.4900000000001", got.Close.String(), "the price must not go through float64")
	assert.Equal(t, "USD", got.Currency)
	assert.Equal(t, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), got.Date)
	assert.Equal(t, yahoo.Source, got.Source)
	all := requests.all()
	require.Len(t, all, 1)
	req := all[0]
	assert.Equal(t, "/v8/finance/chart/AAPL", req.path)
	assert.Equal(t, "range=5d&interval=1d", req.rawQuery)
	assert.NotContains(t, req.userAgent, "Go-http-client", "Yahoo refuses the Go default User-Agent")
}

func TestLastClose_LondonPriceInPence_ReturnsPounds(t *testing.T) {
	t.Parallel()
	provider, _ := startYahoo(t, http.StatusOK, chartBody("8234.5", "GBp", time.Date(2026, 10, 22, 15, 35, 0, 0, time.UTC), 3600))

	got, err := provider.LastClose(t.Context(), "VUSA.L")

	require.NoError(t, err)
	assert.Equal(t, "82.345", got.Close.String())
	assert.Equal(t, "GBP", got.Currency)
	assert.Equal(t, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), got.Date)
}

func TestLastClose_UnknownSymbol_ReturnsErrorWithDescription(t *testing.T) {
	t.Parallel()
	provider, _ := startYahoo(t, http.StatusNotFound,
		`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)

	_, err := provider.LastClose(t.Context(), "NOPE")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE")
	assert.Contains(t, err.Error(), "No data found, symbol may be delisted")
}

func TestLastClose_BadResponses_ReturnError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "server error with no JSON", status: http.StatusInternalServerError, body: "<html>error</html>"},
		{name: "no result", status: http.StatusOK, body: `{"chart":{"result":[],"error":null}}`},
		{name: "no price", status: http.StatusOK, body: `{"chart":{"result":[{"meta":{"currency":"USD","regularMarketTime":1,"gmtoffset":0}}],"error":null}}`},
		{name: "no currency", status: http.StatusOK, body: `{"chart":{"result":[{"meta":{"regularMarketPrice":1.5,"regularMarketTime":1,"gmtoffset":0}}],"error":null}}`},
		{name: "no market time", status: http.StatusOK, body: `{"chart":{"result":[{"meta":{"currency":"USD","regularMarketPrice":1.5,"gmtoffset":0}}],"error":null}}`},
		{name: "not JSON", status: http.StatusOK, body: "not json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, _ := startYahoo(t, tc.status, tc.body)

			_, err := provider.LastClose(t.Context(), "AAPL")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "AAPL", "the error must name the ticker")
		})
	}
}
