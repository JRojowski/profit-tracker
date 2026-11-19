package httpapi_test

import (
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
	}, got)
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
