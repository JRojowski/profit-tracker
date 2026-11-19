// Package openrouter is an assistant.Model that uses the chat completions endpoint of OpenRouter.
// The endpoint uses the OpenAI format. All knowledge of OpenRouter stays in this package.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

// DefaultBaseURL is the address of the OpenRouter API.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// DefaultModel is the model ID when the configuration gives no model. It is a free model that supports
// function calls. Selected from the model list of OpenRouter on 2026-11-19. The list of free models changes.
const DefaultModel = "nvidia/nemotron-3-ultra-550b-a55b:free"

// maxBody is the largest response body that the client reads.
const maxBody = 4 << 20

// maxErrorText is the largest part of an error body that goes into an error.
const maxErrorText = 300

// Client sends the conversation of the Assistant to a model of OpenRouter.
type Client struct {
	// HTTPClient sends the requests. It must have a timeout.
	HTTPClient *http.Client
	// BaseURL is the address of the API, with no final slash.
	BaseURL string
	// Key is the API key. The client never puts it in an error.
	Key string
	// Model is the model ID, for example DefaultModel.
	Model string
}

// New returns a Client with an HTTP client that stops a request after 90 seconds.
// A free model can need some tens of seconds for one answer.
func New(baseURL, key, model string) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 90 * time.Second}, BaseURL: strings.TrimRight(baseURL, "/"), Key: key, Model: model,
	}
}

// chatRequest is the body of a chat completions request.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []tool        `json:"tools,omitempty"`
}

// chatMessage is one message in the OpenAI format.
type chatMessage struct {
	Role string `json:"role"`
	// Content is nil for an assistant message with tool calls and no text.
	Content    *string    `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// toolCall is one function call of the model.
type toolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

// functionCall is the function and the arguments of a tool call.
type functionCall struct {
	Name string `json:"name"`
	// Arguments is JSON text in a JSON string. Some providers send a JSON object. Thus the type is raw JSON.
	Arguments json.RawMessage `json:"arguments"`
}

// tool is one function definition in the OpenAI format.
type tool struct {
	Type     string             `json:"type"`
	Function functionDefinition `json:"function"`
}

// functionDefinition is the description of one function.
type functionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// chatResponse is the part of the chat completions response that the client reads.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
		// Error is set when the provider fails after it started to answer. The status stays 200.
		Error *providerError `json:"error"`
	} `json:"choices"`
	Error *providerError `json:"error"`
}

// providerError is the error object of OpenRouter.
type providerError struct {
	Code     json.RawMessage `json:"code"`
	Message  string          `json:"message"`
	Metadata struct {
		ErrorType string `json:"error_type"`
	} `json:"metadata"`
}

// rateLimited tells if the error is a rate limit. The code is a number, but a provider can send text.
func (e providerError) rateLimited() bool {
	return strings.TrimSpace(string(e.Code)) == strconv.Itoa(http.StatusTooManyRequests) ||
		e.Metadata.ErrorType == "rate_limit_exceeded"
}

// Reply sends the messages and the function definitions to the model and returns its answer.
func (c *Client) Reply(ctx context.Context, messages []assistant.Message, functions []assistant.FunctionDefinition) (assistant.Reply, error) {
	reply, err := c.reply(ctx, messages, functions)
	if err != nil {
		return assistant.Reply{}, fmt.Errorf("openrouter model %s: %w", c.Model, err)
	}
	return reply, nil
}

func (c *Client) reply(ctx context.Context, messages []assistant.Message, functions []assistant.FunctionDefinition) (assistant.Reply, error) {
	body, err := json.Marshal(c.request(messages, functions))
	if err != nil {
		return assistant.Reply{}, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return assistant.Reply{}, fmt.Errorf("make request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// The error of the HTTP client holds the method and the URL, not the headers. Thus it holds no key.
		return assistant.Reply{}, fmt.Errorf("send request: %w: %w", assistant.ErrProviderFailure, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return assistant.Reply{}, fmt.Errorf("read response: %w: %w", assistant.ErrProviderFailure, err)
	}
	if resp.StatusCode != http.StatusOK {
		return assistant.Reply{}, c.statusError(resp.StatusCode, data)
	}

	var chat chatResponse
	if err := json.Unmarshal(data, &chat); err != nil {
		return assistant.Reply{}, fmt.Errorf("decode response: %w: %w", assistant.ErrProviderFailure, err)
	}
	// The provider can send an error in a response with the status 200.
	if chat.Error != nil {
		return assistant.Reply{}, c.bodyError(*chat.Error)
	}
	if len(chat.Choices) == 0 {
		return assistant.Reply{}, fmt.Errorf("the response has no choice: %w", assistant.ErrProviderFailure)
	}
	if chat.Choices[0].Error != nil {
		return assistant.Reply{}, c.bodyError(*chat.Choices[0].Error)
	}
	message := chat.Choices[0].Message
	reply := assistant.Reply{Text: message.Content}
	for i, call := range message.ToolCalls {
		id := call.ID
		if id == "" {
			id = "step-" + strconv.Itoa(i+1)
		}
		reply.Steps = append(reply.Steps, assistant.Step{ID: id, Function: call.Function.Name, Arguments: arguments(call.Function.Arguments)})
	}
	return reply, nil
}

// request returns the body of the chat completions request.
func (c *Client) request(messages []assistant.Message, functions []assistant.FunctionDefinition) chatRequest {
	req := chatRequest{Model: c.Model, Messages: make([]chatMessage, 0, len(messages))}
	for _, m := range messages {
		req.Messages = append(req.Messages, toChatMessage(m))
	}
	for _, f := range functions {
		req.Tools = append(req.Tools, tool{
			Type:     "function",
			Function: functionDefinition{Name: f.Name, Description: f.Description, Parameters: f.Parameters},
		})
	}
	return req
}

// toChatMessage returns the message in the OpenAI format.
func toChatMessage(m assistant.Message) chatMessage {
	switch m.Role {
	case assistant.RoleFunction:
		return chatMessage{Role: "tool", Content: &m.Content, ToolCallID: m.StepID}
	case assistant.RoleAssistant:
		msg := chatMessage{Role: "assistant"}
		if m.Content != "" || len(m.Steps) == 0 {
			msg.Content = &m.Content
		}
		for _, s := range m.Steps {
			// The OpenAI format sends the arguments as JSON text in a JSON string.
			text, err := json.Marshal(string(s.Arguments))
			if err != nil {
				text = []byte(`"{}"`)
			}
			msg.ToolCalls = append(msg.ToolCalls, toolCall{
				ID: s.ID, Type: "function", Function: functionCall{Name: s.Function, Arguments: text},
			})
		}
		return msg
	default:
		return chatMessage{Role: string(m.Role), Content: &m.Content}
	}
}

// arguments returns the arguments of a tool call as JSON text. The OpenAI format sends a JSON string that
// holds the JSON text. A JSON object stays as it is.
func arguments(raw json.RawMessage) json.RawMessage {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return json.RawMessage(text)
	}
	return raw
}

// redact returns the text with each copy of the API key replaced. A provider can repeat the key in an error,
// and an error goes into the log.
func (c *Client) redact(text []byte) []byte {
	if c.Key == "" {
		return text
	}
	return bytes.ReplaceAll(text, []byte(c.Key), []byte("[API key]"))
}

// bodyError returns the error for an error object of the provider in a response with the status 200.
func (c *Client) bodyError(e providerError) error {
	kind := assistant.ErrProviderFailure
	if e.rateLimited() {
		kind = assistant.ErrRateLimited
	}
	return fmt.Errorf("provider error %s: %s: %w", e.Code, c.redact([]byte(e.Message)), kind)
}

// statusError returns the error for a response with a status that is not 200. The status 429 is a rate limit.
// Each other status is a failure of the provider. The error holds the message of the provider if the body has
// one, or else the start of the body.
func (c *Client) statusError(status int, body []byte) error {
	kind := assistant.ErrProviderFailure
	if status == http.StatusTooManyRequests {
		kind = assistant.ErrRateLimited
	}
	body = c.redact(body)
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return fmt.Errorf("status %d: %s: %w", status, parsed.Error.Message, kind)
	}
	text := strings.TrimSpace(string(body))
	if len(text) > maxErrorText {
		text = text[:maxErrorText]
	}
	return fmt.Errorf("status %d: %s: %w", status, text, kind)
}
