package httpapi_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// refreshResponse is the body of POST /api/prices/refresh.
type refreshResponse struct {
	Refreshed int              `json:"refreshed"`
	Failures  []refreshFailure `json:"failures"`
}

// refreshFailure is one Instrument that got no price in a refresh.
type refreshFailure struct {
	Ticker string `json:"ticker"`
	Error  string `json:"error"`
}

// summaryPrices are the price fields of GET /api/accounts/{id}/summary, with the total result.
type summaryPrices struct {
	Cash             string  `json:"cash"`
	Value            string  `json:"value"`
	UnrealizedProfit string  `json:"unrealizedProfit"`
	TotalResult      string  `json:"totalResult"`
	PriceDate        *string `json:"priceDate"`
	StalePrices      bool    `json:"stalePrices"`
	UnpricedHoldings int     `json:"unpricedHoldings"`
}

// holdingPrices are the price fields of one row of GET /api/accounts/{id}/holdings.
type holdingPrices struct {
	Ticker                  string  `json:"ticker"`
	QuoteCurrency           *string `json:"quoteCurrency"`
	Price                   *string `json:"price"`
	PriceDate               *string `json:"priceDate"`
	Value                   *string `json:"value"`
	UnrealizedProfit        *string `json:"unrealizedProfit"`
	UnrealizedProfitPercent *string `json:"unrealizedProfitPercent"`
	Note                    *string `json:"note"`
}

// pricedExport is an export of a USD Account with two Holdings in USD, and the extra purchases.
//
//	Cash:    1000.00 - 200.00 - 60.00 = 740.00, minus the extra purchases
//	ABC.US:  volume 2,   cost 200.00
//	XYZ.US:  volume 0.5, cost 60.00
func pricedExport(extra ...xtbtest.CashOperation) xtbtest.Export {
	ops := append([]xtbtest.CashOperation(nil), extra...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "XYZ.US", Instrument: "XYZ", Amount: "-60.00", ID: "603", Comment: "OPEN BUY 0.5 @ 120.00", PositionID: "6000000002"},
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "ABC.US", Instrument: "ABC", Amount: "-200.00", ID: "602", Comment: "OPEN BUY 2 @ 100.00", PositionID: "6000000001"},
		xtbtest.CashOperation{Type: "Deposit", Amount: "1000.00", ID: "601", Comment: "Blik deposit"},
	)
	return xtbtest.Export{AccountNumber: "56000010", Currency: "USD", CashOperations: ops}
}

// refreshPrices sends POST /api/prices/refresh and returns the response. It stops the test if the status is not 200.
func (s *testServer) refreshPrices(t *testing.T) refreshResponse {
	t.Helper()

	status, body := s.postJSON(t, "/api/prices/refresh", nil)
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got refreshResponse
	decode(t, body, &got)
	return got
}

// summaryPrices returns the price fields of the summary of the Account.
func (s *testServer) summaryPrices(t *testing.T, accountID int64) summaryPrices {
	t.Helper()

	var got summaryPrices
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// holdingPrices returns the price fields of the Holdings of the Account.
func (s *testServer) holdingPrices(t *testing.T, accountID int64) []holdingPrices {
	t.Helper()

	var got []holdingPrices
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

// noValue is a Holding row with no value and the note.
func noValue(ticker string, quoteCurrency *string, note string) holdingPrices {
	return holdingPrices{Ticker: ticker, QuoteCurrency: quoteCurrency, Note: ptr(note)}
}

const noPriceNote = "There is no price for the Instrument. Refresh the prices."

const unknownQuoteCurrencyNote = "The quote currency of the Instrument is unknown. Set it in the price source of the Instrument."

// TestRefreshPrices_HoldingsInAccountCurrency_ReturnsValueAndUnrealizedProfit checks the values after a refresh.
//
//	ABC.US: value 2 × 125.50 = 251.00, Unrealized Profit 251.00 - 200.00 = 51.00, 51.00 × 100 / 200.00 = 25.5 %
//	XYZ.US: value 0.5 × 110.00 = 55.00, Unrealized Profit 55.00 - 60.00 = -5.00, -5.00 × 100 / 60.00 = -8.33… %
//	Value:  740.00 + 251.00 + 55.00 = 1046.00
//	Unrealized Profit: 51.00 - 5.00 = 46.00, which is also the total result
//	Price date: the oldest price date, 2026-10-21
func TestRefreshPrices_HoldingsInAccountCurrency_ReturnsValueAndUnrealizedProfit(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-21")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 2, Failures: []refreshFailure{}}, refresh)
	assert.Equal(t, summaryPrices{
		Cash: "740.00", Value: "1046.00", UnrealizedProfit: "46.00", TotalResult: "46.00",
		PriceDate: ptr("2026-10-21"), StalePrices: false, UnpricedHoldings: 0,
	}, srv.summaryPrices(t, id))
	assert.Equal(t, []holdingPrices{
		{
			Ticker: "ABC.US", QuoteCurrency: ptr("USD"), Price: ptr("125.5"), PriceDate: ptr("2026-10-22"),
			Value: ptr("251.00"), UnrealizedProfit: ptr("51.00"), UnrealizedProfitPercent: ptr("25.5"),
		},
		{
			Ticker: "XYZ.US", QuoteCurrency: ptr("USD"), Price: ptr("110"), PriceDate: ptr("2026-10-21"),
			Value: ptr("55.00"), UnrealizedProfit: ptr("-5.00"), UnrealizedProfitPercent: ptr("-8.3333333333333333"),
		},
	}, srv.holdingPrices(t, id))
}

func TestAccountSummary_NoPrices_ShowsCashAsValueAndStaleWarning(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	assert.Equal(t, summaryPrices{
		Cash: "740.00", Value: "740.00", UnrealizedProfit: "0.00", TotalResult: "0.00",
		PriceDate: nil, StalePrices: true, UnpricedHoldings: 2,
	}, srv.summaryPrices(t, id))
	assert.Equal(t, []holdingPrices{
		noValue("ABC.US", ptr("USD"), noPriceNote),
		noValue("XYZ.US", ptr("USD"), noPriceNote),
	}, srv.holdingPrices(t, id))
}

func TestRefreshPrices_PriceInOtherCurrency_RejectsThePrice(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "EUR", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 1, Failures: []refreshFailure{
		{Ticker: "ABC.US", Error: "the price is in EUR, but the Instrument is quoted in USD"},
	}}, refresh)
	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 2)
	assert.Equal(t, noValue("ABC.US", ptr("USD"), noPriceNote), holdings[0], "the tool must store nothing for a rejected price")
	assert.Equal(t, ptr("55.00"), holdings[1].Value)
	summary := srv.summaryPrices(t, id)
	assert.Equal(t, "795.00", summary.Value)
	assert.Equal(t, 1, summary.UnpricedHoldings)
	assert.True(t, summary.StalePrices)
}

func TestRefreshPrices_ProviderFailsNextDay_KeepsLastPricesWithStaleWarning(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)
	srv.clock.set(testToday.AddDate(0, 0, 1))
	provider.failWith(errors.New("the price source does not respond"))

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 0, Failures: []refreshFailure{
		{Ticker: "ABC.US", Error: "the price source does not respond"},
		{Ticker: "XYZ.US", Error: "the price source does not respond"},
	}}, refresh)
	assert.Equal(t, summaryPrices{
		Cash: "740.00", Value: "1046.00", UnrealizedProfit: "46.00", TotalResult: "46.00",
		PriceDate: ptr("2026-10-22"), StalePrices: true, UnpricedHoldings: 0,
	}, srv.summaryPrices(t, id))
}

func TestRefreshPrices_SameDayAgain_IsNotStale(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)
	// The next day the price date stays the same, for example on a holiday. A new fetch makes the price current.
	srv.clock.set(testToday.AddDate(0, 0, 1))
	provider.set(t, "ABC", "126.00", "USD", "2026-10-22")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, 2, refresh.Refreshed)
	summary := srv.summaryPrices(t, id)
	assert.False(t, summary.StalePrices)
	assert.Equal(t, "1047.00", summary.Value, "the new close of the same price date replaces the old one")
}

func TestAccountHoldings_OtherOrUnknownQuoteCurrency_ShowsNoValueAndNote(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	provider.set(t, "DEF.DE", "40.00", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "GHI.FR", Instrument: "GHI", Amount: "-30.00", ID: "605", Comment: "OPEN BUY 1 @ 30.00", PositionID: "6000000004"},
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "DEF.DE", Instrument: "DEF", Amount: "-50.00", ID: "604", Comment: "OPEN BUY 1 @ 50.00", PositionID: "6000000003"},
	), "XTB USD")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 3, Failures: []refreshFailure{}}, refresh,
		"the refresh stores the EUR price of DEF.DE and skips GHI.FR with no quote currency")
	assert.NotContains(t, provider.requestedTickers(), "GHI.FR")
	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 4)
	assert.Equal(t, noValue("DEF.DE", ptr("EUR"), "The Instrument is quoted in EUR. The tool shows no value in USD."), holdings[1])
	assert.Equal(t, noValue("GHI.FR", nil, unknownQuoteCurrencyNote), holdings[2])
	assert.Equal(t, summaryPrices{
		Cash: "660.00", Value: "966.00", UnrealizedProfit: "46.00", TotalResult: "46.00",
		PriceDate: ptr("2026-10-22"), StalePrices: false, UnpricedHoldings: 2,
	}, srv.summaryPrices(t, id), "a Holding in another quote currency does not make the prices stale")
}

func TestRefreshPrices_DefaultTickerMapping_AsksForPriceTickers(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, xtbtest.Export{
		AccountNumber: "56000011",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "PKO.PL", Amount: "-10.00", ID: "704", Comment: "OPEN BUY 1 @ 10.00", PositionID: "7000000004"},
			{Type: "Stock purchase", Ticker: "VUSA.UK", Amount: "-10.00", ID: "703", Comment: "OPEN BUY 1 @ 10.00", PositionID: "7000000003"},
			{Type: "Stock purchase", Ticker: "SAP.DE", Amount: "-10.00", ID: "702", Comment: "OPEN BUY 1 @ 10.00", PositionID: "7000000002"},
			{Type: "Stock purchase", Ticker: "AAPL.US", Amount: "-10.00", ID: "701", Comment: "OPEN BUY 1 @ 10.00", PositionID: "7000000001"},
		},
	}, "XTB USD")

	srv.refreshPrices(t)

	assert.ElementsMatch(t, []string{"AAPL", "SAP.DE", "VUSA.L", "PKO.WA"}, provider.requestedTickers())
}

func TestRefreshPrices_NoHoldings_RefreshesNothing(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, depositOnlyExport("56000001", "USD"), "XTB USD")

	refresh := srv.refreshPrices(t)

	assert.Equal(t, refreshResponse{Refreshed: 0, Failures: []refreshFailure{}}, refresh)
	assert.Empty(t, provider.requestedTickers())
}

func TestRefreshAtStart_HeldInstrumentWithNoPriceToday_RunsAgainUntilAllAreCurrent(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(
		// GHI.FR has no quote currency. It never gets a price, and it must not start a refresh.
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "GHI.FR", Instrument: "GHI", Amount: "-30.00", ID: "605", Comment: "OPEN BUY 1 @ 30.00", PositionID: "6000000004"},
	), "XTB USD")
	// The first start of the day stores the price of ABC.US. XYZ.US fails.
	result, ran, err := srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	require.True(t, ran)
	require.Equal(t, 1, result.Refreshed)

	// A later start of the same day must try again, because XYZ.US has no price fetched today.
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv.clock.set(testToday.Add(time.Hour))
	result, ran, err = srv.prices.RefreshIfStale(t.Context())

	require.NoError(t, err)
	assert.True(t, ran, "XYZ.US has no price fetched today")
	assert.Equal(t, 2, result.Refreshed)
	assert.Equal(t, 1, srv.summaryPrices(t, id).UnpricedHoldings, "only GHI.FR stays with no value")

	// Now each held Instrument with a quote currency has a price fetched today.
	_, ran, err = srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.False(t, ran, "all prices are current")
	assert.Len(t, provider.requestedTickers(), 4)
}

func TestRefreshPrices_ContextEnds_StopsAndKeepsStoredPrices(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "DEF.DE", "40.00", "EUR", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "DEF.DE", Instrument: "DEF", Amount: "-50.00", ID: "604", Comment: "OPEN BUY 1 @ 50.00", PositionID: "6000000003"},
	), "XTB USD")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// The context ends during the request of the second Instrument, for example at shutdown.
	provider.setOnRequest(func(ctx context.Context, ticker string) error {
		if ticker != "DEF.DE" {
			return nil
		}
		cancel()
		return ctx.Err()
	})

	result, err := srv.prices.Refresh(ctx)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, result.Refreshed)
	assert.Empty(t, result.Failures, "a cancelled request is not a failure of the Instrument")
	assert.Equal(t, []string{"ABC", "DEF.DE"}, provider.requestedTickers(), "the refresh must stop")
	assert.Equal(t, ptr("251.00"), srv.holdingPrices(t, id)[0].Value, "the price stored before the end stays")
}

func TestRefreshPrices_DuringOtherRefresh_StopsWaitingWhenContextEnds(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, pricedExport(), "XTB USD")
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	provider.setOnRequest(func(context.Context, string) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	})
	// The refresh at start runs and waits for the price source.
	startDone := make(chan error, 1)
	go func() {
		_, _, err := srv.prices.RefreshIfStale(context.WithoutCancel(t.Context()))
		startDone <- err
	}()
	<-started

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := srv.prices.Refresh(ctx)

	require.ErrorIs(t, err, context.DeadlineExceeded, "the refresh must stop waiting when its context ends")
	assert.Equal(t, []string{"ABC"}, provider.requestedTickers(), "the waiting refresh must not ask for prices")
	close(release)
	require.NoError(t, <-startDone)
}

func TestRefreshAtStart_RunsOnlyWhenNoPriceWasFetchedToday(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	_, ran, err := srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.True(t, ran, "no price was fetched today")
	assert.Len(t, provider.requestedTickers(), 2)
	assert.Equal(t, "1046.00", srv.summaryPrices(t, id).Value)

	// Late in the same day in UTC, the stored prices are current.
	srv.clock.set(time.Date(2026, 10, 23, 23, 59, 0, 0, time.UTC))
	_, ran, err = srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.False(t, ran, "a price was fetched today")
	assert.Len(t, provider.requestedTickers(), 2)

	srv.clock.set(time.Date(2026, 10, 24, 0, 1, 0, 0, time.UTC))
	_, ran, err = srv.prices.RefreshIfStale(t.Context())
	require.NoError(t, err)
	assert.True(t, ran, "the stored prices are from yesterday")
	assert.Len(t, provider.requestedTickers(), 4)
}
