package httpapi_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/openrouter"
)

// syncBuffer is a buffer for the log lines of the server. It is safe for concurrent use.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestAskAssistant_APIKey_IsInNoResponseAndInNoLogLine uses the real OpenRouter client against a local fake
// provider. The fake provider answers the first question and fails the second. Its error repeats the
// Authorization header, so that the key goes into the error of the client.
func TestAskAssistant_APIKey_IsInNoResponseAndInNoLogLine(t *testing.T) {
	t.Parallel()
	const key = "sk-or-v1-0123456789abcdef"
	var (
		mu    sync.Mutex
		calls int
	)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"You have no Holdings."}}]}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"the key ` + r.Header.Get("Authorization") + ` is not valid"}}`))
	}))
	t.Cleanup(provider.Close)
	logs := &syncBuffer{}
	srv := startServerWithOptions(t, serverOptions{model: openrouter.New(provider.URL, key, openrouter.DefaultModel), logs: logs})

	var bodies []string
	_, _, availability := srv.get(t, "/api/assistant")
	bodies = append(bodies, availability)
	status, answer := srv.postJSON(t, "/api/assistant/questions", question("What are my Holdings?"))
	require.Equal(t, http.StatusOK, status, "body %q", answer)
	bodies = append(bodies, answer)
	status, failure := srv.postJSON(t, "/api/assistant/questions", question("What are my Holdings?"))
	require.Equal(t, http.StatusBadGateway, status, "body %q", failure)
	bodies = append(bodies, failure)

	for _, body := range bodies {
		assert.NotContains(t, body, key)
	}
	require.Contains(t, logs.String(), "is not valid", "the log must hold the error of the provider")
	assert.NotContains(t, logs.String(), key)
}
