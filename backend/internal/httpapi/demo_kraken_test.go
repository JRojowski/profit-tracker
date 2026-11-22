package httpapi_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/demo"
)

// TestDemoKrakenAccount_Import_GivesBitcoinHoldingRewardsAndFeeSum imports the demo Kraken export
// like scripts/demo.sh does: preview, then confirm with the proposed values.
//
//	Deposits:  7 × 700.00 = 4900.00
//	Purchases: 21, 4199.10 EUR in total. 4 have the fee 0.80 EUR, 17 have the fee in BTC (0.4 %).
//	Cash:      4900.00 - 4199.10 - 4 × 0.80 = 697.70
//	Rewards:   93 weekly, 0.00000020 + n × 0.00000002 BTC for n = 0..92, sum 0.00010416 BTC
//	BTC:       volume 0.05627348
//	           cost 4202.30 + 0.00010416 × 90000 (the only close, on or before each Reward) = 4211.6744
//	Rewards part: 0.00010416 × 90000 = 9.3744
//	Fee sum:   13.59732 (fees in BTC × purchase price) + 3.20 (fees in EUR) = 16.79732
//	Value at 70000: 697.70 + 0.05627348 × 70000 = 697.70 + 3939.1436 = 4636.8436
func TestDemoKrakenAccount_Import_GivesBitcoinHoldingRewardsAndFeeSum(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2025-01-10=90000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)

	preview := srv.previewKraken(t, demo.Kraken())
	assert.Empty(t, preview.Conflicts)
	assert.Equal(t, 7+21+93, preview.NewCashOperations, "deposits, purchases and Rewards")
	saved := srv.confirm(t, preview.ID, newAccountBody(preview.SuggestedName, preview.SuggestedType, preview.SuggestedCurrency))
	srv.refreshHistory(t)

	summary := srv.summary(t, saved.AccountID)
	assert.Equal(t, "697.70", summary.Cash)
	assert.Equal(t, "4900.00", summary.NetDeposits)
	holdings := srv.holdings(t, saved.AccountID)
	require.Len(t, holdings, 1)
	assert.Equal(t, "BTC", holdings[0].Ticker)
	assert.Equal(t, "0.05627348", holdings[0].Volume)
	assert.Equal(t, "4211.6744", holdings[0].Cost)
	got := srv.rewardsSummary(t, saved.AccountID)
	assert.Equal(t, ptr("9.3744"), got.Rewards)
	assert.Equal(t, "16.79732", got.FeeSum)
	assert.Equal(t, "4636.8436", got.Value)
}
