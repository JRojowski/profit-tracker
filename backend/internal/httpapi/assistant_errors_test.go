package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

// errorWithCode is the JSON error body of the Assistant. The frontend tells the cases apart by Code.
type errorWithCode struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// hangingModel is an assistant.Model that does not answer until its context ends.
type hangingModel struct{}

// Reply blocks until the context ends, and returns the error of the context.
func (hangingModel) Reply(ctx context.Context, _ []assistant.Message, _ []assistant.FunctionDefinition) (assistant.Reply, error) {
	<-ctx.Done()
	return assistant.Reply{}, fmt.Errorf("wait for the provider: %w", ctx.Err())
}

func TestAskAssistant_ErrorCases_HaveOwnStatusMessageAndCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		model       assistant.Model
		request     any
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name: "not configured", model: nil, request: question("Hi"),
			wantStatus: http.StatusServiceUnavailable, wantCode: "not_configured",
			wantMessage: "the Assistant is not configured",
		},
		{
			name: "request not valid", model: newFakeModel(), request: questionRequest{},
			wantStatus: http.StatusBadRequest, wantCode: "invalid_request",
			wantMessage: "the request has no messages",
		},
		{
			name: "provider failure", model: failingModel(fmt.Errorf("status 500: %w", assistant.ErrProviderFailure)),
			request: question("Hi"), wantStatus: http.StatusBadGateway, wantCode: "provider_failure",
			wantMessage: "the provider of the Assistant does not answer",
		},
		{
			name: "rate limit", model: failingModel(fmt.Errorf("status 429: %w", assistant.ErrRateLimited)),
			request: question("Hi"), wantStatus: http.StatusTooManyRequests, wantCode: "rate_limited",
			wantMessage: "the provider of the Assistant refuses the question because of a rate limit. " +
				"The daily limit of the free model can be the cause. Try again later.",
		},
		{
			name: "time limit", model: hangingModel{}, request: question("Hi"),
			wantStatus: http.StatusBadGateway, wantCode: "provider_failure",
			wantMessage: "the provider of the Assistant does not answer",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A test never waits for the real time limit.
			srv := startServerWithOptions(t, serverOptions{model: tc.model, questionTimeout: 50 * time.Millisecond})

			status, body := srv.postJSON(t, "/api/assistant/questions", tc.request)

			require.Equal(t, tc.wantStatus, status, "body %q", body)
			var got errorWithCode
			decode(t, body, &got)
			assert.Equal(t, tc.wantMessage, got.Error)
			assert.Equal(t, tc.wantCode, got.Code)
		})
	}
}

// signalingModel is an assistant.Model that tells when it has got a call, and then does not answer until its
// context ends.
type signalingModel struct{ started chan struct{} }

// Reply sends on started, blocks until the context ends, and returns the error of the context.
func (m signalingModel) Reply(ctx context.Context, _ []assistant.Message, _ []assistant.FunctionDefinition) (assistant.Reply, error) {
	m.started <- struct{}{}
	<-ctx.Done()
	return assistant.Reply{}, fmt.Errorf("wait for the provider: %w", ctx.Err())
}

func TestAskAssistant_ClientCancels_LogsNoErrorAndNoWarning(t *testing.T) {
	t.Parallel()
	model := signalingModel{started: make(chan struct{}, 1)}
	logs := &syncBuffer{}
	srv := startServerWithOptions(t, serverOptions{model: model, logs: logs})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		body, err := json.Marshal(question("What are my Holdings?"))
		if err != nil {
			done <- err
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/api/assistant/questions", bytes.NewReader(body))
		if err != nil {
			done <- err
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := srv.client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		done <- err
	}()
	<-model.started

	cancel() // The user closes the panel while the model works.

	require.ErrorIs(t, <-done, context.Canceled)
	// The handler logs the cancel at the level INFO. Wait for that line: the handler has logged all it logs.
	require.Eventually(t, func() bool { return strings.Contains(logs.String(), "the client cancelled the question") },
		5*time.Second, 5*time.Millisecond, "logs: %s", logs.String())
	assert.NotContains(t, logs.String(), "level=ERROR")
	assert.NotContains(t, logs.String(), "level=WARN")
}

// failingModel returns a model that fails each call with err.
func failingModel(err error) *fakeModel {
	m := newFakeModel()
	m.failure = err
	return m
}
