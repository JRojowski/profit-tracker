package rebalancing_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/rebalancing"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// TestCalculate_PositiveGapsMoreThanCash_PlannedPurchasesAddUpToTheCashExactly checks the rounding of the factor.
// The Account has cash 2 and a "No Tier" Holding of value 1, so the Base is 3. Three Tiers have a Target Share of
// 33.33, so each Gap is 0.9999. The scaled part is 0.9999 × 2 / 2.9997, which rounds up. The sum must not pass 2.
func TestCalculate_PositiveGapsMoreThanCash_PlannedPurchasesAddUpToTheCashExactly(t *testing.T) {
	t.Parallel()
	share := decimal.RequireFromString("33.33")
	in := rebalancing.Input{
		Accounts: []rebalancing.Account{{
			ID: 1, Currency: rebalancing.USD, Rate: decimal.NewFromInt(1), Cash: decimal.NewFromInt(2),
			Holdings: []rebalancing.Holding{{Ticker: "FREE.US", HasValue: true, Value: decimal.NewFromInt(1)}},
		}},
		Tiers: []tier.Tier{
			{Order: 1, Name: "A", TargetShare: share},
			{Order: 2, Name: "B", TargetShare: share},
			{Order: 3, Name: "C", TargetShare: share},
		},
	}

	got := rebalancing.Calculate(in)

	require.Len(t, got.Accounts, 1)
	acc := got.Accounts[0]
	assert.True(t, decimal.NewFromInt(2).Equal(acc.PlannedPurchases), "planned: %s", acc.PlannedPurchases)
	assert.True(t, acc.UnusedCash.IsZero(), "unused: %s", acc.UnusedCash)
	sum := decimal.Zero
	for _, row := range got.Tiers {
		sum = sum.Add(row.SharedPart.Purchase.Amount)
	}
	assert.True(t, decimal.NewFromInt(2).Equal(sum), "sum of the parts: %s", sum)
}
