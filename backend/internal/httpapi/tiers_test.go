package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// tiersResponse is the body of GET /api/tiers and of a successful PUT /api/tiers.
type tiersResponse struct {
	Tiers       []tierDTO              `json:"tiers"`
	Instruments []tierInstrumentOption `json:"instruments"`
}

// tierDTO is one Tier of the response.
type tierDTO struct {
	Order             int                   `json:"order"`
	Name              string                `json:"name"`
	TargetShare       string                `json:"targetShare"`
	SharedPart        sharedPartDTO         `json:"sharedPart"`
	InstrumentTargets []instrumentTargetDTO `json:"instrumentTargets"`
	SharedInstruments []sharedInstrumentDTO `json:"sharedInstruments"`
}

// sharedPartDTO is the Shared Part of a Tier.
type sharedPartDTO struct {
	Name        string `json:"name"`
	TargetShare string `json:"targetShare"`
}

// instrumentTargetDTO is an Instrument Target of a Tier.
type instrumentTargetDTO struct {
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	TargetShare string `json:"targetShare"`
}

// sharedInstrumentDTO is an Instrument of the Shared Part of a Tier.
type sharedInstrumentDTO struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
}

// tierInstrumentOption is one Instrument of the list for assignment.
type tierInstrumentOption struct {
	Ticker         string  `json:"ticker"`
	Name           string  `json:"name"`
	TierOrder      *int    `json:"tierOrder"`
	TargetShare    *string `json:"targetShare"`
	HasOpenHolding bool    `json:"hasOpenHolding"`
}

// tiersRequest is the body of PUT /api/tiers.
type tiersRequest struct {
	Tiers []tierRequest `json:"tiers"`
}

// tierRequest is one Tier of the request.
type tierRequest struct {
	Order          int                 `json:"order"`
	Name           string              `json:"name"`
	TargetShare    string              `json:"targetShare"`
	SharedPartName string              `json:"sharedPartName"`
	Instruments    []assignmentRequest `json:"instruments"`
}

// assignmentRequest assigns an Instrument to a Tier. A nil TargetShare puts it in the Shared Part.
type assignmentRequest struct {
	Ticker      string  `json:"ticker"`
	TargetShare *string `json:"targetShare"`
}

// tiersErrorResponse is the 422 body of PUT /api/tiers.
type tiersErrorResponse struct {
	Error  string           `json:"error"`
	Errors []tierErrorEntry `json:"errors"`
}

// tierErrorEntry is one message of the 422 body.
type tierErrorEntry struct {
	TierIndex *int   `json:"tierIndex"`
	Ticker    string `json:"ticker"`
	Message   string `json:"message"`
}

// getTiers returns GET /api/tiers. It stops the test if the status is not 200.
func (s *testServer) getTiers(t *testing.T) tiersResponse {
	t.Helper()

	var got tiersResponse
	require.Equal(t, http.StatusOK, s.getJSON(t, "/api/tiers", &got))
	return got
}

// putTiers sends the configuration to PUT /api/tiers and returns the status and the body.
func (s *testServer) putTiers(t *testing.T, req tiersRequest) (status int, body string) {
	t.Helper()

	data, err := json.Marshal(req)
	require.NoError(t, err)
	status, _, body = s.do(t, http.MethodPut, "/api/tiers", bytes.NewReader(data))
	return status, body
}

// putTiersOK saves the configuration. It stops the test if the status is not 200.
func (s *testServer) putTiersOK(t *testing.T, req tiersRequest) tiersResponse {
	t.Helper()

	status, body := s.putTiers(t, req)
	require.Equal(t, http.StatusOK, status, body)
	var got tiersResponse
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	return got
}

// putTiersInvalid sends the configuration and expects 422. It returns the decoded body.
func (s *testServer) putTiersInvalid(t *testing.T, req tiersRequest) tiersErrorResponse {
	t.Helper()

	status, body := s.putTiers(t, req)
	require.Equal(t, http.StatusUnprocessableEntity, status, body)
	var got tiersErrorResponse
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	return got
}

func TestTiers_NoConfiguration_ReturnsEmptyLists(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	got := srv.getTiers(t)

	assert.Equal(t, tiersResponse{Tiers: []tierDTO{}, Instruments: []tierInstrumentOption{}}, got)
}

// startTiersServer starts the backend and imports a USD Account that bought the tickers.
// The Instruments exist, so a Tier can hold them.
func startTiersServer(t *testing.T, tickers ...string) *testServer {
	t.Helper()

	srv := startServer(t)
	holdings := make([]allocationHolding, 0, len(tickers))
	for _, ticker := range tickers {
		holdings = append(holdings, allocationHolding{ticker: ticker, cost: "10.00"})
	}
	srv.importAccount(t, allocationExport("57100001", "USD", "1000.00", holdings...), "USD Account")
	return srv
}

// share returns a pointer to the Target Share text.
func share(s string) *string { return &s }

// TestTiers_SaveThenRead_ReturnsTheConfiguration saves two Tiers and reads them back.
//
//	Tier I:  Target Share 60. Instrument Target AAA.US 25. Shared Part gets 100 - 25 = 75, with BBB.US.
//	Tier II: Target Share 40. No Instrument Targets, so the Shared Part gets 100.
func TestTiers_SaveThenRead_ReturnsTheConfiguration(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US", "BBB.US")
	req := tiersRequest{Tiers: []tierRequest{
		{
			Order: 1, Name: "I. Core", TargetShare: "60", SharedPartName: "Blue Chips",
			Instruments: []assignmentRequest{{Ticker: "BBB.US"}, {Ticker: "AAA.US", TargetShare: share("25")}},
		},
		{Order: 2, Name: "II. Bets", TargetShare: "40"},
	}}

	saved := srv.putTiersOK(t, req)
	got := srv.getTiers(t)

	want := []tierDTO{
		{
			Order: 1, Name: "I. Core", TargetShare: "60", SharedPart: sharedPartDTO{Name: "Blue Chips", TargetShare: "75"},
			InstrumentTargets: []instrumentTargetDTO{{Ticker: "AAA.US", Name: "AAA", TargetShare: "25"}},
			SharedInstruments: []sharedInstrumentDTO{{Ticker: "BBB.US", Name: "BBB"}},
		},
		{
			Order: 2, Name: "II. Bets", TargetShare: "40", SharedPart: sharedPartDTO{TargetShare: "100"},
			InstrumentTargets: []instrumentTargetDTO{}, SharedInstruments: []sharedInstrumentDTO{},
		},
	}
	assert.Equal(t, want, got.Tiers)
	assert.Equal(t, got, saved, "PUT returns the saved configuration")
}

// TestTiers_SaveAgain_ReplacesTheConfiguration checks that a save removes the Tiers and the assignments that it omits.
func TestTiers_SaveAgain_ReplacesTheConfiguration(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "Old", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
	}})

	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{Order: 1, Name: "New", TargetShare: "100"}}})
	got := srv.getTiers(t)

	require.Len(t, got.Tiers, 1)
	assert.Equal(t, "New", got.Tiers[0].Name)
	assert.Empty(t, got.Tiers[0].SharedInstruments)
}

// TestTiers_TargetShareOfZero_IsValid checks that a Tier that the user does not buy any more can stay.
func TestTiers_TargetShareOfZero_IsValid(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")

	got := srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{
		{Order: 1, Name: "Live", TargetShare: "100"},
		{Order: 2, Name: "Retired", TargetShare: "0", Instruments: []assignmentRequest{{Ticker: "AAA.US", TargetShare: share("0")}}},
	}})

	require.Len(t, got.Tiers, 2)
	assert.Equal(t, "0", got.Tiers[1].TargetShare)
	assert.Equal(t, "100", got.Tiers[1].SharedPart.TargetShare)
}

func idx(i int) *int { return &i }

// TestTiers_InvalidConfiguration_Returns422WithAMessageForEachError checks each rule of PUT /api/tiers.
// The save stops, so the stored configuration stays as it was.
func TestTiers_InvalidConfiguration_Returns422WithAMessageForEachError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  tiersRequest
		want []tierErrorEntry
	}{
		{
			name: "Tier Target Shares do not add up to 100",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "60"}, {Order: 2, Name: "B", TargetShare: "30"},
			}},
			want: []tierErrorEntry{{Message: "The Target Shares of the Tiers add up to 90, not 100."}},
		},
		{
			name: "Instrument Targets of one Tier add up to more than 100",
			req: tiersRequest{Tiers: []tierRequest{{
				Order: 1, Name: "Core", TargetShare: "100",
				Instruments: []assignmentRequest{{Ticker: "AAA.US", TargetShare: share("60")}, {Ticker: "BBB.US", TargetShare: share("40.5")}},
			}}},
			want: []tierErrorEntry{{TierIndex: idx(0), Message: `Tier "Core": the Instrument Targets add up to 100.5, more than 100.`}},
		},
		{
			name: "Instrument Targets of exactly 100 are valid",
			req: tiersRequest{Tiers: []tierRequest{{
				Order: 1, Name: "Core", TargetShare: "100",
				Instruments: []assignmentRequest{{Ticker: "AAA.US", TargetShare: share("60")}, {Ticker: "BBB.US", TargetShare: share("40")}},
			}}},
		},
		{
			name: "Tier Target Share above 100",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "120"}, {Order: 2, Name: "B", TargetShare: "-20"},
			}},
			want: []tierErrorEntry{
				{TierIndex: idx(0), Message: `Tier "A": the Target Share 120 is not from 0 to 100.`},
				{TierIndex: idx(1), Message: `Tier "B": the Target Share -20 is not from 0 to 100.`},
			},
		},
		{
			name: "Instrument Target below 0",
			req: tiersRequest{Tiers: []tierRequest{{
				Order: 1, Name: "A", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "AAA.US", TargetShare: share("-1")}},
			}}},
			want: []tierErrorEntry{{TierIndex: idx(0), Ticker: "AAA.US", Message: `Tier "A": the Target Share -1 of AAA.US is not from 0 to 100.`}},
		},
		{
			name: "order number is not unique",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "50"}, {Order: 1, Name: "B", TargetShare: "50"},
			}},
			want: []tierErrorEntry{{TierIndex: idx(1), Message: `Tier "B": the order number 1 is also the order number of Tier "A".`}},
		},
		{
			name: "name is not unique",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "50"}, {Order: 2, Name: "A", TargetShare: "50"},
			}},
			want: []tierErrorEntry{{TierIndex: idx(1), Message: `Tier "A": the name is also the name of Tier 1.`}},
		},
		{
			name: "name is empty",
			req:  tiersRequest{Tiers: []tierRequest{{Order: 1, Name: "  ", TargetShare: "100"}}},
			want: []tierErrorEntry{{TierIndex: idx(0), Message: "Tier 1: the name is empty."}},
		},
		{
			name: "ticker is unknown",
			req: tiersRequest{Tiers: []tierRequest{{
				Order: 1, Name: "A", TargetShare: "100", Instruments: []assignmentRequest{{Ticker: "NOPE.US"}},
			}}},
			want: []tierErrorEntry{{TierIndex: idx(0), Ticker: "NOPE.US", Message: `Tier "A": NOPE.US is not a known Instrument.`}},
		},
		{
			name: "Instrument is in two Tiers",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "50", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
				{Order: 2, Name: "B", TargetShare: "50", Instruments: []assignmentRequest{{Ticker: "AAA.US"}}},
			}},
			want: []tierErrorEntry{{TierIndex: idx(1), Ticker: "AAA.US", Message: `Tier "B": AAA.US is already in Tier "A".`}},
		},
		{
			name: "all errors are returned together",
			req: tiersRequest{Tiers: []tierRequest{
				{Order: 1, Name: "A", TargetShare: "50", Instruments: []assignmentRequest{{Ticker: "NOPE.US"}}},
				{Order: 1, Name: "B", TargetShare: "40"},
			}},
			want: []tierErrorEntry{
				{TierIndex: idx(0), Ticker: "NOPE.US", Message: `Tier "A": NOPE.US is not a known Instrument.`},
				{TierIndex: idx(1), Message: `Tier "B": the order number 1 is also the order number of Tier "A".`},
				{Message: "The Target Shares of the Tiers add up to 90, not 100."},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startTiersServer(t, "AAA.US", "BBB.US")
			srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{Order: 9, Name: "Kept", TargetShare: "100"}}})

			if len(tc.want) == 0 {
				srv.putTiersOK(t, tc.req)
				return
			}
			got := srv.putTiersInvalid(t, tc.req)

			assert.Equal(t, tc.want, got.Errors)
			assert.NotEmpty(t, got.Error)
			kept := srv.getTiers(t)
			require.Len(t, kept.Tiers, 1)
			assert.Equal(t, "Kept", kept.Tiers[0].Name)
		})
	}
}

// TestTiers_NoTiers_ClearsTheConfiguration checks that a save with no Tiers is valid and removes the old Tiers.
func TestTiers_NoTiers_ClearsTheConfiguration(t *testing.T) {
	t.Parallel()
	srv := startTiersServer(t, "AAA.US")
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{Order: 1, Name: "A", TargetShare: "100"}}})

	got := srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{}})

	assert.Empty(t, got.Tiers)
}

// TestTiers_InstrumentsForAssignment_AreTheInstrumentsWithFillsOnATierAccount checks the list for assignment.
//
//	PLN Account (regular):  HELD.PL is open. SOLD.PL is sold in full.
//	USD Account (regular):  USD1.US is open.
//	IKZE Account:           ONLYIKZE.PL is open. It is not in the list.
//	HELD.PL is in Tier 1 as an Instrument Target of 25. USD1.US is in Tier 1 in the Shared Part.
func TestTiers_InstrumentsForAssignment_AreTheInstrumentsWithFillsOnATierAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	at := func(day int) time.Time { return time.Date(2026, 10, day, 9, 0, 0, 0, time.UTC) }
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "57200001", Currency: "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "SOLD.PL", Instrument: "Sold", Time: at(8), Amount: "120.00", ID: "5", Comment: "CLOSE BUY 1 @ 120.00", PositionID: "902"},
			{Type: "Stock purchase", Ticker: "SOLD.PL", Instrument: "Sold", Time: at(7), Amount: "-100.00", ID: "4", Comment: "OPEN BUY 1 @ 100.00", PositionID: "902"},
			{Type: "Stock purchase", Ticker: "HELD.PL", Instrument: "Held", Time: at(6), Amount: "-50.00", ID: "3", Comment: "OPEN BUY 1 @ 50.00", PositionID: "901"},
			cashDeposit("2", "1000.00", at(5)),
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "SOLD.PL", Volume: "1", OpenTime: at(7), CloseTime: at(8), ProfitLoss: "20.00", PositionID: "902"},
		},
	}, "PLN Account")
	srv.importAccount(t, allocationExport("57200002", "USD", "1000.00", allocationHolding{ticker: "USD1.US", cost: "10.00"}), "USD Account")
	srv.importIKZE(t,
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "ONLYIKZE.PL", Instrument: "Only IKZE", Time: at(6), Amount: "-50.00", ID: "3", Comment: "OPEN BUY 1 @ 50.00", PositionID: "903"},
		ikzeDeposit("2", "1000.00", at(5)),
	)
	srv.putTiersOK(t, tiersRequest{Tiers: []tierRequest{{
		Order: 1, Name: "Core", TargetShare: "100",
		Instruments: []assignmentRequest{{Ticker: "HELD.PL", TargetShare: share("25")}, {Ticker: "USD1.US"}},
	}}})

	got := srv.getTiers(t)

	one := 1
	assert.Equal(t, []tierInstrumentOption{
		{Ticker: "HELD.PL", Name: "Held", TierOrder: &one, TargetShare: share("25"), HasOpenHolding: true},
		{Ticker: "SOLD.PL", Name: "Sold"},
		{Ticker: "USD1.US", Name: "USD1", TierOrder: &one, HasOpenHolding: true},
	}, got.Instruments)
}

// TestTiers_KrakenAccount_HasNoInstrumentsForAssignment checks that a crypto Instrument is not in the list.
func TestTiers_KrakenAccount_HasNoInstrumentsForAssignment(t *testing.T) {
	t.Parallel()
	srv, _, _ := startXTBAndKrakenServer(t)

	got := srv.getTiers(t)

	assert.Empty(t, got.Instruments)
}

// TestTiers_BodyIsNotJSON_Returns400checks that a broken body is not a validation error.
func TestTiers_BodyIsNotJSON_Returns400(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	status, _, _ := srv.do(t, http.MethodPut, "/api/tiers", bytes.NewReader([]byte("{")))

	assert.Equal(t, http.StatusBadRequest, status)
}
