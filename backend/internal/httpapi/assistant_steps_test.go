package httpapi_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

func TestAskAssistant_TooManyStepsInOneTurn_RunsTheFirstEightAndTellsTheModelToAskForFewer(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	var steps []assistant.Step
	for i := 1; i <= 10; i++ {
		steps = append(steps, assistant.Step{ID: fmt.Sprintf("call-%d", i), Function: "get_holdings", Arguments: json.RawMessage(`{}`)})
	}
	model.setScript(assistant.Reply{Steps: steps}, assistant.Reply{Text: "Here is the answer."})

	answer := srv.ask(t, question("What are my Holdings?"))

	assert.Equal(t, "Here is the answer.", answer.Answer)
	assert.Len(t, answer.Steps, 8, "the steps over the cap do not run, so the answer does not list them")
	results := model.functionResults(t)
	require.Len(t, results, 10, "each step has a result, because the provider needs one for each tool call")
	for i, r := range results {
		assert.Equal(t, fmt.Sprintf("call-%d", i+1), r.StepID)
		var got struct {
			Error string `json:"error"`
		}
		decode(t, r.Content, &got)
		if i < 8 {
			assert.Empty(t, got.Error, "step %d runs", i+1)
			continue
		}
		assert.Contains(t, got.Error, "at most 8 steps", "step %d", i+1)
		assert.Contains(t, got.Error, "fewer", "step %d", i+1)
	}
}

func TestAskAssistant_EightStepsInOneTurn_AllRun(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	var steps []assistant.Step
	for i := 1; i <= 8; i++ {
		steps = append(steps, assistant.Step{ID: fmt.Sprintf("call-%d", i), Function: "get_holdings", Arguments: json.RawMessage(`{}`)})
	}
	model.setScript(assistant.Reply{Steps: steps}, assistant.Reply{Text: "Here is the answer."})

	answer := srv.ask(t, question("What are my Holdings?"))

	assert.Len(t, answer.Steps, 8)
}
