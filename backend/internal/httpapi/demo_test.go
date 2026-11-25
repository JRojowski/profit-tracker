package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/demo"
)

// The demo data must import with no Import Conflicts, so that `make demo` keeps working.
func TestDemoExports_ImportWithNoConflicts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	for _, a := range demo.Accounts() {
		preview := srv.preview(t, a.Export)
		require.Empty(t, preview.Conflicts, "account %s", a.Name)

		got := srv.confirm(t, preview.ID, newAccountBody(a.Name, a.Type, a.Currency))
		assert.Equal(t, preview.ID, got.ID, "account %s", a.Name)
	}
}

// The sample Tiers of the demo data must be a valid configuration for the demo Instruments, so that `make demo`
// keeps working.
func TestDemoTiers_AreValidForTheDemoAccounts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	for _, a := range demo.Accounts() {
		srv.confirm(t, srv.preview(t, a.Export).ID, newAccountBody(a.Name, a.Type, a.Currency))
	}
	body, err := json.Marshal(demo.Tiers())
	require.NoError(t, err)

	status, _, got := srv.do(t, http.MethodPut, "/api/tiers", bytes.NewReader(body))

	require.Equal(t, http.StatusOK, status, got)
	tiers := srv.getTiers(t)
	assert.Len(t, tiers.Tiers, 4)
	// INTC.US is sold in full. It stays in its Tier and in the list for assignment.
	var intc *tierInstrumentOption
	for i := range tiers.Instruments {
		if tiers.Instruments[i].Ticker == "INTC.US" {
			intc = &tiers.Instruments[i]
		}
	}
	require.NotNil(t, intc)
	assert.False(t, intc.HasOpenHolding)
	assert.NotNil(t, intc.TierOrder)
}
