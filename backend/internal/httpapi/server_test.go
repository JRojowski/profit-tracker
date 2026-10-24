package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/httpapi"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// testFrontend is a small stand-in for the built frontend.
var testFrontend = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>profit-tracker</title>")},
	"assets/app.js": {Data: []byte("console.log('app')")},
}

// testServer is the backend that runs for one test, with its own database.
type testServer struct {
	URL    string
	client *http.Client
	// prices is the price service of the server. A test calls it to run the refresh at start.
	prices *price.Service
	// clock is the clock of the server. A test moves it to change "today".
	clock *testClock
	// db is the database of the server. Only a test that needs data that no endpoint shows reads it.
	db *store.Store
}

// startServer starts the backend with a new SQLite file in a temporary folder.
// The migrations are applied. The price provider has no prices. The server stops when the test ends.
func startServer(t *testing.T) *testServer {
	t.Helper()
	return startServerWith(t, testFrontend, newFakeProvider())
}

// startServerWithFrontend starts the backend and serves the frontend files from frontend.
func startServerWithFrontend(t *testing.T, frontend fs.FS) *testServer {
	t.Helper()
	return startServerWith(t, frontend, newFakeProvider())
}

// startServerWithPrices starts the backend with the fake price provider.
func startServerWithPrices(t *testing.T, provider *fakeProvider) *testServer {
	t.Helper()
	return startServerWith(t, testFrontend, provider)
}

// startServerWith starts the backend with the frontend files and the price provider.
// The clock starts at testToday.
func startServerWith(t *testing.T, frontend fs.FS, provider price.Provider) *testServer {
	t.Helper()

	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "profit-tracker.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	clock := &testClock{now: testToday}
	prices := price.NewService(db, provider, clock.Now)
	handler := httpapi.NewHandler(httpapi.Options{
		Store:    db,
		Frontend: frontend,
		Logger:   slog.New(slog.DiscardHandler),
		Prices:   prices,
		Now:      clock.Now,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &testServer{URL: srv.URL, client: srv.Client(), prices: prices, clock: clock, db: db}
}

// testToday is the time of the server clock at the start of a test.
var testToday = time.Date(2026, 10, 23, 9, 30, 0, 0, time.UTC)

// testClock is a clock that a test sets. It is safe for concurrent use.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

// Now returns the time of the clock.
func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// set changes the time of the clock.
func (c *testClock) set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

// fakeProvider is a price.Provider with fixed quotes. It records each requested price ticker.
// It is safe for concurrent use.
type fakeProvider struct {
	mu        sync.Mutex
	quotes    map[string]price.Quote
	failure   error
	requested []string
	// onRequest runs for each request, before the fake answers. It can block or cancel a context.
	onRequest func(ctx context.Context, ticker string) error
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{quotes: map[string]price.Quote{}}
}

// LastClose returns the quote for the price ticker, or an error if the fake has no quote or must fail.
func (f *fakeProvider) LastClose(ctx context.Context, ticker string) (price.Quote, error) {
	f.mu.Lock()
	f.requested = append(f.requested, ticker)
	onRequest := f.onRequest
	f.mu.Unlock()
	// The hook runs with no lock, so that a blocked hook does not block requestedTickers.
	if onRequest != nil {
		if err := onRequest(ctx, ticker); err != nil {
			return price.Quote{}, err
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failure != nil {
		return price.Quote{}, f.failure
	}
	q, ok := f.quotes[ticker]
	if !ok {
		return price.Quote{}, fmt.Errorf("the fake has no quote for %s", ticker)
	}
	return q, nil
}

// set sets the quote for the price ticker: the close price, its currency and the price date as YYYY-MM-DD.
func (f *fakeProvider) set(t *testing.T, ticker, closePrice, currency, date string) {
	t.Helper()
	at, err := time.Parse(time.DateOnly, date)
	require.NoError(t, err)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quotes[ticker] = price.Quote{Close: decimal.RequireFromString(closePrice), Currency: currency, Date: at, Source: "fake"}
}

// setOnRequest sets the hook that runs for each later request.
func (f *fakeProvider) setOnRequest(hook func(ctx context.Context, ticker string) error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onRequest = hook
}

// failWith makes each later call fail with err.
func (f *fakeProvider) failWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failure = err
}

// requestedTickers returns the requested price tickers, in request order.
func (f *fakeProvider) requestedTickers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requested...)
}

// get sends a GET request to path and returns the status, the content type and the body.
func (s *testServer) get(t *testing.T, path string) (status int, contentType, body string) {
	t.Helper()
	return s.do(t, http.MethodGet, path, nil)
}

// do sends a request to path and returns the status, the content type and the body.
func (s *testServer) do(t *testing.T, method, path string, reqBody io.Reader) (status int, contentType, body string) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, s.URL+path, reqBody)
	require.NoError(t, err)
	resp, err := s.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(data)
}

// getJSON sends a GET request to path, checks the JSON content type and decodes the body into v.
func (s *testServer) getJSON(t *testing.T, path string, v any) int {
	t.Helper()

	status, contentType, body := s.get(t, path)
	require.True(t, strings.HasPrefix(contentType, "application/json"), "content type %q, body %q", contentType, body)
	require.NoError(t, json.Unmarshal([]byte(body), v), "body %q", body)
	return status
}

// upload sends data as the multipart field "file" with the file name to POST /api/imports.
// It returns the status and the body.
func (s *testServer) upload(t *testing.T, fileName string, data []byte) (status int, body string) {
	t.Helper()

	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	part, err := form.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, form.Close())

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.URL+"/api/imports", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := s.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(got)
}

// postJSON sends v as a JSON body to path. A nil v sends no body.
// It returns the status and the body.
func (s *testServer) postJSON(t *testing.T, path string, v any) (status int, body string) {
	t.Helper()

	var reqBody io.Reader
	if v != nil {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		reqBody = bytes.NewReader(data)
	}
	status, _, body = s.do(t, http.MethodPost, path, reqBody)
	return status, body
}

// decode decodes the JSON body into v and stops the test if it is not valid JSON.
func decode(t *testing.T, body string, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal([]byte(body), v), "body %q", body)
}
