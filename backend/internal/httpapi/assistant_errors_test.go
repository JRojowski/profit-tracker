package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
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

// failingModel returns a model that fails each call with err.
func failingModel(err error) *fakeModel {
	m := newFakeModel()
	m.failure = err
	return m
}
