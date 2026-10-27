package httpapi_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// priceWarning is one Price Warning in the API responses.
type priceWarning struct {
	Ticker              string  `json:"ticker"`
	Name                string  `json:"name"`
	Kind                string  `json:"kind"`
	SourceCurrency      *string `json:"sourceCurrency"`
	MedianDifference    *string `json:"medianDifference"`
	MaxDifference       *string `json:"maxDifference"`
	Confirmable         bool    `json:"confirmable"`
	PriceTicker         string  `json:"priceTicker"`
	PriceTickerManual   bool    `json:"priceTickerManual"`
	QuoteCurrency       *string `json:"quoteCurrency"`
	QuoteCurrencyManual bool    `json:"quoteCurrencyManual"`
	PriceConfirmed      bool    `json:"priceConfirmed"`
}

// warningsRefresh is the body of POST /api/prices/refresh with the Price Warnings.
type warningsRefresh struct {
	PriceWarnings []priceWarning `json:"priceWarnings"`
}

// holdingWarnings is one row of GET /api/accounts/{id}/holdings with its Price Warnings.
type holdingWarnings struct {
	Ticker         string         `json:"ticker"`
	PriceConfirmed bool           `json:"priceConfirmed"`
	PriceWarnings  []priceWarning `json:"priceWarnings"`
}

// historyWarnings is the body of a value-history endpoint with its Price Warnings and its points.
type historyWarnings struct {
	Points        []valuePoint   `json:"points"`
	PriceWarnings []priceWarning `json:"priceWarnings"`
}

// confirmedInstrument is the body of PUT /api/instruments/{ticker}/price-confirmation.
type confirmedInstrument struct {
	instrumentResponse
	PriceConfirmed bool `json:"priceConfirmed"`
}

// refreshWarnings sends POST /api/prices/refresh and returns its Price Warnings.
func (s *testServer) refreshWarnings(t *testing.T) []priceWarning {
	t.Helper()

	status, body := s.postJSON(t, "/api/prices/refresh", nil)
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got warningsRefresh
	decode(t, body, &got)
	require.NotNil(t, got.PriceWarnings, "the list is empty, not null")
	return got.PriceWarnings
}

// holdingWarnings returns the Holdings of the Account with their Price Warnings.
func (s *testServer) holdingWarnings(t *testing.T, accountID int64) []holdingWarnings {
	t.Helper()

	var got []holdingWarnings
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	for _, h := range got {
		require.NotNil(t, h.PriceWarnings, "the list of %s is empty, not null", h.Ticker)
	}
	return got
}

// historyWarnings returns the body of the value-history endpoint at path.
func (s *testServer) historyWarnings(t *testing.T, path string) historyWarnings {
	t.Helper()

	var got historyWarnings
	status := s.getJSON(t, path, &got)
	require.Equal(t, http.StatusOK, status)
	require.NotNil(t, got.PriceWarnings, "the list is empty, not null")
	return got
}

// accountWarnings returns the Price Warnings of GET /api/accounts/{id}/value-history.
func (s *testServer) accountWarnings(t *testing.T, accountID int64) []priceWarning {
	t.Helper()
	return s.historyWarnings(t, fmt.Sprintf("/api/accounts/%d/value-history", accountID)).PriceWarnings
}

// confirmPrice sends PUT /api/instruments/{ticker}/price-confirmation and returns the Instrument.
func (s *testServer) confirmPrice(t *testing.T, ticker string, confirmed bool) confirmedInstrument {
	t.Helper()

	status, body := s.putJSON(t, "/api/instruments/"+ticker+"/price-confirmation", map[string]bool{"confirmed": confirmed})
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got confirmedInstrument
	decode(t, body, &got)
	return got
}

// kinds returns the ticker and the kind of each Price Warning, as "TICKER kind".
func kinds(warnings []priceWarning) []string {
	got := make([]string, 0, len(warnings))
	for _, w := range warnings {
		got = append(got, w.Ticker+" "+w.Kind)
	}
	return got
}

// tradeExport is a USD Account that buys 1 MIS.US on 2026-10-19, 2026-10-20 and 2026-10-21, one Lot each,
// at the XTB prices in order.
func tradeExport(xtbPrices ...string) xtbtest.Export {
	ops := []xtbtest.CashOperation{cashDeposit("1401", "10000.00", historyTime(1))}
	for i, p := range xtbPrices {
		ops = append(ops, xtbtest.CashOperation{
			Type: "Stock purchase", Ticker: "MIS.US", Instrument: "Mismatch Inc", Time: historyTime(19 + i),
			Amount: "-" + p, ID: fmt.Sprintf("141%d", i), Comment: "OPEN BUY 1 @ " + p, PositionID: fmt.Sprintf("1400000000%d", i),
		})
	}
	return xtbtest.Export{AccountNumber: "59000001", Currency: "USD", CashOperations: ops}
}

// misProvider returns a fake price provider with the closes of MIS on 2026-10-19, 2026-10-20 and 2026-10-21,
// in USD, and a current price on 2026-10-22.
func misProvider(t *testing.T, closes ...string) *fakeProvider {
	t.Helper()
	provider := newFakeProvider()
	days := []string{"2026-10-19", "2026-10-20", "2026-10-21"}
	rows := make([]string, 0, len(closes))
	for i, c := range closes {
		rows = append(rows, days[i]+"="+c)
	}
	provider.setHistory(t, "MIS", "USD", rows...)
	provider.set(t, "MIS", "100", "USD", "2026-10-22")
	return provider
}

// mismatch returns the price mismatch warning of MIS.US with the differences.
func mismatch(median, largest string) priceWarning {
	return priceWarning{
		Ticker: "MIS.US", Name: "Mismatch Inc", Kind: "priceMismatch",
		MedianDifference: ptr(median), MaxDifference: ptr(largest), Confirmable: true,
		PriceTicker: "MIS", QuoteCurrency: ptr("USD"),
	}
}

func TestPriceWarnings_PriceCheckExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		closes []string
		want   []priceWarning
	}{
		// Spec #27: differences of 1%, 2% and 3% give no warning.
		{name: "differences of 1, 2 and 3 percent", closes: []string{"101", "98", "103"}, want: []priceWarning{}},
		// Spec #27: a median difference of 8% gives a warning.
		{name: "median difference of 8 percent", closes: []string{"108", "92", "110"}, want: []priceWarning{mismatch("8", "10")}},
		{name: "one difference of more than 30 percent", closes: []string{"100", "100", "131"}, want: []priceWarning{mismatch("0", "31")}},
		{name: "one difference of exactly 30 percent", closes: []string{"100", "100", "130"}, want: []priceWarning{}},
		{name: "median difference of exactly 5 percent", closes: []string{"105", "95", "105"}, want: []priceWarning{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServerWithPrices(t, misProvider(t, tc.closes...))
			id := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")

			got := srv.refreshWarnings(t)

			assert.Equal(t, tc.want, got)
			holdings := srv.holdingWarnings(t, id)
			require.Len(t, holdings, 1)
			assert.Equal(t, tc.want, holdings[0].PriceWarnings)
			assert.Equal(t, tc.want, srv.accountWarnings(t, id))
			assert.Equal(t, tc.want, srv.historyWarnings(t, "/api/all-accounts/value-history").PriceWarnings)
		})
	}
}

func TestPriceWarnings_EvenNumberOfDifferences_MedianIsMeanOfTheMiddleTwo(t *testing.T) {
	t.Parallel()
	// The differences are 4% and 8.125%. The median is 6.0625%. The frontend rounds for display.
	srv := startServerWithPrices(t, misProvider(t, "104", "108.125"))
	srv.importAccount(t, tradeExport("100", "100"), "XTB USD")

	assert.Equal(t, []priceWarning{mismatch("6.0625", "8.125")}, srv.refreshWarnings(t))
	status, body := srv.postJSON(t, "/api/prices/refresh", nil)
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, body, `"message"`, "the frontend builds the sentence")
}

func TestPriceWarnings_DayWithNoClose_IsSkipped(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	// 2026-10-20 is a holiday with no close. Its Fill at 200 is not compared.
	provider.setHistory(t, "MIS", "USD", "2026-10-19=101", "2026-10-21=102")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, tradeExport("100", "200", "100"), "XTB USD")

	assert.Empty(t, srv.refreshWarnings(t))
}

func TestPriceWarnings_SoldInstrument_IsChecked(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	// ABC.US: bought at 100.00 on 2026-10-19, sold at 105.00 on 2026-10-21. The closes are about 20% too high.
	provider.setHistory(t, "ABC", "USD", "2026-10-19=120", "2026-10-20=123", "2026-10-21=126")
	provider.setHistory(t, "XYZ", "USD", "2026-10-20=50.5", "2026-10-21=51")
	provider.set(t, "XYZ", "52", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, historyExport(), "XTB USD")

	refresh := srv.refreshWarnings(t)

	want := []priceWarning{{
		Ticker: "ABC.US", Name: "ABC", Kind: "priceMismatch",
		MedianDifference: ptr("20"), MaxDifference: ptr("20"), Confirmable: true, PriceTicker: "ABC", QuoteCurrency: ptr("USD"),
	}}
	assert.Equal(t, want, refresh)
	assert.Equal(t, want, srv.accountWarnings(t, id), "the account value history lists a sold Instrument")
	assert.Equal(t, want, srv.historyWarnings(t, "/api/all-accounts/value-history").PriceWarnings)
	holdings := srv.holdingWarnings(t, id)
	require.Len(t, holdings, 1)
	assert.Equal(t, "XYZ.US", holdings[0].Ticker)
	assert.Empty(t, holdings[0].PriceWarnings)
}

func TestPriceWarnings_NoData_GivesNoDataWarning(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("MIS", fmt.Errorf("yahoo daily prices of MIS: status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, tradeExport("100"), "XTB USD")

	got := srv.refreshWarnings(t)

	want := []priceWarning{{
		Ticker: "MIS.US", Name: "Mismatch Inc", Kind: "noData",
		PriceTicker: "MIS", QuoteCurrency: ptr("USD"),
	}}
	assert.Equal(t, want, got)
	holdings := srv.holdingWarnings(t, id)
	require.Len(t, holdings, 1)
	assert.Equal(t, want, holdings[0].PriceWarnings)
	assert.Equal(t, want, srv.accountWarnings(t, id))
}

func TestPriceWarnings_NoDataWithStoredCurrentPrice_GivesNoWarning(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "MIS", "100", "USD", "2026-10-22")
	provider.failHistory("MIS", fmt.Errorf("status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, tradeExport("100"), "XTB USD")

	assert.Empty(t, srv.refreshWarnings(t), "the tool stores a close of the Instrument")
}

func TestPriceWarnings_DelistedInstrumentWithStoredHistory_GivesNoNoDataWarning(t *testing.T) {
	t.Parallel()
	provider := misProvider(t, "100", "100", "100")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	srv.refreshWarnings(t)

	srv.clock.set(testToday.AddDate(0, 0, 2))
	provider.failHistory("MIS", fmt.Errorf("status 404: %w", price.ErrNoData))
	provider.failWith(fmt.Errorf("status 404: %w", price.ErrNoData))

	assert.Empty(t, srv.refreshWarnings(t), "the stored closes are the only copy of the history of a delisted Instrument")
}

func TestPriceWarnings_LaterAnswerWithData_RemovesNoData(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("MIS", fmt.Errorf("status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, tradeExport("100"), "XTB USD")
	require.Equal(t, []string{"MIS.US noData"}, kinds(srv.refreshWarnings(t)))

	srv.clock.set(testToday.AddDate(0, 0, 1))
	provider.failHistory("MIS", nil)
	provider.setHistory(t, "MIS", "USD", "2026-10-19=100", "2026-10-23=101")

	assert.Empty(t, srv.refreshWarnings(t))
}

func TestPriceWarnings_DifferentCurrency_NamesBothCurrencies(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "MIS", "EUR", "2026-10-19=92", "2026-10-20=92")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, tradeExport("100", "100"), "XTB USD")

	got := srv.refreshWarnings(t)

	want := []priceWarning{{
		Ticker: "MIS.US", Name: "Mismatch Inc", Kind: "differentCurrency",
		SourceCurrency: ptr("EUR"), PriceTicker: "MIS", QuoteCurrency: ptr("USD"),
	}}
	assert.Equal(t, want, got)
	assert.Equal(t, want, srv.accountWarnings(t, id))
}

func TestPriceWarnings_CurrentPriceInDifferentCurrency_GivesDifferentCurrency(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "MIS", "92", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	srv.importAccount(t, tradeExport("100"), "XTB USD")

	got := srv.refreshWarnings(t)

	require.Len(t, got, 1)
	assert.Equal(t, "differentCurrency", got[0].Kind)
	assert.Equal(t, ptr("EUR"), got[0].SourceCurrency)
}

func TestPriceWarnings_FixedQuoteCurrency_RemovesDifferentCurrency(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.setHistory(t, "MIS", "EUR", "2026-10-19=100")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, tradeExport("100"), "XTB USD")
	require.Equal(t, []string{"MIS.US differentCurrency"}, kinds(srv.refreshWarnings(t)))

	srv.setQuoteCurrency(t, "MIS.US", "EUR")

	assert.Empty(t, srv.accountWarnings(t, id))
}

func TestPriceWarnings_UnknownQuoteCurrency_GivesNoWarning(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("ODD.XX", fmt.Errorf("status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, xtbtest.Export{AccountNumber: "59000002", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ODD.XX", Instrument: "Odd", Time: historyTime(19), Amount: "-10.00", ID: "1502", Comment: "OPEN BUY 1 @ 10.00", PositionID: "15000000001"},
		cashDeposit("1501", "100.00", historyTime(1)),
	}}, "XTB USD")

	assert.Empty(t, srv.refreshWarnings(t))
	assert.Empty(t, srv.holdingWarnings(t, id)[0].PriceWarnings)
}

// startMixedWarnings starts a server with a USD Account that holds AAA.US and MIS.US, and runs two refreshes.
//
//	AAA.US: the price source has no data.
//	MIS.US: the first refresh stores a close of 150.00 in USD for the Fill at 100.00.
//	        The refresh of the next day gets a current price in EUR.
func startMixedWarnings(t *testing.T) (*testServer, int64) {
	t.Helper()
	provider := newFakeProvider()
	provider.failHistory("AAA", fmt.Errorf("status 404: %w", price.ErrNoData))
	provider.setHistory(t, "MIS", "USD", "2026-10-19=150")
	srv := startServerWithPrices(t, provider)
	export := tradeExport("100")
	export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{
		Type: "Stock purchase", Ticker: "AAA.US", Instrument: "AAA", Time: historyTime(20), Amount: "-10.00", ID: "1499", Comment: "OPEN BUY 1 @ 10.00", PositionID: "14000000009",
	})
	id := srv.importAccount(t, export, "XTB USD")
	srv.refreshWarnings(t)

	srv.clock.set(testToday.AddDate(0, 0, 1))
	provider.set(t, "MIS", "92", "EUR", "2026-10-23")
	return srv, id
}

func TestPriceWarnings_Order_IsTickerThenKind(t *testing.T) {
	t.Parallel()
	srv, _ := startMixedWarnings(t)

	got := srv.refreshWarnings(t)

	assert.Equal(t, []string{"AAA.US noData", "MIS.US differentCurrency", "MIS.US priceMismatch"}, kinds(got))
}

func TestPriceConfirmation_RemovesOnlyPriceMismatch(t *testing.T) {
	t.Parallel()
	srv, id := startMixedWarnings(t)
	srv.refreshWarnings(t)

	mis := srv.confirmPrice(t, "MIS.US", true)
	aaa := srv.confirmPrice(t, "AAA.US", true)

	assert.Equal(t, confirmedInstrument{
		instrumentResponse: instrumentResponse{Ticker: "MIS.US", PriceTicker: "MIS", QuoteCurrency: ptr("USD")},
		PriceConfirmed:     true,
	}, mis)
	assert.True(t, aaa.PriceConfirmed)
	got := srv.accountWarnings(t, id)
	assert.Equal(t, []string{"AAA.US noData", "MIS.US differentCurrency"}, kinds(got), "noData and differentCurrency have no confirmation")
	for _, w := range got {
		assert.False(t, w.Confirmable, "%s %s", w.Ticker, w.Kind)
		assert.True(t, w.PriceConfirmed, "%s %s", w.Ticker, w.Kind)
	}
	holdings := srv.holdingWarnings(t, id)
	require.Len(t, holdings, 2)
	assert.True(t, holdings[1].PriceConfirmed)
	assert.Equal(t, []string{"AAA.US noData", "MIS.US differentCurrency"}, kinds(srv.refreshWarnings(t)), "a refresh keeps the confirmation")

	srv.confirmPrice(t, "MIS.US", false)

	assert.Equal(t, []string{"AAA.US noData", "MIS.US differentCurrency", "MIS.US priceMismatch"}, kinds(srv.accountWarnings(t, id)))
}

func TestPriceConfirmation_PriceTickerChange_RemovesConfirmationAndOldAnswers(t *testing.T) {
	t.Parallel()
	provider := misProvider(t, "108", "92", "110")
	provider.setHistory(t, "MISX", "USD", "2026-10-19=150", "2026-10-20=150", "2026-10-21=150")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	srv.refreshWarnings(t)
	srv.confirmPrice(t, "MIS.US", true)
	require.Empty(t, srv.accountWarnings(t, id))

	srv.setPriceTicker(t, "MIS.US", "MISX")
	holdings := srv.holdingWarnings(t, id)
	require.Len(t, holdings, 1)
	assert.False(t, holdings[0].PriceConfirmed)

	got := srv.refreshWarnings(t)

	require.Len(t, got, 1)
	assert.Equal(t, "priceMismatch", got[0].Kind)
	assert.Equal(t, "MISX", got[0].PriceTicker)
	assert.False(t, got[0].PriceConfirmed)
}

func TestPriceConfirmation_PriceTickerChange_RemovesNoDataAndSourceCurrency(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.failHistory("MIS", fmt.Errorf("status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, tradeExport("100"), "XTB USD")
	require.Equal(t, []string{"MIS.US noData"}, kinds(srv.refreshWarnings(t)))

	srv.setPriceTicker(t, "MIS.US", "MISX")

	assert.Empty(t, srv.accountWarnings(t, id), "the tool did not ask the price source for the new price ticker yet")
}

func TestPriceConfirmation_SamePriceTicker_KeepsConfirmation(t *testing.T) {
	t.Parallel()
	srv := startServerWithPrices(t, misProvider(t, "108", "92", "110"))
	id := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	srv.refreshWarnings(t)
	srv.confirmPrice(t, "MIS.US", true)

	srv.setPriceTicker(t, "MIS.US", "MIS")

	assert.Empty(t, srv.accountWarnings(t, id))
}

func TestPriceConfirmation_QuoteCurrencyChange_RemovesConfirmation(t *testing.T) {
	t.Parallel()
	srv := startServerWithPrices(t, misProvider(t, "108", "92", "110"))
	id := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	srv.refreshWarnings(t)
	srv.confirmPrice(t, "MIS.US", true)

	same := srv.setQuoteCurrency(t, "MIS.US", "USD")
	require.Empty(t, srv.accountWarnings(t, id), "the same Quote Currency keeps the confirmation")
	srv.setQuoteCurrency(t, "MIS.US", "EUR")
	srv.setQuoteCurrency(t, "MIS.US", "")

	assert.Equal(t, "USD", *same.QuoteCurrency)
	assert.Equal(t, []priceWarning{mismatch("8", "10")}, srv.accountWarnings(t, id))
}

func TestPriceConfirmation_InvalidRequest_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, tradeExport("100"), "XTB USD")

	status, _, body := srv.do(t, http.MethodPut, "/api/instruments/MIS.US/price-confirmation", strings.NewReader("{"))
	assert.Equal(t, http.StatusBadRequest, status, "body %q", body)

	status, body = srv.putJSON(t, "/api/instruments/NOPE.US/price-confirmation", map[string]bool{"confirmed": true})
	assert.Equal(t, http.StatusNotFound, status, "body %q", body)
}

func TestPriceWarnings_ValueHistory_ShowsChartWithWarnings(t *testing.T) {
	t.Parallel()
	srv := startServerWithPrices(t, misProvider(t, "108", "92", "110"))
	id := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	srv.refreshWarnings(t)

	got := srv.historyWarnings(t, fmt.Sprintf("/api/accounts/%d/value-history", id))

	assert.Len(t, got.PriceWarnings, 1)
	assert.NotEmpty(t, got.Points, "the chart shows also when a Price Warning stays")
}

func TestPriceWarnings_AccountValueHistory_ListsOnlyInstrumentsOfItsLedger(t *testing.T) {
	t.Parallel()
	provider := misProvider(t, "108", "92", "110")
	provider.failHistory("ZZZ", fmt.Errorf("status 404: %w", price.ErrNoData))
	srv := startServerWithPrices(t, provider)
	misID := srv.importAccount(t, tradeExport("100", "100", "100"), "XTB USD")
	zzzID := srv.importAccount(t, xtbtest.Export{AccountNumber: "59000003", Currency: "USD", CashOperations: []xtbtest.CashOperation{
		{Type: "Stock purchase", Ticker: "ZZZ.US", Instrument: "ZZZ", Time: historyTime(19), Amount: "-10.00", ID: "1602", Comment: "OPEN BUY 1 @ 10.00", PositionID: "16000000001"},
		cashDeposit("1601", "100.00", historyTime(1)),
	}}, "XTB USD 2")

	refresh := srv.refreshWarnings(t)

	assert.Equal(t, []string{"MIS.US priceMismatch", "ZZZ.US noData"}, kinds(refresh))
	assert.Equal(t, []string{"MIS.US priceMismatch"}, kinds(srv.accountWarnings(t, misID)))
	assert.Equal(t, []string{"ZZZ.US noData"}, kinds(srv.accountWarnings(t, zzzID)))
	assert.Equal(t, []string{"MIS.US priceMismatch", "ZZZ.US noData"},
		kinds(srv.historyWarnings(t, "/api/all-accounts/value-history").PriceWarnings))
}
