package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// overviewResponse is the body of GET /api/accounts/{id}/instruments/{ticker}/overview.
type overviewResponse struct {
	Ticker              string               `json:"ticker"`
	Name                string               `json:"name"`
	PriceTicker         string               `json:"priceTicker"`
	PriceTickerManual   bool                 `json:"priceTickerManual"`
	QuoteCurrency       *string              `json:"quoteCurrency"`
	QuoteCurrencyManual bool                 `json:"quoteCurrencyManual"`
	PriceConfirmed      bool                 `json:"priceConfirmed"`
	Prices              overviewPrices       `json:"prices"`
	Fills               []overviewFillResult `json:"fills"`
	// Holding and OpenLots are checked in their own tests. The tests of the other fields leave them out.
	Holding  json.RawMessage   `json:"holding"`
	OpenLots []overviewOpenLot `json:"openLots"`
	// ClosedPositions are the items of the Closed Positions endpoint, as raw JSON.
	ClosedPositions []json.RawMessage `json:"closedPositions"`
}

type overviewOpenLot struct {
	PositionID string               `json:"positionId"`
	Volume     string               `json:"volume"`
	Cost       string               `json:"cost"`
	OpenTime   string               `json:"openTime"`
	OpenPrice  *string              `json:"openPrice"`
	Fills      []overviewFillResult `json:"fills"`
}

// withoutLotData returns the overview with no Holding and no open Lots.
func (o overviewResponse) withoutLotData() overviewResponse {
	o.Holding, o.OpenLots, o.ClosedPositions = nil, nil, nil
	return o
}

type overviewPrices struct {
	Currency *string         `json:"currency"`
	Series   []overviewPoint `json:"series"`
}

type overviewPoint struct {
	Date  string `json:"date"`
	Close string `json:"close"`
}

type overviewFillResult struct {
	XTBID      string  `json:"xtbId"`
	Time       string  `json:"time"`
	Volume     string  `json:"volume"`
	Price      *string `json:"price"`
	PositionID string  `json:"positionId"`
}

// overviewPath returns the path of the Instrument Overview.
func overviewPath(accountID int64, ticker string) string {
	return fmt.Sprintf("/api/accounts/%d/instruments/%s/overview", accountID, ticker)
}

// overview returns the Instrument Overview. It stops the test if the status is not 200.
func (s *testServer) overview(t *testing.T, accountID int64, ticker string) overviewResponse {
	t.Helper()

	var got overviewResponse
	status := s.getJSON(t, overviewPath(accountID, ticker), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// storeDailyCloses stores the closes of the daily history of the Instrument. Each close is "YYYY-MM-DD=price".
func (s *testServer) storeDailyCloses(t *testing.T, ticker, currency string, closes ...string) {
	t.Helper()

	var prices []store.Price
	for _, c := range closes {
		date, value, ok := strings.Cut(c, "=")
		require.True(t, ok, "close %q", c)
		at, err := time.Parse(time.DateOnly, date)
		require.NoError(t, err)
		prices = append(prices, store.Price{
			Ticker: ticker, Date: at, Close: decimal.RequireFromString(value), Currency: currency, Source: "test",
			FetchedAt: testToday,
		})
	}
	asked := store.PriceRange{
		Ticker: ticker, FirstDay: prices[0].Date, LastDay: prices[len(prices)-1].Date, FetchedAt: testToday,
	}
	require.NoError(t, s.db.SaveDailyPrices(t.Context(), asked, prices))
}

// overviewExport is a USD Account with one Lot of ABC.US with one Fill.
func overviewExport() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "57000001", Currency: "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC Inc", Time: opTime(3), Amount: "-200.00", ID: "902", Comment: "OPEN BUY 2 @ 100.00", PositionID: "9000000001"},
			{Type: "Deposit", Time: opTime(1), Amount: "1000.00", ID: "901", Comment: "Blik deposit"},
		},
		OpenLots: []xtbtest.OpenLot{{Ticker: "ABC.US", PositionID: "9000000001", Volume: "2"}},
	}
}

func TestInstrumentOverview_OpenHoldingWithNoSells_GivesIdentityPricesAndFill(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")
	srv.storeDailyCloses(t, "ABC.US", "USD", "2026-02-03=101", "2026-02-04=102.5")
	// A current price is not a close of the daily history. The series must not contain it.
	require.NoError(t, srv.db.SavePrice(t.Context(), store.Price{
		Ticker: "ABC.US", Date: time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), Close: decimal.RequireFromString("103"),
		Currency: "USD", Source: "test", FetchedAt: testToday,
	}))

	got := srv.overview(t, accountID, "ABC.US")

	assert.Equal(t, overviewResponse{
		Ticker: "ABC.US", Name: "ABC Inc", PriceTicker: "ABC", QuoteCurrency: ptr("USD"),
		Prices: overviewPrices{
			Currency: ptr("USD"),
			Series:   []overviewPoint{{Date: "2026-02-03", Close: "101"}, {Date: "2026-02-04", Close: "102.5"}},
		},
		Fills: []overviewFillResult{
			{XTBID: "902", Time: "2026-02-03T10:00:00Z", Volume: "2", Price: ptr("100"), PositionID: "9000000001"},
		},
	}, got.withoutLotData())
}

func TestInstrumentOverview_LotWithTwoFills_GivesTwoFillsInTimeOrder(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// ADR 0002 example A: one Lot 2860602916 with the Fills 0.27 @ 39.37 and 1 @ 39.24.
	accountID := srv.importAccount(t, adr0002ExampleA(), "XTB USD")

	got := srv.overview(t, accountID, "FPS.US")

	assert.Equal(t, []overviewFillResult{
		{XTBID: "502", Time: "2026-02-03T09:30:00Z", Volume: "0.27", Price: ptr("39.37"), PositionID: "2860602916"},
		{XTBID: "503", Time: "2026-02-03T09:30:02Z", Volume: "1", Price: ptr("39.24"), PositionID: "2860602916"},
	}, got.Fills)
}

func TestInstrumentOverview_SameInstrumentOnTwoAccounts_EachAccountGivesItsOwnFills(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.importAccount(t, overviewExport(), "XTB USD")
	other := overviewExport()
	other.AccountNumber = "57000002"
	other.CashOperations = []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC Inc", Time: opTime(6), Amount: "-30.00", ID: "912", Comment: "OPEN BUY 1 @ 30.00", PositionID: "9100000001"},
		{Type: "Deposit", Time: opTime(1), Amount: "100.00", ID: "911", Comment: "Blik deposit"},
	}
	other.OpenLots = []xtbtest.OpenLot{{Ticker: "ABC.US", PositionID: "9100000001", Volume: "1"}}
	second := srv.importAccount(t, other, "XTB USD 2")

	assert.Equal(t, []overviewFillResult{
		{XTBID: "902", Time: "2026-02-03T10:00:00Z", Volume: "2", Price: ptr("100"), PositionID: "9000000001"},
	}, srv.overview(t, first, "ABC.US").Fills)
	assert.Equal(t, []overviewFillResult{
		{XTBID: "912", Time: "2026-02-06T10:00:00Z", Volume: "1", Price: ptr("30"), PositionID: "9100000001"},
	}, srv.overview(t, second, "ABC.US").Fills)
}

func TestInstrumentOverview_NoPriceData_GivesEmptySeriesAndNullCurrency(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")

	status, _, body := srv.get(t, overviewPath(accountID, "ABC.US"))

	require.Equal(t, http.StatusOK, status, "body %q", body)
	assert.Contains(t, body, `"prices":{"currency":null,"series":[]}`, "the series is an empty array, not null")
	assert.Contains(t, body, `"fills":[{`)
}

func TestInstrumentOverview_FullySoldInstrument_StillGivesFills(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := overviewExport()
	export.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock sell", Ticker: "ABC.US", Instrument: "ABC Inc", Time: closeTime(5), Amount: "220.00", ID: "903", Comment: "CLOSE BUY 2/2 @ 110.00", PositionID: "9000000001"},
	}, export.CashOperations...)
	export.OpenLots = nil
	export.ClosedPositions = []xtbtest.ClosedPosition{
		{Ticker: "ABC.US", Instrument: "ABC Inc", Volume: "2", OpenTime: opTime(3), CloseTime: closeTime(5), ProfitLoss: "20.00", PositionID: "9000000001"},
	}
	accountID := srv.importAccount(t, export, "XTB USD")
	require.Empty(t, srv.holdings(t, accountID), "the Instrument is fully sold")

	got := srv.overview(t, accountID, "ABC.US")

	assert.Equal(t, []overviewFillResult{
		{XTBID: "902", Time: "2026-02-03T10:00:00Z", Volume: "2", Price: ptr("100"), PositionID: "9000000001"},
	}, got.Fills, "the sell is not a Fill")
}

func TestInstrumentOverview_UnknownAccount_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, overviewExport(), "XTB USD")

	for _, id := range []string{"999", "abc"} {
		status, _, body := srv.get(t, "/api/accounts/"+id+"/instruments/ABC.US/overview")
		assert.Equal(t, http.StatusNotFound, status, "account %s, body %q", id, body)
	}
}

func TestInstrumentOverview_InstrumentTheAccountNeverHeld_ReturnsNotFound(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")
	// FPS.US exists in the database, but only on the other Account.
	srv.importAccount(t, adr0002ExampleA(), "XTB USD 2")

	for _, ticker := range []string{"FPS.US", "NONE.US"} {
		status, _, body := srv.get(t, overviewPath(accountID, ticker))
		assert.Equal(t, http.StatusNotFound, status, "ticker %s, body %q", ticker, body)
	}
}

func TestInstrumentOverview_Request_StartsNoPriceRefresh(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	srv := startServerWithPrices(t, provider)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")

	srv.overview(t, accountID, "ABC.US")

	assert.Empty(t, provider.requestedTickers())
	assert.Empty(t, provider.historyRequests())
}

// holdingRows returns the rows of GET /api/accounts/{id}/holdings as raw JSON, by ticker.
func (s *testServer) holdingRows(t *testing.T, accountID int64) map[string]json.RawMessage {
	t.Helper()

	var rows []json.RawMessage
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &rows)
	require.Equal(t, http.StatusOK, status)
	byTicker := map[string]json.RawMessage{}
	for _, row := range rows {
		var id struct {
			Ticker string `json:"ticker"`
		}
		require.NoError(t, json.Unmarshal(row, &id))
		byTicker[id.Ticker] = row
	}
	return byTicker
}

func TestInstrumentOverview_OpenHolding_HoldingEqualsTheHoldingsRow(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")
	srv.storeDailyCloses(t, "ABC.US", "USD", "2026-02-03=101", "2026-02-04=110")

	got := srv.overview(t, accountID, "ABC.US")

	row, ok := srv.holdingRows(t, accountID)["ABC.US"]
	require.True(t, ok)
	require.Contains(t, string(row), `"value":"220.00"`, "the Holding has a value, so the test compares real values")
	assert.JSONEq(t, string(row), string(got.Holding))
}

func TestInstrumentOverview_HoldingWithNoPrice_HoldingEqualsTheHoldingsRow(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")

	got := srv.overview(t, accountID, "ABC.US")

	row := srv.holdingRows(t, accountID)["ABC.US"]
	require.Contains(t, string(row), `"note":"There is no price`)
	assert.JSONEq(t, string(row), string(got.Holding))
}

func TestInstrumentOverview_OpenLot_GivesVolumeCostOpenTimePriceAndFills(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// ADR 0002 example A: one Lot 2860602916 with two Fills, volume 1.27, purchase amount 49.87.
	accountID := srv.importAccount(t, adr0002ExampleA(), "XTB USD")

	got := srv.overview(t, accountID, "FPS.US")

	require.Len(t, got.OpenLots, 1)
	lot := got.OpenLots[0]
	assert.Equal(t, "2860602916", lot.PositionID)
	assert.Equal(t, "1.27", lot.Volume)
	assert.Equal(t, "49.87", lot.Cost)
	assert.Equal(t, "2026-02-03T09:30:00Z", lot.OpenTime)
	// (0.27 × 39.37 + 1 × 39.24) / 1.27
	require.NotNil(t, lot.OpenPrice)
	assert.Equal(t, "39.2676377952755906", *lot.OpenPrice)
	assert.Equal(t, got.Fills, lot.Fills, "the Lot has the Fills of the Instrument")
}

func TestInstrumentOverview_LotClosedInPart_OpenLotHasRemainingVolumeAndCostOfThatPart(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// ADR 0001 example 2: open volume 0.1134, cost 185.31 × 0.1134 / 0.2638 ≈ 79.66.
	accountID := srv.importAccount(t, adrExample2(), "XTB PLN")

	got := srv.overview(t, accountID, "XYZ.US")

	require.Len(t, got.OpenLots, 1)
	assert.Equal(t, "2026-02-02T10:00:00Z", got.OpenLots[0].OpenTime)
	assert.Equal(t, "1349972184", got.OpenLots[0].PositionID)
	assert.Equal(t, "0.1134", got.OpenLots[0].Volume)
	assert.Equal(t, "79.66", cents(t, got.OpenLots[0].Cost))
	require.Len(t, got.OpenLots[0].Fills, 1, "the Lot keeps its Fills")
	assert.Equal(t, "0.2638", got.OpenLots[0].Fills[0].Volume)
	var holding struct {
		Volume string `json:"volume"`
		Cost   string `json:"cost"`
	}
	require.NoError(t, json.Unmarshal(got.Holding, &holding))
	assert.Equal(t, "0.1134", holding.Volume)
	assert.Equal(t, got.OpenLots[0].Cost, holding.Cost)
}

func TestInstrumentOverview_FullySold_HasNullHoldingAndNoOpenLots(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := overviewExport()
	export.CashOperations = append([]xtbtest.CashOperation{
		{Type: "Stock sell", Ticker: "ABC.US", Instrument: "ABC Inc", Time: closeTime(5), Amount: "220.00", ID: "903", Comment: "CLOSE BUY 2/2 @ 110.00", PositionID: "9000000001"},
	}, export.CashOperations...)
	export.OpenLots = nil
	export.ClosedPositions = []xtbtest.ClosedPosition{
		{Ticker: "ABC.US", Instrument: "ABC Inc", Volume: "2", OpenTime: opTime(3), CloseTime: closeTime(5), ProfitLoss: "20.00", PositionID: "9000000001"},
	}
	accountID := srv.importAccount(t, export, "XTB USD")

	status, _, body := srv.get(t, overviewPath(accountID, "ABC.US"))

	require.Equal(t, http.StatusOK, status, "body %q", body)
	assert.Contains(t, body, `"holding":null`)
	assert.Contains(t, body, `"openLots":[]`, "the open Lots are an empty array, not null")
}

// closedPositionRows returns the items of GET /api/accounts/{id}/closed-positions of the Instrument as raw JSON.
func (s *testServer) closedPositionRows(t *testing.T, accountID int64, ticker string) []json.RawMessage {
	t.Helper()

	var rows []json.RawMessage
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/closed-positions", accountID), &rows)
	require.Equal(t, http.StatusOK, status)
	got := []json.RawMessage{}
	for _, row := range rows {
		var id struct {
			Ticker string `json:"ticker"`
		}
		require.NoError(t, json.Unmarshal(row, &id))
		if id.Ticker == ticker {
			got = append(got, row)
		}
	}
	return got
}

// assertSameJSON checks that the two lists hold the same JSON items in the same order.
func assertSameJSON(t *testing.T, want, got []json.RawMessage) {
	t.Helper()

	require.Len(t, got, len(want))
	for i := range want {
		assert.JSONEq(t, string(want[i]), string(got[i]), "item %d", i)
	}
}

func TestInstrumentOverview_LotClosedInParts_ClosedPositionsEqualTheEndpointItems(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// ADR 0001 example 1: one Lot closed in two parts on the same day.
	accountID := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")

	got := srv.overview(t, accountID, "ABC.PL")

	want := srv.closedPositionRows(t, accountID, "ABC.PL")
	require.Len(t, want, 2)
	assertSameJSON(t, want, got.ClosedPositions)
	for _, item := range got.ClosedPositions {
		var ids struct {
			XTBID      string `json:"xtbId"`
			PositionID string `json:"positionId"`
		}
		require.NoError(t, json.Unmarshal(item, &ids))
		assert.Equal(t, "2636532207", ids.PositionID, "the item has the ID of its Lot")
		assert.NotEmpty(t, ids.XTBID)
	}
}

func TestInstrumentOverview_FullySold_HasClosedPositionsAndPricesAfterTheLastSell(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")
	srv.storeDailyCloses(t, "ABC.PL", "PLN", "2026-03-02=10.5", "2026-03-03=10.7", "2026-03-04=10.9")

	got := srv.overview(t, accountID, "ABC.PL")

	assert.Equal(t, "null", string(got.Holding))
	assertSameJSON(t, srv.closedPositionRows(t, accountID, "ABC.PL"), got.ClosedPositions)
	require.Len(t, got.ClosedPositions, 2)
	require.Len(t, got.Prices.Series, 3)
	assert.Equal(t, "2026-03-04", got.Prices.Series[2].Date, "the series continues after the last sell on 2026-03-02")
}

func TestInstrumentOverview_NoClosedPositions_GivesEmptyArray(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, overviewExport(), "XTB USD")

	_, _, body := srv.get(t, overviewPath(accountID, "ABC.US"))

	assert.Contains(t, body, `"closedPositions":[]`, "the Closed Positions are an empty array, not null")
}

func TestInstrumentOverview_SameInstrumentOnTwoAccounts_EachAccountGivesItsOwnClosedPositions(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.importAccount(t, adrExample1("-6.12", "-0.66"), "XTB PLN")
	other := adrExample1("-6.12", "-0.66")
	other.AccountNumber = "50000099"
	for i := range other.CashOperations {
		other.CashOperations[i].ID += "9"
		other.CashOperations[i].PositionID = map[bool]string{true: "", false: "2636599999"}[other.CashOperations[i].PositionID == ""]
	}
	for i := range other.ClosedPositions {
		other.ClosedPositions[i].PositionID = "2636599999"
	}
	// The second Account sells only one part of its Lot.
	other.CashOperations = append(other.CashOperations[:0:0], other.CashOperations[1:]...)
	other.ClosedPositions = other.ClosedPositions[1:]
	second := srv.importAccount(t, other, "XTB PLN 2")

	firstGot := srv.overview(t, first, "ABC.PL")
	secondGot := srv.overview(t, second, "ABC.PL")

	assertSameJSON(t, srv.closedPositionRows(t, first, "ABC.PL"), firstGot.ClosedPositions)
	assertSameJSON(t, srv.closedPositionRows(t, second, "ABC.PL"), secondGot.ClosedPositions)
	assert.Len(t, firstGot.ClosedPositions, 2)
	assert.Len(t, secondGot.ClosedPositions, 1)
}
