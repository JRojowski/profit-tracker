package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

const questionBody = `{"messages": [{"role": "user", "content": "What are my Holdings?"}]}`

func TestAskAssistant_RequestFromAnotherSite_IsRefusedAndTheModelGetsNoCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		header     http.Header
		wantStatus int
		wantCode   string
	}{
		{
			name:       "a text/plain body, which a page sends with no preflight",
			header:     http.Header{"Content-Type": {"text/plain"}},
			wantStatus: http.StatusUnsupportedMediaType, wantCode: "invalid_request",
		},
		{
			name:       "no content type",
			header:     http.Header{},
			wantStatus: http.StatusUnsupportedMediaType, wantCode: "invalid_request",
		},
		{
			name: "Sec-Fetch-Site says cross-site",
			header: http.Header{
				"Content-Type": {"application/json"}, "Sec-Fetch-Site": {"cross-site"}, "Origin": {"http://evil.example"},
			},
			wantStatus: http.StatusForbidden, wantCode: "invalid_request",
		},
		{
			name: "an old browser sends an Origin that is not the Host",
			header: http.Header{
				"Content-Type": {"application/json"}, "Origin": {"http://evil.example"},
			},
			wantStatus: http.StatusForbidden, wantCode: "invalid_request",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			model := newFakeModel(assistant.Reply{Text: "An answer."})
			srv := startServerWithOptions(t, serverOptions{model: model})

			status, _, body := srv.doWithHeader(t, http.MethodPost, "/api/assistant/questions", strings.NewReader(questionBody), tc.header)

			require.Equal(t, tc.wantStatus, status, "body %q", body)
			var got errorWithCode
			decode(t, body, &got)
			assert.NotEmpty(t, got.Error)
			assert.Equal(t, tc.wantCode, got.Code)
			assert.Empty(t, model.recordedCalls(), "the model must get no call")
		})
	}
}

func TestAskAssistant_RequestFromTheOwnPage_IsAccepted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
	}{
		{
			name:   "same origin",
			header: http.Header{"Content-Type": {"application/json"}, "Sec-Fetch-Site": {"same-origin"}},
		},
		{
			name:   "no browser, for example curl",
			header: http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		},
		{
			// The Vite dev server forwards the Host and the Origin of the page to the backend.
			name: "Vite dev proxy: the Origin is the Host that the proxy forwards",
			header: http.Header{
				"Content-Type": {"application/json"}, "Host": {"localhost:5173"}, "Origin": {"http://localhost:5173"},
			},
		},
		{
			name: "Vite dev proxy of an old browser, with no Sec-Fetch-Site",
			header: http.Header{
				"Content-Type": {"application/json"}, "Sec-Fetch-Site": {"same-origin"},
				"Host": {"localhost:5173"}, "Origin": {"http://localhost:5173"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			model := newFakeModel(assistant.Reply{Text: "An answer."})
			srv := startServerWithOptions(t, serverOptions{model: model})

			status, _, body := srv.doWithHeader(t, http.MethodPost, "/api/assistant/questions", strings.NewReader(questionBody), tc.header)

			assert.Equal(t, http.StatusOK, status, "body %q", body)
		})
	}
}
