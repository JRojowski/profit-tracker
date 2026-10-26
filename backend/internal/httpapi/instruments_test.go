package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// instrumentResponse is the body of PUT /api/instruments/{ticker}/price-ticker and /quote-currency.
type instrumentResponse struct {
	Ticker              string  `json:"ticker"`
	PriceTicker         string  `json:"priceTicker"`
	PriceTickerManual   bool    `json:"priceTickerManual"`
	QuoteCurrency       *string `json:"quoteCurrency"`
	QuoteCurrencyManual bool    `json:"quoteCurrencyManual"`
}

// holdingSource is the price source fields of one row of GET /api/accounts/{id}/holdings, with its price.
type holdingSource struct {
	Ticker              string  `json:"ticker"`
	PriceTicker         string  `json:"priceTicker"`
	PriceTickerManual   bool    `json:"priceTickerManual"`
	QuoteCurrency       *string `json:"quoteCurrency"`
	QuoteCurrencyManual bool    `json:"quoteCurrencyManual"`
	Price               *string `json:"price"`
	Value               *string `json:"value"`
}

// putJSON sends v as a JSON body with the method PUT to path. It returns the status and the body.
func (s *testServer) putJSON(t *testing.T, path string, v any) (status int, body string) {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)
	status, _, body = s.do(t, http.MethodPut, path, bytes.NewReader(data))
	return status, body
}

// setPriceTicker sets the manual price ticker of the Instrument and returns the Instrument.
// It stops the test if the status is not 200.
func (s *testServer) setPriceTicker(t *testing.T, ticker, priceTicker string) instrumentResponse {
	t.Helper()

	status, body := s.putJSON(t, "/api/instruments/"+ticker+"/price-ticker", map[string]string{"priceTicker": priceTicker})
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got instrumentResponse
	decode(t, body, &got)
	return got
}

// holdingSources returns the price source fields of the Holdings of the Account.
func (s *testServer) holdingSources(t *testing.T, accountID int64) []holdingSource {
	t.Helper()

	var got []holdingSource
	status := s.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", accountID), &got)
	require.Equal(t, http.StatusOK, status)
	return got
}

func TestSetPriceTicker_ManualTicker_RefreshAsksForIt(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABCX", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")

	got := srv.setPriceTicker(t, "ABC.US", "  ABCX  ")
	refresh := srv.refreshPrices(t)

	assert.Equal(t, instrumentResponse{
		Ticker: "ABC.US", PriceTicker: "ABCX", PriceTickerManual: true, QuoteCurrency: ptr("USD"), QuoteCurrencyManual: false,
	}, got, "the tool trims the whitespace")
	assert.Equal(t, refreshResponse{Refreshed: 2, Failures: []refreshFailure{}, RateFailures: []rateFailure{}}, refresh)
	assert.ElementsMatch(t, []string{"ABCX", "XYZ"}, provider.requestedTickers())
	assert.Equal(t, []holdingSource{
		{Ticker: "ABC.US", PriceTicker: "ABCX", PriceTickerManual: true, QuoteCurrency: ptr("USD"), Price: ptr("125.5"), Value: ptr("251.00")},
		{Ticker: "XYZ.US", PriceTicker: "XYZ", QuoteCurrency: ptr("USD"), Price: ptr("110"), Value: ptr("55.00")},
	}, srv.holdingSources(t, id))
}

// setQuoteCurrency sets the manual quote currency of the Instrument and returns the Instrument.
// It stops the test if the status is not 200.
func (s *testServer) setQuoteCurrency(t *testing.T, ticker, quoteCurrency string) instrumentResponse {
	t.Helper()

	status, body := s.putJSON(t, "/api/instruments/"+ticker+"/quote-currency", map[string]string{"quoteCurrency": quoteCurrency})
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got instrumentResponse
	decode(t, body, &got)
	return got
}

func TestSetPriceTicker_EmptyValue_RefreshUsesDefaultMappingAgain(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABCX", "125.50", "USD", "2026-10-22")
	provider.set(t, "ABC", "130.00", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.setPriceTicker(t, "ABC.US", "ABCX")
	srv.refreshPrices(t)

	got := srv.setPriceTicker(t, "ABC.US", "")
	srv.refreshPrices(t)

	assert.Equal(t, instrumentResponse{Ticker: "ABC.US", PriceTicker: "ABC", PriceTickerManual: false, QuoteCurrency: ptr("USD")}, got)
	assert.Equal(t, []string{"ABCX", "XYZ", "ABC", "XYZ"}, provider.requestedTickers())
	holdings := srv.holdingSources(t, id)
	require.Len(t, holdings, 2)
	assert.Equal(t, holdingSource{
		Ticker: "ABC.US", PriceTicker: "ABC", QuoteCurrency: ptr("USD"), Price: ptr("130"), Value: ptr("260.00"),
	}, holdings[0])
}

func TestSetPriceTicker_InstrumentWithStoredPrice_ShowsNoPriceUntilNextRefresh(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)

	srv.setPriceTicker(t, "ABC.US", "ABCX")

	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 2)
	assert.Equal(t, noValue("ABC.US", ptr("USD"), noPriceNote), holdings[0],
		"the stored price came from the old price ticker")
	assert.Equal(t, ptr("55.00"), holdings[1].Value, "the change keeps the prices of the other Instruments")
	assert.Len(t, provider.requestedTickers(), 2, "the change does not get a price")

	provider.set(t, "ABCX", "126.00", "USD", "2026-10-22")
	srv.refreshPrices(t)

	assert.Equal(t, ptr("252.00"), srv.holdingPrices(t, id)[0].Value)
}

func TestSetPriceTicker_SamePriceTickerAsDefaultMapping_KeepsStoredPrice(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "ABC", "125.50", "USD", "2026-10-22")
	provider.set(t, "XYZ", "110.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(), "XTB USD")
	srv.refreshPrices(t)

	got := srv.setPriceTicker(t, "ABC.US", "ABC")

	assert.Equal(t, instrumentResponse{Ticker: "ABC.US", PriceTicker: "ABC", PriceTickerManual: true, QuoteCurrency: ptr("USD")}, got)
	assert.Equal(t, ptr("251.00"), srv.holdingPrices(t, id)[0].Value, "the price ticker that the tool uses did not change")
}

// cspxExport is an export of a USD Account with one Holding of CSPX.UK. The ticker suffix tells GBP,
// but the Instrument is quoted in USD. The rows have fixed times, so that a newer export with more rows gives
// the same data for them.
func cspxExport(extra ...xtbtest.CashOperation) xtbtest.Export {
	ops := append([]xtbtest.CashOperation(nil), extra...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "CSPX.UK", Instrument: "CSPX", Time: opTime(2), Amount: "-600.00", ID: "802", Comment: "OPEN BUY 1 @ 600.00", PositionID: "8000000001"},
		xtbtest.CashOperation{Type: "Deposit", Time: opTime(1), Amount: "1000.00", ID: "801", Comment: "Blik deposit"},
	)
	return xtbtest.Export{AccountNumber: "56000020", Currency: "USD", CashOperations: ops}
}

func TestSetQuoteCurrency_USDOnUKInstrument_RefreshAcceptsUSDPrice(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "CSPX.L", "650.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, cspxExport(), "XTB USD")
	require.Equal(t, refreshResponse{Refreshed: 0, Failures: []refreshFailure{
		{Ticker: "CSPX.UK", Error: "the price is in USD, but the Instrument is quoted in GBP"},
	}, RateFailures: []rateFailure{}}, srv.refreshPrices(t))

	got := srv.setQuoteCurrency(t, "CSPX.UK", " usd ")
	refresh := srv.refreshPrices(t)

	assert.Equal(t, instrumentResponse{
		Ticker: "CSPX.UK", PriceTicker: "CSPX.L", PriceTickerManual: false, QuoteCurrency: ptr("USD"), QuoteCurrencyManual: true,
	}, got, "the tool trims the whitespace and changes the letters to upper case")
	assert.Equal(t, refreshResponse{Refreshed: 1, Failures: []refreshFailure{}, RateFailures: []rateFailure{}}, refresh)
	assert.Equal(t, []holdingSource{
		{Ticker: "CSPX.UK", PriceTicker: "CSPX.L", QuoteCurrency: ptr("USD"), QuoteCurrencyManual: true, Price: ptr("650"), Value: ptr("650.00")},
	}, srv.holdingSources(t, id))
}

func TestSetQuoteCurrency_EmptyValue_RestoresCurrencyFromSuffixAndKeepsPrices(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "CSPX.L", "650.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, cspxExport(), "XTB USD")
	srv.setQuoteCurrency(t, "CSPX.UK", "USD")
	srv.refreshPrices(t)

	got := srv.setQuoteCurrency(t, "CSPX.UK", "")

	assert.Equal(t, instrumentResponse{Ticker: "CSPX.UK", PriceTicker: "CSPX.L", QuoteCurrency: ptr("GBP")}, got)
	assert.Equal(t, []holdingPrices{
		noValue("CSPX.UK", ptr("GBP"), "The Instrument is quoted in GBP. The tool has no Exchange Rate between GBP and USD."),
	}, srv.holdingPrices(t, id), "the tool ignores the stored USD price of an Instrument quoted in GBP")

	// The change kept the USD price, so the manual USD gives the value again with no refresh.
	srv.setQuoteCurrency(t, "CSPX.UK", "USD")

	assert.Equal(t, ptr("650.00"), srv.holdingPrices(t, id)[0].Value)
	assert.Len(t, provider.requestedTickers(), 1, "the changes do not get a price")
}

func TestSetQuoteCurrency_EmptyValueOnUnknownSuffix_QuoteCurrencyIsUnknown(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, pricedExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "GHI.FR", Instrument: "GHI", Amount: "-30.00", ID: "605", Comment: "OPEN BUY 1 @ 30.00", PositionID: "6000000004"},
	), "XTB USD")
	srv.setQuoteCurrency(t, "GHI.FR", "EUR")

	got := srv.setQuoteCurrency(t, "GHI.FR", "")

	assert.Equal(t, instrumentResponse{Ticker: "GHI.FR", PriceTicker: "GHI.FR", QuoteCurrency: nil}, got)
}

func TestSetPriceTicker_UnknownSuffixWithNoQuoteCurrency_TellsUserToSetQuoteCurrency(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "GHI.PA", "30.00", "USD", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importAccount(t, pricedExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "GHI.FR", Instrument: "GHI", Amount: "-30.00", ID: "605", Comment: "OPEN BUY 1 @ 30.00", PositionID: "6000000004"},
	), "XTB USD")
	srv.setPriceTicker(t, "GHI.FR", "GHI.PA")

	srv.refreshPrices(t)

	assert.NotContains(t, provider.requestedTickers(), "GHI.PA", "the tool cannot check the price of an Instrument with no quote currency")
	holdings := srv.holdingPrices(t, id)
	require.Len(t, holdings, 3)
	assert.Equal(t, noValue("GHI.FR", nil, unknownQuoteCurrencyNote), holdings[1])
}

func TestSetInstrumentSource_InvalidValue_Returns400AndChangesNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "price ticker with a space inside", path: "/api/instruments/ABC.US/price-ticker", body: `{"priceTicker": "VUSA L"}`},
		{name: "price ticker with a tab inside", path: "/api/instruments/ABC.US/price-ticker", body: `{"priceTicker": "VUSA\tL"}`},
		{name: "price ticker of 33 characters", path: "/api/instruments/ABC.US/price-ticker", body: `{"priceTicker": "` + strings.Repeat("A", 33) + `"}`},
		{name: "price ticker body that is not JSON", path: "/api/instruments/ABC.US/price-ticker", body: `VUSA.L`},
		{name: "quote currency of two letters", path: "/api/instruments/ABC.US/quote-currency", body: `{"quoteCurrency": "US"}`},
		{name: "quote currency of four letters", path: "/api/instruments/ABC.US/quote-currency", body: `{"quoteCurrency": "USDT"}`},
		{name: "quote currency with a digit", path: "/api/instruments/ABC.US/quote-currency", body: `{"quoteCurrency": "U5D"}`},
		{name: "quote currency with a letter that is not A to Z", path: "/api/instruments/ABC.US/quote-currency", body: `{"quoteCurrency": "ZŁO"}`},
		{name: "quote currency body that is not JSON", path: "/api/instruments/ABC.US/quote-currency", body: `USD`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			id := srv.importAccount(t, pricedExport(), "XTB USD")

			status, _, body := srv.do(t, http.MethodPut, tc.path, strings.NewReader(tc.body))

			assert.Equal(t, http.StatusBadRequest, status, "body %q", body)
			var got errorBody
			decode(t, body, &got)
			assert.NotEmpty(t, got.Error)
			holdings := srv.holdingSources(t, id)
			require.Len(t, holdings, 2)
			assert.Equal(t, holdingSource{Ticker: "ABC.US", PriceTicker: "ABC", QuoteCurrency: ptr("USD")}, holdings[0])
		})
	}
}

func TestSetInstrumentSource_UnknownTicker_Returns404(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importAccount(t, pricedExport(), "XTB USD")

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "set price ticker", path: "/api/instruments/NOPE.US/price-ticker", body: `{"priceTicker": "NOPE"}`},
		{name: "remove price ticker", path: "/api/instruments/NOPE.US/price-ticker", body: `{"priceTicker": ""}`},
		{name: "set quote currency", path: "/api/instruments/NOPE.US/quote-currency", body: `{"quoteCurrency": "USD"}`},
		{name: "remove quote currency", path: "/api/instruments/NOPE.US/quote-currency", body: `{"quoteCurrency": ""}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			status, _, body := srv.do(t, http.MethodPut, tc.path, strings.NewReader(tc.body))

			assert.Equal(t, http.StatusNotFound, status, "body %q", body)
			assert.JSONEq(t, `{"error": "instrument NOPE.US: not found"}`, body)
		})
	}
}

func TestImport_InstrumentWithManualSource_KeepsManualValues(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	id := srv.importAccount(t, cspxExport(), "XTB USD")
	srv.setPriceTicker(t, "CSPX.UK", "CSPX.AS")
	srv.setQuoteCurrency(t, "CSPX.UK", "USD")

	// A newer export of the same Account with one more purchase of the Instrument.
	preview := srv.preview(t, cspxExport(
		xtbtest.CashOperation{Type: "Stock purchase", Ticker: "CSPX.UK", Instrument: "CSPX", Time: opTime(3), Amount: "-300.00", ID: "803", Comment: "OPEN BUY 0.5 @ 600.00", PositionID: "8000000002"},
	))
	saved := srv.confirm(t, preview.ID, nil)

	require.Equal(t, id, saved.AccountID)
	assert.Equal(t, []holdingSource{
		{Ticker: "CSPX.UK", PriceTicker: "CSPX.AS", PriceTickerManual: true, QuoteCurrency: ptr("USD"), QuoteCurrencyManual: true},
	}, srv.holdingSources(t, id))
}
