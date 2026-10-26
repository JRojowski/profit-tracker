package httpapi_test

import (
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
