package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestUnknownAPIPath_ReturnsJSONNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, path := range []string{"/api/unknown", "/api/accounts/7/unknown", "/api/"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Error string `json:"error"`
			}
			status := srv.getJSON(t, path, &got)

			assert.Equal(t, http.StatusNotFound, status)
			assert.Contains(t, got.Error, path)
		})
	}
}

func TestFrontend_ServesFilesAndFallsBackToIndex(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	tests := []struct {
		name     string
		path     string
		wantBody string
	}{
		{name: "root serves index.html", path: "/", wantBody: "<title>profit-tracker</title>"},
		{name: "asset file", path: "/assets/app.js", wantBody: "console.log('app')"},
		{name: "client route falls back to index.html", path: "/accounts/7", wantBody: "<title>profit-tracker</title>"},
		{name: "nested client route falls back to index.html", path: "/imports/3/conflicts", wantBody: "<title>profit-tracker</title>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, _, body := srv.get(t, tc.path)

			assert.Equal(t, http.StatusOK, status)
			assert.Contains(t, body, tc.wantBody)
		})
	}
}

func TestFrontend_MissingFile_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	// A missing asset must not get index.html. The browser would run HTML as a script.
	for _, path := range []string{"/assets/index-OLD.js", "/assets/no-extension", "/favicon.ico", "/accounts/7/report.pdf"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			status, _, body := srv.get(t, path)

			assert.Equal(t, http.StatusNotFound, status)
			assert.NotContains(t, body, "<title>profit-tracker</title>")
		})
	}
}

func TestFrontend_NotBuilt_ShowsBuildMessage(t *testing.T) {
	t.Parallel()
	srv := startServerWithFrontend(t, fstest.MapFS{})

	for _, path := range []string{"/", "/accounts/7"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			status, contentType, body := srv.get(t, path)

			assert.Equal(t, http.StatusOK, status)
			assert.True(t, strings.HasPrefix(contentType, "text/plain"), "content type %q", contentType)
			assert.Contains(t, body, "frontend is not built")
		})
	}
}

func TestFrontend_RejectsWriteMethods(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, _, _ := srv.do(t, http.MethodPost, "/accounts/7", nil)

	assert.Equal(t, http.StatusMethodNotAllowed, status)
}
