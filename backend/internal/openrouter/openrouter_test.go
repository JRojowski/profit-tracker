package openrouter_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/openrouter"
)

// testKey is the API key of the tests. It must never appear in an error.
const testKey = "sk-or-v1-test-key"

// request is one request that the test server got.
type request struct {
	method, path, authorization, contentType string
	body                                     []byte
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

// startOpenRouter starts a server that answers each request with the status and the body.
// It returns a client for the server and the recorder of the requests.
func startOpenRouter(t *testing.T, status int, body string) (*openrouter.Client, *recorder) {
	t.Helper()
	got := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		got.add(request{
			method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization"),
			contentType: r.Header.Get("Content-Type"), body: data,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &openrouter.Client{HTTPClient: srv.Client(), BaseURL: srv.URL, Key: testKey, Model: "vendor/model:free"}, got
}

// textBody is a response with a text answer.
const textBody = `{"id":"gen-1","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"You have 2 Holdings."}}]}`

func TestReply_SendsTheMessagesAndTheFunctionDefinitions(t *testing.T) {
	t.Parallel()
	client, requests := startOpenRouter(t, http.StatusOK, textBody)
	messages := []assistant.Message{
		{Role: assistant.RoleSystem, Content: "The rules."},
		{Role: assistant.RoleUser, Content: "What are my Holdings?"},
		{Role: assistant.RoleAssistant, Steps: []assistant.Step{
			{ID: "call-1", Function: "get_holdings", Arguments: json.RawMessage(`{"accountId":1}`)},
		}},
		{Role: assistant.RoleFunction, StepID: "call-1", Content: `{"holdings":[]}`},
	}
	functions := []assistant.FunctionDefinition{{
		Name: "get_holdings", Description: "Returns the Holdings.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"accountId":{"type":"integer"}}}`),
	}}

	_, err := client.Reply(t.Context(), messages, functions)

	require.NoError(t, err)
	all := requests.all()
	require.Len(t, all, 1)
	got := all[0]
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/chat/completions", got.path)
	assert.Equal(t, "Bearer "+testKey, got.authorization)
	assert.Equal(t, "application/json", got.contentType)
	assert.JSONEq(t, `{
		"model": "vendor/model:free",
		"messages": [
			{"role": "system", "content": "The rules."},
			{"role": "user", "content": "What are my Holdings?"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call-1", "type": "function", "function": {"name": "get_holdings", "arguments": "{\"accountId\":1}"}}
			]},
			{"role": "tool", "tool_call_id": "call-1", "content": "{\"holdings\":[]}"}
		],
		"tools": [
			{"type": "function", "function": {
				"name": "get_holdings",
				"description": "Returns the Holdings.",
				"parameters": {"type": "object", "properties": {"accountId": {"type": "integer"}}}
			}}
		]
	}`, string(got.body))
}

func TestReply_TextAnswer_ReturnsTheText(t *testing.T) {
	t.Parallel()
	client, _ := startOpenRouter(t, http.StatusOK, textBody)

	got, err := client.Reply(t.Context(), []assistant.Message{{Role: assistant.RoleUser, Content: "Hi"}}, nil)

	require.NoError(t, err)
	assert.Equal(t, assistant.Reply{Text: "You have 2 Holdings."}, got)
}

func TestReply_ToolCalls_ReturnsTheSteps(t *testing.T) {
	t.Parallel()
	client, _ := startOpenRouter(t, http.StatusOK, `{"choices":[{"finish_reason":"tool_calls","message":{
		"role":"assistant","content":"",
		"tool_calls":[
			{"id":"call-1","type":"function","function":{"name":"get_holdings","arguments":"{\"accountId\": 1}"}},
			{"type":"function","function":{"name":"get_holdings","arguments":""}}
		]}}]}`)

	got, err := client.Reply(t.Context(), []assistant.Message{{Role: assistant.RoleUser, Content: "Hi"}}, nil)

	require.NoError(t, err)
	assert.Equal(t, assistant.Reply{Steps: []assistant.Step{
		{ID: "call-1", Function: "get_holdings", Arguments: json.RawMessage(`{"accountId": 1}`)},
		// A step with no ID gets an ID from its position, because the result must refer to it.
		{ID: "step-2", Function: "get_holdings", Arguments: json.RawMessage(``)},
	}}, got)
}

func TestReply_BadResponses_ReturnError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
		// want is a part of the error text.
		want string
	}{
		{
			name: "error status with the error of the provider", status: http.StatusUnauthorized,
			body: `{"error":{"code":401,"message":"No auth credentials found"}}`, want: "401",
		},
		{name: "error status with no JSON", status: http.StatusBadGateway, body: "<html>bad gateway</html>", want: "502"},
		{name: "error in a 200 response", status: http.StatusOK, body: `{"error":{"code":502,"message":"Provider returned error"}}`, want: "Provider returned error"},
		{name: "no choices", status: http.StatusOK, body: `{"choices":[]}`, want: "no choice"},
		{name: "not JSON", status: http.StatusOK, body: "not json", want: "decode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, _ := startOpenRouter(t, tc.status, tc.body)

			_, err := client.Reply(t.Context(), []assistant.Message{{Role: assistant.RoleUser, Content: "Hi"}}, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), testKey)
		})
	}
}
