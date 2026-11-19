package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
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

// DefaultAssistantTimeout is the time limit of one question to the Assistant. A free model can need some tens
// of seconds for one model call, and a question can need some calls.
const DefaultAssistantTimeout = 2 * time.Minute

// The codes of the errors of the Assistant. The frontend shows one message for each code.
const (
	codeNotConfigured  = "not_configured"
	codeInvalidRequest = "invalid_request"
	codeProviderFailed = "provider_failure"
	codeRateLimited    = "rate_limited"
)

// maxMessages is the largest number of messages in a question request. The backend keeps no conversation,
// so the limit protects the model and the cost.
const maxMessages = 40

// maxQuestionLength is the largest length of a message of the user, in characters.
const maxQuestionLength = 4000

// maxAnswerLength is the largest length of an earlier answer of the Assistant, in characters. The frontend sends
// the answers back with the next question, and an answer can be a long list. Thus the limit is larger than
// maxQuestionLength.
const maxAnswerLength = 20000

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

// assistantOriginGuard returns the handler that refuses a request from another origin before the handler runs.
// A request that costs money must come from the page of the tool. The guard allows an Origin that is equal to
// the Host. Thus it allows the Vite dev server, which forwards the Host of the page.
func assistantOriginGuard(next http.Handler) http.Handler {
	guard := http.NewCrossOriginProtection()
	guard.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeCodedError(w, http.StatusForbidden, codeInvalidRequest, "the request comes from another origin")
	}))
	return guard.Handler(next)
}

// assistantAvailability tells the frontend if the Assistant is available.
func (a *api) assistantAvailability(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, availabilityResponse{Available: a.model != nil})
}

// askAssistant answers one question of the user.
func (a *api) askAssistant(w http.ResponseWriter, r *http.Request) {
	if a.model == nil {
		writeCodedError(w, http.StatusServiceUnavailable, codeNotConfigured, "the Assistant is not configured")
		return
	}
	// A page of another site can send a "simple" request with the type text/plain and no preflight.
	// The type check refuses it, so a hostile page cannot spend the quota of the provider.
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		writeCodedError(w, http.StatusUnsupportedMediaType, codeInvalidRequest, "the content type must be application/json")
		return
	}
	var req questionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxQuestionBody)).Decode(&req); err != nil {
		writeCodedError(w, http.StatusBadRequest, codeInvalidRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	if err := req.validate(); err != nil {
		writeCodedError(w, http.StatusBadRequest, codeInvalidRequest, err.Error())
		return
	}
	page, problem, err := a.assistantPage(r.Context(), req.Context)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	if problem != "" {
		writeCodedError(w, http.StatusBadRequest, codeInvalidRequest, problem)
		return
	}
	instructions, err := a.assistantInstructions(r.Context(), page)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	messages := make([]assistant.Message, 0, len(req.Messages)+1)
	messages = append(messages, assistant.Message{Role: assistant.RoleSystem, Content: instructions})
	for _, m := range req.Messages {
		messages = append(messages, assistant.Message{Role: assistant.Role(m.Role), Content: m.Content})
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.questionTimeout)
	defer cancel()
	answer, err := assistant.Ask(ctx, a.model, messages, a.assistantFunctions())
	if err != nil && a.writeModelError(ctx, w, err) {
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

// writeModelError writes the error response if err comes from the provider of the model or from the time limit.
// It returns false for another error, and writes nothing then.
func (a *api) writeModelError(ctx context.Context, w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, assistant.ErrRateLimited):
		a.logger.WarnContext(ctx, "the provider of the Assistant refuses the question", "error", err)
		writeCodedError(w, http.StatusTooManyRequests, codeRateLimited,
			"the provider of the Assistant refuses the question because of a rate limit. "+
				"The daily limit of the free model can be the cause. Try again later.")
	case errors.Is(err, assistant.ErrModel), errors.Is(ctx.Err(), context.DeadlineExceeded):
		a.logger.ErrorContext(ctx, "the provider of the Assistant does not answer", "error", err)
		writeCodedError(w, http.StatusBadGateway, codeProviderFailed, "the provider of the Assistant does not answer")
	default:
		return false
	}
	return true
}

// validate returns an error if the request is not valid.
func (q questionRequest) validate() error {
	if len(q.Messages) == 0 {
		return errors.New("the request has no messages")
	}
	if len(q.Messages) > maxMessages {
		return fmt.Errorf("the request has %d messages: the limit is %d", len(q.Messages), maxMessages)
	}
	for i, m := range q.Messages {
		if m.Role != string(assistant.RoleUser) && m.Role != string(assistant.RoleAssistant) {
			return fmt.Errorf("message %d has the role %q: use \"user\" or \"assistant\"", i+1, m.Role)
		}
		if strings.TrimSpace(m.Content) == "" {
			return fmt.Errorf("message %d has no content", i+1)
		}
		limit := maxQuestionLength
		if m.Role == string(assistant.RoleAssistant) {
			limit = maxAnswerLength
		}
		if n := utf8.RuneCountInString(m.Content); n > limit {
			return fmt.Errorf("message %d has %d characters: the limit is %d", i+1, n, limit)
		}
	}
	if q.Messages[len(q.Messages)-1].Role != string(assistant.RoleUser) {
		return errors.New("the last message is not a question: give it the role \"user\"")
	}
	return nil
}

// assistantInstructions returns the instructions for the model, with the date of today in Polish time and
// the Accounts. The Accounts have no broker account number.
func (a *api) assistantInstructions(ctx context.Context, page *assistant.Page) (string, error) {
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		return "", err
	}
	list := make([]assistant.Account, 0, len(accounts))
	for _, acc := range accounts {
		list = append(list, assistant.Account{ID: acc.ID, Name: acc.Name, Type: acc.Type, Currency: acc.Currency})
	}
	return assistant.Instructions(exchange.StartOfDayInWarsaw(a.now()), list, page), nil
}

// assistantPage resolves the page context of the request. It returns nil if the request has no page context.
// An unknown Account gives a problem text for the 400 error. An unknown Instrument is left out. The ticker has the case of the
// Instrument in the tool, so that the instructions never hold text that the user typed.
func (a *api) assistantPage(ctx context.Context, c *pageContext) (*assistant.Page, string, error) {
	if c == nil {
		return nil, "", nil
	}
	var page assistant.Page
	if c.AccountID != nil {
		acc, err := a.store.Account(ctx, *c.AccountID)
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Sprintf("the context has an unknown Account %d", *c.AccountID), nil
		}
		if err != nil {
			return nil, "", err
		}
		page.Account = &assistant.Account{ID: acc.ID, Name: acc.Name, Type: acc.Type, Currency: acc.Currency}
	}
	if c.Instrument != nil && *c.Instrument != "" {
		ticker, err := a.assistantInstrument(ctx, *c.Instrument)
		var argErr *assistant.ArgumentError
		// An old link can hold a ticker that the tool does not know. The page then has no Instrument.
		if err != nil && !errors.As(err, &argErr) {
			return nil, "", err
		}
		page.Instrument = ticker
	}
	return &page, "", nil
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
