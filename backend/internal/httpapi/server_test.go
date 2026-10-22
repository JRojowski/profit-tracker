package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/httpapi"
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
}

// startServer starts the backend with a new SQLite file in a temporary folder.
// The migrations are applied. The server stops when the test ends.
func startServer(t *testing.T) *testServer {
	t.Helper()
	return startServerWithFrontend(t, testFrontend)
}

// startServerWithFrontend starts the backend and serves the frontend files from frontend.
func startServerWithFrontend(t *testing.T, frontend fs.FS) *testServer {
	t.Helper()

	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "profit-tracker.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	handler := httpapi.NewHandler(httpapi.Options{
		Store:    db,
		Frontend: frontend,
		Logger:   slog.New(slog.DiscardHandler),
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &testServer{URL: srv.URL, client: srv.Client()}
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
