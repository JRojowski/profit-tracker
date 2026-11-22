package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken/krakentest"
)

// krakenPreviewResponse is the part of the body of POST /api/imports that the Kraken tests read.
type krakenPreviewResponse struct {
	previewResponse
	Provider          string `json:"provider"`
	SuggestedName     string `json:"suggestedName"`
	SuggestedType     string `json:"suggestedType"`
	SuggestedCurrency string `json:"suggestedCurrency"`
}

// accountWithProvider is an Account in GET /api/accounts, with its Provider.
type accountWithProvider struct {
	ID            int64  `json:"id"`
	AccountNumber string `json:"accountNumber"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Currency      string `json:"currency"`
	Provider      string `json:"provider"`
}

// krakenTime returns the time of a Kraken entry on day n of March 2026, at 09:00 UTC.
func krakenTime(day int) time.Time {
	return time.Date(2026, 3, day, 9, 0, 0, 0, time.UTC)
}

// krakenExport is a small Kraken ledger: two EUR deposits and two BTC purchases with the fee in BTC.
//
//	Cash:         500.00 + 300.00 - 100.00 - 200.00 = 500.00
//	Net Deposits: 500.00 + 300.00                   = 800.00
//	BTC:          volume (0.00200000 - 0.00000500) + (0.00250000 - 0.00000625) = 0.00448875
//	              cost 100.00 + 200.00 = 300.00, Average Cost 300.00 / 0.00448875 = 66833.7510442773600668
func krakenExport() krakentest.Export {
	entries := []krakentest.Entry{
		krakentest.Deposit("DEP-0001", krakenTime(1), "500.00"),
		krakentest.Deposit("DEP-0002", krakenTime(2), "300.00"),
	}
	entries = append(entries, krakentest.Purchase("TRD-0001", krakenTime(3), "-100.00", "BTC", "0.00200000", "0.00000500")...)
	entries = append(entries, krakentest.Purchase("TRD-0002", krakenTime(4), "-200.00", "BTC", "0.00250000", "0.00000625")...)
	return krakentest.Export{Entries: entries}
}

// previewKraken uploads the Kraken export and returns the preview. It stops the test if the status is not 200.
func (s *testServer) previewKraken(t *testing.T, export krakentest.Export) krakenPreviewResponse {
	t.Helper()

	status, body := s.upload(t, export.Name(), krakentest.Build(t, export))
	require.Equal(t, http.StatusOK, status, "body %q", body)
	var got krakenPreviewResponse
	decode(t, body, &got)
	return got
}

// importKraken imports the Kraken export into the Kraken Account with the proposed values, and returns the Account ID.
func (s *testServer) importKraken(t *testing.T, export krakentest.Export) int64 {
	t.Helper()

	preview := s.previewKraken(t, export)
	saved := s.confirm(t, preview.ID, newAccountBody(preview.SuggestedName, preview.SuggestedType, preview.SuggestedCurrency))
	return saved.AccountID
}

func TestKrakenImport_NewFile_MakesTheKrakenAccount(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	preview := srv.previewKraken(t, krakenExport())

	assert.Equal(t, "kraken-spot", preview.AccountNumber)
	assert.Nil(t, preview.Account, "the Kraken Account is new")
	assert.Equal(t, "kraken", preview.Provider)
	assert.Equal(t, "Kraken", preview.SuggestedName)
	assert.Equal(t, "regular", preview.SuggestedType)
	assert.Equal(t, "EUR", preview.SuggestedCurrency)
	assert.Equal(t, 4, preview.NewCashOperations, "two deposits and two purchases")

	saved := srv.confirm(t, preview.ID, newAccountBody(preview.SuggestedName, preview.SuggestedType, preview.SuggestedCurrency))

	var accounts []accountWithProvider
	require.Equal(t, http.StatusOK, srv.getJSON(t, "/api/accounts", &accounts))
	assert.Equal(t, []accountWithProvider{{
		ID: saved.AccountID, AccountNumber: "kraken-spot", Name: "Kraken", Type: "regular", Currency: "EUR", Provider: "kraken",
	}}, accounts)
}

func TestKrakenImport_ConfirmWithCurrencyNotEUR_ReturnsBadRequest(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.previewKraken(t, krakenExport())

	status, body := srv.postJSON(t, fmt.Sprintf("/api/imports/%d/confirm", preview.ID), newAccountBody("Kraken", "regular", "USD"))

	assert.Equal(t, http.StatusBadRequest, status)
	var got struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	decode(t, body, &got)
	assert.Equal(t, "kraken-currency-not-eur", got.Code)
	assert.Contains(t, got.Error, `"USD"`)
	assert.Contains(t, got.Error, "EUR")
	assert.Empty(t, srv.imports(t), "nothing is saved")
}

func TestKrakenImport_DepositsAndPurchases_GiveCashAndHoldingAtAverageCost(t *testing.T) {
	t.Parallel()
	srv := startServer(t)

	id := srv.importKraken(t, krakenExport())

	summary := srv.summary(t, id)
	assert.Equal(t, "EUR", summary.Currency)
	assert.Equal(t, "500.00", summary.Cash)
	assert.Equal(t, "800.00", summary.NetDeposits)
	assert.Equal(t, "0.00", summary.Fees, "ADR 0009: the fee is in the cost, not in the fees part")
	assert.Equal(t, "0.00", summary.RealizedProfit)
	assert.Equal(t, []holdingResponse{{
		Ticker: "BTC", Name: "BTC", Volume: "0.00448875", Cost: "300.00", AverageCost: "66833.7510442773600668",
	}}, srv.holdings(t, id))
}

func TestKrakenImport_OldAssetCodeXBT_GivesTheInstrumentBTC(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := krakentest.Export{Entries: append(
		[]krakentest.Entry{krakentest.Deposit("DEP-0001", krakenTime(1), "100.00")},
		krakentest.Purchase("TRD-0001", krakenTime(2), "-100.00", "XBT", "0.00200000", "0.00000500")...,
	)}

	id := srv.importKraken(t, export)

	assert.Equal(t, []holdingResponse{{
		Ticker: "BTC", Name: "BTC", Volume: "0.001995", Cost: "100.00", AverageCost: "50125.3132832080200501",
	}}, srv.holdings(t, id))
}

func TestKrakenImport_OverlappingExport_AddsOnlyTheNewEntries(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	first := srv.importKraken(t, krakenExport())

	// A newer export: it starts after the first deposit and has one new deposit and one new purchase.
	newer := krakenExport()
	newer.Entries = append(newer.Entries[1:], krakentest.Deposit("DEP-0003", krakenTime(5), "100.00"))
	newer.Entries = append(newer.Entries, krakentest.Purchase("TRD-0003", krakenTime(6), "-50.00", "BTC", "0.00100000", "0.00000250")...)
	preview := srv.previewKraken(t, newer)

	require.NotNil(t, preview.Account)
	assert.Equal(t, first, preview.Account.ID, "a later Kraken Import goes into the same Account")
	assert.Equal(t, 2, preview.NewCashOperations)
	assert.Equal(t, 3, preview.SkippedCashOperations)
	assert.Empty(t, preview.Conflicts)

	saved := srv.confirm(t, preview.ID, nil)

	assert.Equal(t, first, saved.AccountID)
	assert.Equal(t, "550.00", srv.summary(t, first).Cash, "500.00 + 100.00 - 50.00")
	holdings := srv.holdings(t, first)
	require.Len(t, holdings, 1)
	assert.Equal(t, "0.00548625", holdings[0].Volume, "0.00448875 + 0.00100000 - 0.00000250")
	assert.Equal(t, "350.00", holdings[0].Cost)
}

// krakenHolding is the part of a row of GET /api/accounts/{id}/holdings that the price test reads.
type krakenHolding struct {
	Ticker            string  `json:"ticker"`
	PriceTicker       string  `json:"priceTicker"`
	PriceTickerManual bool    `json:"priceTickerManual"`
	QuoteCurrency     *string `json:"quoteCurrency"`
	Price             *string `json:"price"`
	Value             *string `json:"value"`
	UnrealizedProfit  *string `json:"unrealizedProfit"`
}

func TestKrakenImport_PriceOfTheAsset_GivesUnrealizedProfitFromTheEURClose(t *testing.T) {
	t.Parallel()
	provider := newFakeProvider()
	provider.set(t, "BTC-EUR", "70000", "EUR", "2026-10-22")
	srv := startServerWithPrices(t, provider)
	id := srv.importKraken(t, krakenExport())

	srv.refreshPrices(t)

	var holdings []krakenHolding
	require.Equal(t, http.StatusOK, srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/holdings", id), &holdings))
	// Value: 0.00448875 × 70000 = 314.2125. Unrealized Profit: 314.2125 - 300.00 = 14.2125.
	assert.Equal(t, []krakenHolding{{
		Ticker: "BTC", PriceTicker: "BTC-EUR", PriceTickerManual: false, QuoteCurrency: ptr("EUR"),
		Price: ptr("70000"), Value: ptr("314.2125"), UnrealizedProfit: ptr("14.2125"),
	}}, holdings)
	var summary struct {
		UnrealizedProfit string `json:"unrealizedProfit"`
		TotalResult      string `json:"totalResult"`
		Value            string `json:"value"`
	}
	require.Equal(t, http.StatusOK, srv.getJSON(t, fmt.Sprintf("/api/accounts/%d/summary", id), &summary))
	assert.Equal(t, "14.2125", summary.UnrealizedProfit)
	assert.Equal(t, "14.2125", summary.TotalResult, "the fees part is zero")
	assert.Equal(t, "814.2125", summary.Value, "cash 500.00 + BTC 314.2125")
}

// unsupportedEntriesError is the error body of an Import that an unsupported Kraken entry stops.
type unsupportedEntriesError struct {
	Error              string             `json:"error"`
	Code               string             `json:"code"`
	UnsupportedEntries []unsupportedEntry `json:"unsupportedEntries"`
}

// unsupportedEntry is one unsupported entry in the error body.
type unsupportedEntry struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Asset   string `json:"asset"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
}

func TestKrakenImport_UnsupportedEntry_StopsTheImportAndListsTheEntry(t *testing.T) {
	t.Parallel()

	at := krakenTime(20)
	atText := "2026-03-20T09:00:00Z"
	purchase := func(edit func(eur, crypto *krakentest.Entry)) []krakentest.Entry {
		rows := krakentest.Purchase("TRD-0099", at, "-100.00", "BTC", "0.00200000", "0.00000500")
		edit(&rows[0], &rows[1])
		return rows
	}
	tests := []struct {
		name    string
		entries []krakentest.Entry
		want    []unsupportedEntry
	}{
		{
			name: "sale",
			entries: []krakentest.Entry{
				{RefID: "TRD-0099", Time: at, Type: "trade", Subtype: "tradespot", Asset: "BTC", Amount: "-0.00100000"},
				{RefID: "TRD-0099", Time: at, Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: "60.00"},
			},
			want: []unsupportedEntry{
				{Type: "trade", Subtype: "tradespot", Asset: "BTC", Time: atText, Reason: "sale"},
				{Type: "trade", Subtype: "tradespot", Asset: "EUR", Time: atText, Reason: "sale"},
			},
		},
		{
			name:    "withdrawal",
			entries: []krakentest.Entry{{RefID: "WDR-0099", Time: at, Type: "withdrawal", Asset: "BTC", Amount: "-0.00100000"}},
			want:    []unsupportedEntry{{Type: "withdrawal", Subtype: "", Asset: "BTC", Time: atText, Reason: "withdrawal"}},
		},
		{
			name:    "fee on both rows of a purchase",
			entries: purchase(func(eur, _ *krakentest.Entry) { eur.Fee = "0.40" }),
			want: []unsupportedEntry{
				{Type: "trade", Subtype: "tradespot", Asset: "EUR", Time: atText, Reason: "fee that the tool does not support"},
				{Type: "trade", Subtype: "tradespot", Asset: "BTC", Time: atText, Reason: "fee that the tool does not support"},
			},
		},
		{
			name: "fee on the EUR row in a third currency",
			entries: purchase(func(eur, crypto *krakentest.Entry) {
				eur.Fee, eur.FeeCurrency, crypto.Fee = "0.40", "USD", "0"
			}),
			want: []unsupportedEntry{
				{Type: "trade", Subtype: "tradespot", Asset: "EUR", Time: atText, Reason: "fee that the tool does not support"},
				{Type: "trade", Subtype: "tradespot", Asset: "BTC", Time: atText, Reason: "fee that the tool does not support"},
			},
		},
		{
			name:    "fee on the crypto row that is not in the crypto asset",
			entries: purchase(func(_, crypto *krakentest.Entry) { crypto.FeeCurrency = "EUR" }),
			want: []unsupportedEntry{
				{Type: "trade", Subtype: "tradespot", Asset: "EUR", Time: atText, Reason: "fee that the tool does not support"},
				{Type: "trade", Subtype: "tradespot", Asset: "BTC", Time: atText, Reason: "fee that the tool does not support"},
			},
		},
		{
			name:    "deposit that is not EUR",
			entries: []krakentest.Entry{{RefID: "DEP-0099", Time: at, Type: "deposit", Asset: "BTC", Amount: "0.01000000"}},
			want:    []unsupportedEntry{{Type: "deposit", Subtype: "", Asset: "BTC", Time: atText, Reason: "deposit that is not EUR"}},
		},
		{
			name:    "wallet that is not spot / main",
			entries: []krakentest.Entry{{RefID: "DEP-0099", Time: at, Type: "deposit", Asset: "EUR", Amount: "50.00", Wallet: "earn / flexible"}},
			want:    []unsupportedEntry{{Type: "deposit", Subtype: "", Asset: "EUR", Time: atText, Reason: "wallet that is not spot / main"}},
		},
		{
			name:    "reward in EUR",
			entries: []krakentest.Entry{{RefID: "ERN-0099", Time: at, Type: "earn", Subtype: "reward", Asset: "EUR", Amount: "0.10"}},
			want:    []unsupportedEntry{{Type: "earn", Subtype: "reward", Asset: "EUR", Time: atText, Reason: "unknown entry"}},
		},
		{
			name:    "reward with a negative amount",
			entries: []krakentest.Entry{{RefID: "ERN-0099", Time: at, Type: "earn", Subtype: "reward", Asset: "BTC", Amount: "-0.00000110"}},
			want:    []unsupportedEntry{{Type: "earn", Subtype: "reward", Asset: "BTC", Time: atText, Reason: "unknown entry"}},
		},
		{
			name:    "staking move",
			entries: []krakentest.Entry{{RefID: "STK-0099", Time: at, Type: "earn", Subtype: "migration", Asset: "BTC", Amount: "-0.00100000"}},
			want:    []unsupportedEntry{{Type: "earn", Subtype: "migration", Asset: "BTC", Time: atText, Reason: "unknown entry"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)
			export := krakenExport()
			export.Entries = append(export.Entries, tc.entries...)

			status, body := srv.upload(t, export.Name(), krakentest.Build(t, export))

			require.Equal(t, http.StatusBadRequest, status, "body %q", body)
			var got unsupportedEntriesError
			decode(t, body, &got)
			assert.Equal(t, "unsupported-entries", got.Code)
			assert.Contains(t, got.Error, "entries that the tool does not support")
			assert.Equal(t, tc.want, got.UnsupportedEntries)
			assert.Empty(t, srv.imports(t), "the Import stops and saves nothing")
		})
	}
}

func TestKrakenImport_KnownRefIDWithDifferentData_GivesImportConflict(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	srv.importKraken(t, krakenExport())

	changed := krakenExport()
	// The second purchase: a different EUR amount for the known reference ID TRD-0002.
	changed.Entries[4].Amount = "-210.00"
	preview := srv.previewKraken(t, changed)

	assert.Equal(t, "needs-review", preview.State)
	assert.Equal(t, 0, preview.NewCashOperations)
	assert.Equal(t, 3, preview.SkippedCashOperations)
	require.Len(t, preview.Conflicts, 1)
	conflict := preview.Conflicts[0]
	assert.Equal(t, "changed-data", conflict.Kind)
	assert.Equal(t, "TRD-0002", conflict.XTBID)
	assert.Equal(t, "-200.00", conflict.Existing.Amount)
	assert.Equal(t, "-210.00", conflict.New.Amount)
}
