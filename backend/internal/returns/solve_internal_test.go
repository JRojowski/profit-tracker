package returns

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestSolve_PowerFails_DoesNotConverge checks the branch that no flow at the API gives: f fails inside the bracket.
func TestSolve_PowerFails_DoesNotConverge(t *testing.T) {
	t.Parallel()
	f := func(x decimal.Decimal) (fx, dfx decimal.Decimal, ok bool) {
		// f(x) = x − 2 has a sign change in the bracket, but f fails at the first guess.
		return x.Sub(two), decimal.NewFromInt(1), !x.Equal(xFirst)
	}

	_, reason := solve(f)

	assert.Equal(t, ReasonNoConvergence, reason)
}
