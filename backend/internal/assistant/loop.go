package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// MaxRounds is the highest number of model calls for one question. After the last round, the Assistant
// says that it cannot answer.
const MaxRounds = 5

// MaxStepsPerRound is the highest number of steps that the loop runs for one reply of the model. A weak model can
// send many steps in one reply, and each step can need much calculation. The steps over the limit get an error
// result that tells the model to ask for fewer steps.
const MaxStepsPerRound = 8

// CannotAnswer is the answer text when the loop gets no text answer from the model in MaxRounds rounds.
const CannotAnswer = "I cannot answer this question with the data of the tool."

// ErrModel tells that the model did not answer. Ask wraps each error of the model with it.
var ErrModel = errors.New("the model does not answer")

// Function is one read-only function that the model can call. ADR 0007: the model reads data only through
// the functions.
type Function struct {
	Definition FunctionDefinition
	// Run returns the result for the arguments. The model gets the result as JSON text.
	// For a problem with the arguments, Run returns an ArgumentError. The model then gets an error result,
	// and the loop continues. Each other error stops the loop.
	Run func(ctx context.Context, arguments json.RawMessage) (any, error)
}

// ArgumentError is a problem with the arguments of a step, for example an unknown Account.
// The model gets its message as an error result, so that it can correct the arguments.
type ArgumentError struct {
	Message string
}

// Error returns the message for the model.
func (e *ArgumentError) Error() string {
	return e.Message
}

// Argumentf returns an ArgumentError with the formatted message.
func Argumentf(format string, a ...any) error {
	return &ArgumentError{Message: fmt.Sprintf(format, a...)}
}

// DecodeArguments decodes the arguments of a step into v. A field that v does not have is an error.
// Each error is an ArgumentError.
func DecodeArguments(arguments json.RawMessage, v any) error {
	dec := json.NewDecoder(bytes.NewReader(arguments))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Argumentf("the arguments are not valid: %v", err)
	}
	return nil
}

// errorResult is the result that tells the model what is wrong with a step.
type errorResult struct {
	Error string `json:"error"`
}

// Answer is the answer of the Assistant to one question.
type Answer struct {
	// Text is the answer text for the user.
	Text string
	// Steps are the steps that the loop ran, in run order.
	Steps []Step
}

// Ask sends the conversation and the function definitions to the model. It runs each step of the model and
// sends the results back, until the model answers with text or the loop reaches MaxRounds.
// messages are the instructions and the conversation. The last message is the new question.
func Ask(ctx context.Context, model Model, messages []Message, functions []Function) (Answer, error) {
	definitions := make([]FunctionDefinition, 0, len(functions))
	byName := make(map[string]Function, len(functions))
	for _, f := range functions {
		definitions = append(definitions, f.Definition)
		byName[f.Definition.Name] = f
	}

	conversation := slices.Clone(messages)
	steps := []Step{}
	for round := 1; round <= MaxRounds; round++ {
		reply, err := model.Reply(ctx, conversation, definitions)
		if err != nil {
			return Answer{}, fmt.Errorf("round %d: %w: %w", round, ErrModel, err)
		}
		if len(reply.Steps) == 0 {
			if strings.TrimSpace(reply.Text) == "" {
				return Answer{Text: CannotAnswer, Steps: steps}, nil
			}
			return Answer{Text: reply.Text, Steps: steps}, nil
		}
		// The steps of the last round do not run, because no round is left to describe their results.
		if round == MaxRounds {
			break
		}
		conversation = append(conversation, Message{Role: RoleAssistant, Content: reply.Text, Steps: reply.Steps})
		for i, s := range reply.Steps {
			// A function does not check the context in all of its calculation. Thus the loop checks it here.
			if err := ctx.Err(); err != nil {
				return Answer{}, fmt.Errorf("stop before step %d of round %d: %w", i+1, round, err)
			}
			if i >= MaxStepsPerRound {
				// The provider needs a result for each tool call, so the step gets an error result.
				result, err := jsonResult(errorResult{Error: fmt.Sprintf(
					"the step is not run. Run at most %d steps in one turn. Ask for fewer steps, or ask for the rest in the next turn.",
					MaxStepsPerRound)})
				if err != nil {
					return Answer{}, err
				}
				// The answer lists the step with its error result, so that the user sees each step of the model.
				s.Result = result
				steps = append(steps, s)
				conversation = append(conversation, Message{Role: RoleFunction, StepID: s.ID, Content: string(result)})
				continue
			}
			result, err := run(ctx, byName, s)
			if err != nil {
				return Answer{}, err
			}
			s.Result = result
			steps = append(steps, s)
			conversation = append(conversation, Message{Role: RoleFunction, StepID: s.ID, Content: string(result)})
		}
	}
	return Answer{Text: CannotAnswer, Steps: steps}, nil
}

// run runs the step and returns its result as JSON. A problem with the step gives an error result.
func run(ctx context.Context, byName map[string]Function, s Step) ([]byte, error) {
	f, ok := byName[s.Function]
	if !ok {
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		slices.Sort(names)
		return jsonResult(errorResult{Error: fmt.Sprintf("unknown function %q. The functions are: %s.",
			s.Function, strings.Join(names, ", "))})
	}
	arguments := bytes.TrimSpace(s.Arguments)
	if len(arguments) == 0 {
		arguments = []byte("{}")
	}
	if !json.Valid(arguments) || arguments[0] != '{' {
		return jsonResult(errorResult{Error: "the arguments are not a valid JSON object"})
	}

	result, err := f.Run(ctx, arguments)
	if argErr, ok := errors.AsType[*ArgumentError](err); ok {
		return jsonResult(errorResult{Error: argErr.Message})
	}
	if err != nil {
		return nil, fmt.Errorf("run function %s: %w", s.Function, err)
	}
	return jsonResult(result)
}

// jsonResult returns the result as JSON text.
func jsonResult(result any) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode function result: %w", err)
	}
	return data, nil
}
