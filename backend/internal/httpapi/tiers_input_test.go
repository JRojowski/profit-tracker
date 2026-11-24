package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/tier"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// putTiersRaw sends the body as it is to PUT /api/tiers.
func (s *testServer) putTiersRaw(t *testing.T, body string) (status int, responseBody string) {
	t.Helper()

	status, _, responseBody = s.do(t, http.MethodPut, "/api/tiers", bytes.NewReader([]byte(body)))
	return status, responseBody
}

// ikzeOnlyOperations is a purchase of ONLYIKZE.PL on the IKZE Account, which is not a Tier Account.
func ikzeOnlyOperations() []xtbtest.CashOperation {
	at := func(day int) time.Time { return time.Date(2026, 10, day, 9, 0, 0, 0, time.UTC) }
	return []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ONLYIKZE.PL", Instrument: "Only IKZE", Time: at(6), Amount: "-50.00", ID: "3", Comment: "OPEN BUY 1 @ 50.00", PositionID: "903"},
		ikzeDeposit("2", "1000.00", at(5)),
	}
}

// TestTiers_AssignedTickerWithNoFillOnATierAccount_StaysInTheListAndSaveable checks that the editor keeps an
// Instrument that a saved Tier holds, also when the Instrument has no Fill on a Tier Account any more.
func TestTiers_AssignedTickerWithNoFillOnATierAccount_StaysInTheListAndSaveable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importIKZE(t, ikzeOnlyOperations()...)
	require.NoError(t, srv.db.ReplaceTiers(t.Context(), []tier.Tier{{
		Order: 1, Name: "Core", TargetShare: decimal.NewFromInt(100), Instruments: []tier.Assignment{{Ticker: "ONLYIKZE.PL"}},
	}}))

	got := srv.getTiers(t)

	one := 1
	want := []tierInstrumentOption{{Ticker: "ONLYIKZE.PL", Name: "Only IKZE", TierOrder: &one}}
	assert.Equal(t, want, got.Instruments)

	saved := srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "ONLYIKZE.PL"}},
	}}})
	assert.Equal(t, want, saved.Instruments)
}

// TestTiers_InstrumentWithNoFillOnATierAccount_Returns422 checks that an Instrument of the IKZE Account is not
// assignable, although the tool knows it.
func TestTiers_InstrumentWithNoFillOnATierAccount_Returns422(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")
	srv.importIKZE(t, ikzeOnlyOperations()...)

	got := srv.putTiersInvalid(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100",
		Instruments: []assignmentRequest{{Ticker: "AAA.US"}, {Ticker: "ONLYIKZE.PL"}},
	}}})

	assert.Equal(t, []tierErrorEntry{
		{TierIndex: idx(0), Ticker: "ONLYIKZE.PL", Message: `Tier "Core": ONLYIKZE.PL is not a known Instrument.`},
	}, got.Errors)
}

// TestTiers_SharedPartName_IsTrimmed checks that the name of the Shared Part loses the spaces around it.
func TestTiers_SharedPartName_IsTrimmed(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")

	got := srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100", SharedPartName: "  Blue Chips \t",
	}}})

	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "Blue Chips", got.Tiers[0].SharedPart.Name)
}

// TestTiers_NumberIsNotValid_Returns422WithTheTierAndTheTicker checks that a bad number is a validation error
// of its Tier, not a JSON error.
func TestTiers_NumberIsNotValid_Returns422WithTheTierAndTheTicker(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []tierErrorEntry
	}{
		{
			name: "Target Share of a Tier with a comma",
			body: `{"tiers":[{"order":1,"name":"A","targetShare":"12,5"}]}`,
			want: []tierErrorEntry{{TierIndex: idx(0), Message: `Tier "A": the Target Share "12,5" is not a number.`}},
		},
		{
			name: "Target Share of an Instrument Target with letters",
			body: `{"tiers":[{"order":1,"name":"A","targetShare":"100","instruments":[{"ticker":"AAA.US","targetShare":"abc"}]}]}`,
			want: []tierErrorEntry{{TierIndex: idx(0), Ticker: "AAA.US", Message: `Tier "A": the Target Share "abc" of AAA.US is not a number.`}},
		},
		{
			name: "order that is not a whole number",
			body: `{"tiers":[{"order":1.5,"name":"A","targetShare":"100"}]}`,
			want: []tierErrorEntry{{TierIndex: idx(0), Message: `Tier "A": the order number "1.5" is not a whole number.`}},
		},
		{
			name: "order as text with letters, in the second Tier",
			body: `{"tiers":[{"order":1,"name":"A","targetShare":"50"},{"order":"x","name":"B","targetShare":"50"}]}`,
			want: []tierErrorEntry{{TierIndex: idx(1), Message: `Tier "B": the order number "x" is not a whole number.`}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startTiersServer(t, "AAA.US")

			status, body := srv.putTiersRaw(t, tc.body)

			require.Equal(t, http.StatusUnprocessableEntity, status, body)
			var got tiersErrorResponse
			require.NoError(t, json.Unmarshal([]byte(body), &got))
			assert.Equal(t, tc.want, got.Errors)
			assert.NotEmpty(t, got.Error)
		})
	}
}

// TestTiers_NumbersAsJSONNumbers_AreStillAccepted checks that a client may send the numbers with no quotes.
func TestTiers_NumbersAsJSONNumbers_AreStillAccepted(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")

	status, body := srv.putTiersRaw(t, `{"tiers":[{"order":1,"name":"A","targetShare":100,"instruments":[{"ticker":"AAA.US","targetShare":40.5}]}]}`)

	require.Equal(t, http.StatusOK, status, body)
	var got tiersResponse
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "100", got.Tiers[0].TargetShare)
	assert.Equal(t, []instrumentTargetDTO{{Ticker: "AAA.US", Name: "AAA", TargetShare: "40.5"}}, got.Tiers[0].InstrumentTargets)
}

// TestTiersBreakdown_LastSaleDate_IsThePolishDay checks that a sale at 23:30 UTC on 2026-10-09 is on 2026-10-10
// in Poland (UTC+2).
func TestTiersBreakdown_LastSaleDate_IsThePolishDay(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	bought := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	sold := time.Date(2026, 10, 9, 23, 30, 0, 0, time.UTC)
	id := srv.importAccount(t, xtbtest.Export{
		AccountNumber: "57200003", Currency: "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "SOLD.PL", Instrument: "Sold", Time: sold, Amount: "120.00", ID: "5", Comment: "CLOSE BUY 1 @ 120.00", PositionID: "902"},
			{Type: "Stock purchase", Ticker: "SOLD.PL", Instrument: "Sold", Time: bought, Amount: "-100.00", ID: "4", Comment: "OPEN BUY 1 @ 100.00", PositionID: "902"},
			cashDeposit("2", "1000.00", bought.Add(-time.Hour)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "SOLD.PL", Volume: "1", OpenTime: bought, CloseTime: sold, ProfitLoss: "20.00", PositionID: "902"},
		},
	}, "PLN Account")
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "SOLD.PL"}},
	}}})

	got := srv.tiersBreakdown(t, id)

	require.Len(t, got.Tiers, 1)
	require.Len(t, got.Tiers[0].Instruments, 1)
	closed := got.Tiers[0].Instruments[0].ClosedPositions
	require.NotNil(t, closed)
	assert.Equal(t, "2026-10-10", closed.LastSaleDate)
}

// TestAllTiersBreakdown_UnusedUSDAccountWithNoRate_StaysAvailable checks that a USD Account with no Holdings and
// no Closed Positions does not need the latest USD rate.
func TestAllTiersBreakdown_UnusedUSDAccountWithNoRate_StaysAvailable(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.rateProvider.fail("USD", errors.New("the NBP does not respond"))
	srv.importAccount(t, allocationExport("57300021", "USD", "1000.00"), "USD Account")

	got := srv.allTiersBreakdown(t)

	assert.True(t, got.Available, "warnings: %v", got.Warnings)
	assert.Empty(t, got.Warnings)
}
