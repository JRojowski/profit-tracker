package httpapi_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// assistantKrakenHolding is the part of a get_holdings Holding that the Kraken tests read.
type assistantKrakenHolding struct {
	Ticker      string  `json:"ticker"`
	Volume      string  `json:"volume"`
	Cost        *string `json:"cost"`
	AverageCost *string `json:"averageCost"`
}

// TestAskAssistant_HoldingsOfKrakenAccount_GivesTheAverageCost checks get_holdings for the Kraken Account.
// ADR 0009, rule 5, with the values of krakenExport:
//
//	cost 300.00, volume 0.00448875, Average Cost 300.00 / 0.00448875 = 66833.7510442773600668 → 66833.75
func TestAskAssistant_HoldingsOfKrakenAccount_GivesTheAverageCost(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importKraken(t, krakenExport())

	_, content := askWithStep(t, srv, model, step("get_holdings", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got struct {
		Accounts []struct {
			Holdings []assistantKrakenHolding `json:"holdings"`
		} `json:"accounts"`
	}
	decode(t, content, &got)
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, []assistantKrakenHolding{{
		Ticker: "BTC", Volume: "0.00448875", Cost: ptr("300.00"), AverageCost: ptr("66833.75"),
	}}, got.Accounts[0].Holdings)
}

// TestAskAssistant_HoldingsOfXTBAccount_HasNoAverageCost checks that get_holdings gives the same result as before
// for an XTB Account. The glossary defines the Average Cost only for an Account with no Lots.
func TestAskAssistant_HoldingsOfXTBAccount_HasNoAverageCost(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_holdings", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got struct {
		Accounts []struct {
			Holdings []map[string]json.RawMessage `json:"holdings"`
		} `json:"accounts"`
	}
	decode(t, content, &got)
	require.Len(t, got.Accounts, 1)
	require.NotEmpty(t, got.Accounts[0].Holdings)
	for _, h := range got.Accounts[0].Holdings {
		assert.NotContains(t, h, "averageCost")
	}
}

// TestAskAssistant_HoldingsOfKrakenAccountWithUnknownRewardValue_GivesNullAverageCost checks a Reward with no
// stored close on or before its day. The cost, the Average Cost and the Unrealized Profit have no known value, so
// they are null. ADR 0009, rule 3: the price is known, so the value is 0.00448985 × 70000 = 314.2895 → 314.29.
func TestAskAssistant_HoldingsOfKrakenAccountWithUnknownRewardValue_GivesNullAverageCost(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-11=65000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))
	srv.refreshHistory(t)

	_, content := askWithStep(t, srv, model, step("get_holdings", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got struct {
		Accounts []struct {
			Holdings []map[string]json.RawMessage `json:"holdings"`
		} `json:"accounts"`
	}
	decode(t, content, &got)
	require.Len(t, got.Accounts, 1)
	require.Len(t, got.Accounts[0].Holdings, 1)
	holding := got.Accounts[0].Holdings[0]
	assert.JSONEq(t, "null", string(holding["cost"]))
	require.Contains(t, holding, "averageCost", "a Kraken Holding always names its Average Cost")
	assert.JSONEq(t, "null", string(holding["averageCost"]))
	assert.JSONEq(t, `"314.29"`, string(holding["value"]))
	assert.JSONEq(t, "null", string(holding["unrealizedProfit"]))
}

// assistantKrakenResults is the part of a get_results result that the Kraken tests read.
type assistantKrakenResults struct {
	TotalResult *string `json:"totalResult"`
	Fees        *string `json:"fees"`
	Rewards     *string `json:"rewards"`
	FeeSum      *string `json:"feeSum"`
}

// TestAskAssistant_ResultsOfKrakenAccount_GivesTheRewardsAndTheFeeSum checks get_results for the Kraken Account.
// The values are the values of TestKrakenReward_LaterPriceChange_GoesIntoUnrealizedProfitNotIntoRewards
// and TestKrakenFeeSum_Purchases_GivesTheSumOfTheFeesInEUR:
//
//	Rewards: 0.0682 + 0.0768 = 0.1450 → 0.15
//	Total Result: 14.3735 → 14.37
//	Sum of the fees: 0.25 + 0.50 = 0.75. The fees part stays 0.00 (ADR 0009, rule 2).
func TestAskAssistant_ResultsOfKrakenAccount_GivesTheRewardsAndTheFeeSum(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-10=62000", "2026-03-16=64000", "2026-03-18=66000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(
		krakenReward("ERN-0001", krakenTime(10), "0.00000110"),
		krakenReward("ERN-0002", krakenTime(17), "0.00000120"),
	))
	srv.refreshHistory(t)

	_, content := askWithStep(t, srv, model, step("get_results", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got assistantKrakenResults
	decode(t, content, &got)
	assert.Equal(t, assistantKrakenResults{
		TotalResult: ptr("14.37"), Fees: ptr("0.00"), Rewards: ptr("0.15"), FeeSum: ptr("0.75"),
	}, got)
}

// TestAskAssistant_ResultsOfKrakenAccountWithUnknownRewardValue_GivesNullRewards checks that the Rewards part
// with no known value is null, and that the result still names it. The Unrealized Profit is null too, because the
// tool cannot split the result into these two parts (ADR 0009, rule 3).
func TestAskAssistant_ResultsOfKrakenAccountWithUnknownRewardValue_GivesNullRewards(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	_, content := askWithStep(t, srv, model, step("get_results", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got map[string]json.RawMessage
	decode(t, content, &got)
	require.Contains(t, got, "rewards")
	assert.JSONEq(t, "null", string(got["rewards"]))
	assert.JSONEq(t, "null", string(got["unrealizedProfit"]))
	assert.JSONEq(t, `"0.75"`, string(got["feeSum"]))
}

// TestAskAssistant_ResultsOfXTBAccount_HasNoRewardsAndNoFeeSum checks that get_results gives the same result as
// before for an XTB Account.
func TestAskAssistant_ResultsOfXTBAccount_HasNoRewardsAndNoFeeSum(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_results", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got map[string]json.RawMessage
	decode(t, content, &got)
	assert.NotContains(t, got, "rewards")
	assert.NotContains(t, got, "feeSum")
}

// startKrakenRewardsServer starts the backend with the BTC-EUR closes of the Reward days, imports krakenExport
// with two Rewards and stores the history. It returns the server, the model and the Kraken Account ID.
//
//	Reward on 2026-03-10: 0.00000110 × 62000 (close of the day)                 = 0.0682
//	Reward on 2026-03-17: 0.00000120 × 64000 (close of 2026-03-16, on or before) = 0.0768
//	Fees: TRD-0001 on 2026-03-03 0.25, TRD-0002 on 2026-03-04 0.50
func startKrakenRewardsServer(t *testing.T) (*testServer, *fakeModel, int64) {
	t.Helper()

	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-10=62000", "2026-03-16=64000", "2026-03-18=66000")
	srv, model := startAssistantServer(t, provider)
	id := srv.importKraken(t, krakenExportWithRewards(
		krakenReward("ERN-0001", krakenTime(10), "0.00000110"),
		krakenReward("ERN-0002", krakenTime(17), "0.00000120"),
	))
	srv.refreshHistory(t)
	return srv, model, id
}

// assistantKrakenIncome is the part of a get_income_and_fees result that the Kraken tests read.
// A map keeps the difference between an absent field and a null field.
type assistantKrakenIncome struct {
	Totals   []map[string]any `json:"totals"`
	Accounts []map[string]any `json:"accounts"`
}

// TestAskAssistant_IncomeOfAllAccountsWithKrakenAccount_GivesTheRewardsAndTheFeeSum checks get_income_and_fees
// for an XTB Account and the Kraken Account. Only the Kraken Account and its currency have the Rewards
// (0.0682 + 0.0768 = 0.1450 → 0.15) and the sum of the fees (0.75).
func TestAskAssistant_IncomeOfAllAccountsWithKrakenAccount_GivesTheRewardsAndTheFeeSum(t *testing.T) {
	t.Parallel()
	srv, model, krakenID := startKrakenRewardsServer(t)
	xtbID := srv.importAccount(t, pricedExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_income_and_fees", `{}`))

	var got assistantKrakenIncome
	decode(t, content, &got)
	byAccount := map[float64]map[string]any{}
	for _, acc := range got.Accounts {
		byAccount[acc["accountId"].(float64)] = acc
	}
	assert.Equal(t, map[string]any{
		"accountId": float64(krakenID), "accountName": "Kraken", "currency": "EUR",
		"netDividends": "0.00", "netInterest": "0.00", "fees": "0.00", "rewards": "0.15", "feeSum": "0.75",
	}, byAccount[float64(krakenID)])
	assert.NotContains(t, byAccount[float64(xtbID)], "rewards")
	assert.NotContains(t, byAccount[float64(xtbID)], "feeSum")

	byCurrency := map[string]map[string]any{}
	for _, sums := range got.Totals {
		byCurrency[sums["currency"].(string)] = sums
	}
	assert.Equal(t, map[string]any{
		"currency": "EUR", "netDividends": "0.00", "netInterest": "0.00", "fees": "0.00", "rewards": "0.15", "feeSum": "0.75",
	}, byCurrency["EUR"])
	assert.NotContains(t, byCurrency["USD"], "rewards")
	assert.NotContains(t, byCurrency["USD"], "feeSum")
}

// TestAskAssistant_IncomeOfKrakenAccountInPeriod_CountsOnlyTheRewardsAndFeesOfThePeriod checks the period filter.
// From 2026-03-15: only the Reward of 2026-03-17 (0.0768 → 0.08) and no purchase, so the sum of the fees is 0.00.
func TestAskAssistant_IncomeOfKrakenAccountInPeriod_CountsOnlyTheRewardsAndFeesOfThePeriod(t *testing.T) {
	t.Parallel()
	srv, model, id := startKrakenRewardsServer(t)

	_, content := askWithStep(t, srv, model,
		step("get_income_and_fees", fmt.Sprintf(`{"accountId": %d, "instrument": "btc", "from": "2026-03-15"}`, id)))

	var got assistantKrakenIncome
	decode(t, content, &got)
	require.Len(t, got.Accounts, 1)
	assert.Equal(t, "0.08", got.Accounts[0]["rewards"])
	assert.Equal(t, "0.00", got.Accounts[0]["feeSum"])
}

// TestAskAssistant_IncomeOfKrakenAccountWithUnknownRewardValue_GivesNullRewards checks a Reward with no stored close.
func TestAskAssistant_IncomeOfKrakenAccountWithUnknownRewardValue_GivesNullRewards(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	id := srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))

	_, content := askWithStep(t, srv, model, step("get_income_and_fees", fmt.Sprintf(`{"accountId": %d}`, id)))

	var got assistantKrakenIncome
	decode(t, content, &got)
	require.Len(t, got.Accounts, 1)
	require.Contains(t, got.Accounts[0], "rewards")
	assert.Nil(t, got.Accounts[0]["rewards"])
	require.Len(t, got.Totals, 1)
	require.Contains(t, got.Totals[0], "rewards")
	assert.Nil(t, got.Totals[0]["rewards"])
	assert.Equal(t, "0.75", got.Totals[0]["feeSum"])
}

// TestAskAssistant_XTBAndKrakenAccounts_InstructionsNameBothProviders checks that the instructions do not say
// that all Accounts are at XTB, and that they give the Provider of each Account.
func TestAskAssistant_XTBAndKrakenAccounts_InstructionsNameBothProviders(t *testing.T) {
	t.Parallel()
	model := newFakeModel(assistant.Reply{Text: "An answer."})
	srv := startServerWithOptions(t, serverOptions{model: model})
	xtbID := srv.importAccount(t, dashboardExport(), "XTB USD")
	krakenID := srv.importKraken(t, krakenExport())

	srv.ask(t, question("What is the Average Cost of my Bitcoin?"))

	calls := model.recordedCalls()
	require.Len(t, calls, 1)
	instructions := calls[0][0].Content
	assert.NotContains(t, instructions, "at the broker XTB")
	for _, want := range []string{
		"two Providers: the broker XTB and the crypto exchange Kraken",
		"- Average Cost:",
		"- Reward:",
		"Realized Profit, Unrealized Profit, net dividends, net interest, Rewards and fees",
		fmt.Sprintf(`- ID %d: "XTB USD", Provider XTB, type regular, currency USD`, xtbID),
		fmt.Sprintf(`- ID %d: "Kraken", Provider Kraken, type regular, currency EUR`, krakenID),
	} {
		assert.Contains(t, instructions, want)
	}
}

// TestAskAssistant_ResultsOfAllAccountsWithKrakenAccount_GivesTheRewardsInPLN checks get_results for all Accounts
// with an XTB Account in PLN and the Kraken Account, with the values of startXTBAndKrakenServer.
//
//	Rewards:      0.0682 EUR × 4.25 = 0.28985 PLN → 0.29
//	Total Result: 60.440525 + 0.28985 + 10.00 = 70.730375 PLN → 70.73
func TestAskAssistant_ResultsOfAllAccountsWithKrakenAccount_GivesTheRewardsInPLN(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "BTC-EUR", "EUR", "2026-03-10=62000")
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv, model := startAssistantServer(t, provider)
	srv.rateProvider.set(t, "EUR", "2026-02-27", "4.20")
	srv.rateProvider.set(t, "EUR", "2026-03-02", "4.30")
	srv.rateProvider.set(t, "EUR", "2026-10-22", "4.25")
	srv.importAccount(t, xtbtest.Export{AccountNumber: "59000001", Currency: "PLN", CashOperations: []xtbtest.CashOperation{
		cashDeposit("9101", "1000.00", krakenTime(2)),
	}}, "XTB PLN")
	srv.importKraken(t, krakenExportWithRewards(krakenReward("ERN-0001", krakenTime(10), "0.00000110")))
	srv.refreshHistory(t)

	_, content := askWithStep(t, srv, model, step("get_results", `{}`))

	var got map[string]json.RawMessage
	decode(t, content, &got)
	assert.JSONEq(t, `"0.29"`, string(got["rewards"]))
	assert.JSONEq(t, `"70.73"`, string(got["totalResult"]))
	assert.NotContains(t, got, "feeSum", "the sum of the fees stays a value of the Kraken Account in EUR")
}

// TestAskAssistant_ResultsOfAllAccountsWithNoKrakenAccount_HasNoRewards checks that get_results for all XTB
// Accounts gives the same result as before.
func TestAskAssistant_ResultsOfAllAccountsWithNoKrakenAccount_HasNoRewards(t *testing.T) {
	t.Parallel()
	srv, model := startAssistantServer(t, newFakeProvider())
	srv.importAccount(t, pricedExport(), "XTB USD")

	_, content := askWithStep(t, srv, model, step("get_results", `{}`))

	var got map[string]json.RawMessage
	decode(t, content, &got)
	assert.NotContains(t, got, "rewards")
}
