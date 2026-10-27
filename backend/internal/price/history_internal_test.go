package price

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGap_IsFromOneToTwoSeconds(t *testing.T) {
	t.Parallel()
	for range 1000 {
		g := gap()
		require.GreaterOrEqual(t, g, time.Second)
		require.LessOrEqual(t, g, 2*time.Second)
	}
}

func TestPauseBetweenRequests_ContextEnds_StopsWaiting(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	err := PauseBetweenRequests(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), time.Second)
}
