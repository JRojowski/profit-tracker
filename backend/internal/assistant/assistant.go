// Package assistant is the conversation loop of the Assistant. The Assistant sends the conversation and the
// function definitions to a language model. The model answers with text, or with steps: calls of the functions.
// The loop runs each step and sends the results back to the model. ADR 0007: the model reads data only through
// the functions and does not calculate.
//
// The package has no HTTP code and no knowledge of a model provider.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
)

// Role is the role of a message in the conversation.
type Role string

// The roles of the messages.
const (
	// RoleSystem is the role of the instructions for the model.
	RoleSystem Role = "system"
	// RoleUser is the role of a question of the user.
	RoleUser Role = "user"
	// RoleAssistant is the role of an answer or of the steps of the model.
	RoleAssistant Role = "assistant"
	// RoleFunction is the role of the result of one step.
	RoleFunction Role = "function"
)

// Message is one message of the conversation.
type Message struct {
	Role Role
	// Content is the text of the message. For RoleFunction, it is the result of the step as JSON text.
	Content string
	// Steps are the steps of a RoleAssistant message. They are empty for a text answer.
	Steps []Step
	// StepID is the ID of the step whose result a RoleFunction message holds.
	StepID string
}

// Step is one call of a function by the model.
type Step struct {
	// ID identifies the step in the conversation. The result of the step refers to it.
	ID string
	// Function is the name of the function.
	Function string
	// Arguments are the arguments as the model gave them. They can be JSON that is not valid.
	Arguments json.RawMessage
	// Result is the JSON that the function gave to the model. Only the loop sets it, in Answer.Steps.
	Result json.RawMessage
}

// FunctionDefinition is the description of one function for the model.
type FunctionDefinition struct {
	Name        string
	Description string
	// Parameters is the JSON Schema of the arguments.
	Parameters json.RawMessage
}

// Reply is one answer of the model: text, or one or more steps.
type Reply struct {
	Text  string
	Steps []Step
}

// Model is a language model that answers a conversation. It can use the functions.
type Model interface {
	// Reply sends the messages and the function definitions to the model and returns its answer.
	Reply(ctx context.Context, messages []Message, functions []FunctionDefinition) (Reply, error)
}

// ErrRateLimited tells that the provider of the model refuses the question because of a rate limit.
// For a free model, the daily limit can be the cause.
var ErrRateLimited = errors.New("the provider refuses the request because of a rate limit")

// ErrProviderFailure tells that the provider of the model fails: it returns an error, or a response that
// the client cannot use.
var ErrProviderFailure = errors.New("the provider fails")
