package assistant_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

// scriptedModel answers with the replies in order.
type scriptedModel struct{ replies []assistant.Reply }

// Reply returns the next reply.
func (m *scriptedModel) Reply(context.Context, []assistant.Message, []assistant.FunctionDefinition) (assistant.Reply, error) {
	reply := m.replies[0]
	m.replies = m.replies[1:]
	return reply, nil
}

func TestAsk_ContextEndsBetweenSteps_StopsBeforeTheNextStep(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	runs := 0
	function := assistant.Function{
		Definition: assistant.FunctionDefinition{Name: "slow"},
		Run: func(context.Context, json.RawMessage) (any, error) {
			runs++
			cancel() // The user closes the panel while the first step runs.
			return map[string]string{}, nil
		},
	}
	model := &scriptedModel{replies: []assistant.Reply{{Steps: []assistant.Step{
		{ID: "a", Function: "slow", Arguments: json.RawMessage(`{}`)},
		{ID: "b", Function: "slow", Arguments: json.RawMessage(`{}`)},
	}}}}

	_, err := assistant.Ask(ctx, model, []assistant.Message{{Role: assistant.RoleUser, Content: "Hi"}}, []assistant.Function{function})

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, runs, "the second step must not run")
}
