package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
)

// availabilityResponse is the body of GET /api/assistant.
type availabilityResponse struct {
	Available bool `json:"available"`
}

// questionMessage is one message of the conversation in a question request.
type questionMessage struct {
	// Role is "user" or "assistant".
	Role    string `json:"role"`
	Content string `json:"content"`
}

// questionRequest is the body of POST /api/assistant/questions.
type questionRequest struct {
	// Messages are the earlier questions and answers and the new question, in time order.
	// The last message is the new question.
	Messages []questionMessage `json:"messages"`
	// Context is the page of the user. It and each of its fields are optional.
	Context *pageContext `json:"context"`
}

// pageContext is the page that the user has open when the user asks.
type pageContext struct {
	// AccountID is the Account of the page.
	AccountID *int64 `json:"accountId"`
	// Instrument is the broker ticker of the open Instrument Overview.
	Instrument *string `json:"instrument"`
}

// maxQuestionBody is the largest body of a question request. A long conversation is some kilobytes.
const maxQuestionBody = 1 << 20

// stepResponse is one step of an answer: the function and its arguments.
type stepResponse struct {
	Function  string          `json:"function"`
	Arguments json.RawMessage `json:"arguments"`
}

// answerResponse is the body of the response of POST /api/assistant/questions.
type answerResponse struct {
	Answer string         `json:"answer"`
	Steps  []stepResponse `json:"steps"`
}

// assistantAvailability tells the frontend if the Assistant is available.
func (a *api) assistantAvailability(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, availabilityResponse{Available: a.model != nil})
}

// askAssistant answers one question of the user.
func (a *api) askAssistant(w http.ResponseWriter, r *http.Request) {
	if a.model == nil {
		writeError(w, http.StatusServiceUnavailable, "the Assistant is not configured")
		return
	}
	var req questionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxQuestionBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	instructions, err := a.assistantInstructions(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	messages := make([]assistant.Message, 0, len(req.Messages)+1)
	messages = append(messages, assistant.Message{Role: assistant.RoleSystem, Content: instructions})
	for _, m := range req.Messages {
		messages = append(messages, assistant.Message{Role: assistant.Role(m.Role), Content: m.Content})
	}

	answer, err := assistant.Ask(r.Context(), a.model, messages, a.assistantFunctions())
	if errors.Is(err, assistant.ErrModel) {
		a.logger.ErrorContext(r.Context(), "the provider of the Assistant does not answer", "error", err)
		writeError(w, http.StatusBadGateway, "the provider of the Assistant does not answer")
		return
	}
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	steps := make([]stepResponse, 0, len(answer.Steps))
	for _, s := range answer.Steps {
		steps = append(steps, stepResponse{Function: s.Function, Arguments: stepArguments(s.Arguments)})
	}
	writeJSON(w, http.StatusOK, answerResponse{Answer: answer.Text, Steps: steps})
}

// validate returns an error if the request is not valid.
func (q questionRequest) validate() error {
	if len(q.Messages) == 0 {
		return errors.New("the request has no messages")
	}
	for i, m := range q.Messages {
		if m.Role != string(assistant.RoleUser) && m.Role != string(assistant.RoleAssistant) {
			return fmt.Errorf("message %d has the role %q: use \"user\" or \"assistant\"", i+1, m.Role)
		}
		if strings.TrimSpace(m.Content) == "" {
			return fmt.Errorf("message %d has no content", i+1)
		}
	}
	if q.Messages[len(q.Messages)-1].Role != string(assistant.RoleUser) {
		return errors.New("the last message is not a question: give it the role \"user\"")
	}
	return nil
}

// assistantInstructions returns the instructions for the model, with the date of today in Polish time and
// the Accounts. The Accounts have no broker account number.
func (a *api) assistantInstructions(ctx context.Context) (string, error) {
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		return "", err
	}
	list := make([]assistant.Account, 0, len(accounts))
	for _, acc := range accounts {
		list = append(list, assistant.Account{ID: acc.ID, Name: acc.Name, Type: acc.Type, Currency: acc.Currency})
	}
	return assistant.Instructions(exchange.StartOfDayInWarsaw(a.now()), list), nil
}

// stepArguments returns the arguments of a step for the response. Arguments that are not a JSON object
// become an empty object, so that the response is always valid JSON.
func stepArguments(arguments json.RawMessage) json.RawMessage {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &object); err != nil || object == nil {
		return json.RawMessage("{}")
	}
	return arguments
}
