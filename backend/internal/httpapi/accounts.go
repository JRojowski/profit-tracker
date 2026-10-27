package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// accountResponse is an Account in a response.
type accountResponse struct {
	ID            int64  `json:"id"`
	AccountNumber string `json:"accountNumber"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Currency      string `json:"currency"`
}

// renameRequest is the body of PATCH /api/accounts/{id}.
type renameRequest struct {
	Name string `json:"name"`
}

// summaryResponse is the body of GET /api/accounts/{id}/summary. Amounts are decimal text.
type summaryResponse struct {
	AccountID      int64  `json:"accountId"`
	Currency       string `json:"currency"`
	Cash           string `json:"cash"`
	NetDeposits    string `json:"netDeposits"`
	RealizedProfit string `json:"realizedProfit"`
	NetDividends   string `json:"netDividends"`
	NetInterest    string `json:"netInterest"`
	Fees           string `json:"fees"`
	// UnrealizedProfit is the sum over the Holdings with a price. It is a part of TotalResult.
	UnrealizedProfit string `json:"unrealizedProfit"`
	TotalResult      string `json:"totalResult"`
	// Value is the cash balance plus the value of the Holdings with a price.
	Value string `json:"value"`
	// PriceDate is the oldest price date of the Holdings with a value, as YYYY-MM-DD. It is nil if no Holding has a value.
	PriceDate *string `json:"priceDate"`
	// ExchangeRateDate is the oldest table date of the Exchange Rates that the Holdings use, as YYYY-MM-DD.
	// It is nil if no Holding uses an Exchange Rate.
	ExchangeRateDate *string `json:"exchangeRateDate"`
	// StalePrices tells that a Holding with a known Quote Currency has no price in that currency,
	// or its price was fetched before today.
	StalePrices bool `json:"stalePrices"`
	// StaleRates tells that a Holding needs an Exchange Rate that is missing,
	// or that the tool fetched before today in Polish time.
	StaleRates bool `json:"staleRates"`
	// UnpricedHoldings is the number of Holdings with no value.
	UnpricedHoldings int `json:"unpricedHoldings"`
	// SimpleReturn is the Simple Return as a percentage in decimal text: "10" is 10 %. It is exact, with at most
	// 16 decimal places. The frontend rounds for display. It is nil if it is not available.
	// Then SimpleReturnNote tells the reason.
	SimpleReturn     *string `json:"simpleReturn"`
	SimpleReturnNote *string `json:"simpleReturnNote"`
	// AnnualReturn is the Annual Return as a percentage in decimal text, with at most 8 decimal places.
	// It is nil if it is not available. Then AnnualReturnNote tells the reason.
	AnnualReturn     *string `json:"annualReturn"`
	AnnualReturnNote *string `json:"annualReturnNote"`
}

// holdingResponse is one row of GET /api/accounts/{id}/holdings. Amounts and the volume are decimal text.
// The price fields are nil when the Holding has no value. Then Note tells the reason.
type holdingResponse struct {
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	Volume      string `json:"volume"`
	Cost        string `json:"cost"`
	AverageCost string `json:"averageCost"`
	priceSourceResponse
	// Price is the close price in the Quote Currency. PriceDate is its date as YYYY-MM-DD.
	Price     *string `json:"price"`
	PriceDate *string `json:"priceDate"`
	// ExchangeRate is the Exchange Rate that Value uses: the PLN amount for 1 unit of ExchangeRateCurrency,
	// the currency that is not PLN. ExchangeRateDate is its NBP table date as YYYY-MM-DD.
	// They are nil if Value uses no rate.
	ExchangeRate         *string `json:"exchangeRate"`
	ExchangeRateCurrency *string `json:"exchangeRateCurrency"`
	ExchangeRateDate     *string `json:"exchangeRateDate"`
	// Value is in the Account currency.
	Value *string `json:"value"`
	// UnrealizedProfit is Value minus Cost.
	UnrealizedProfit *string `json:"unrealizedProfit"`
	// UnrealizedProfitPercent is UnrealizedProfit × 100 / Cost, exact. The frontend rounds for display.
	UnrealizedProfitPercent *string `json:"unrealizedProfitPercent"`
	// Note is a sentence for the user when the Holding has no value.
	Note *string `json:"note"`
}

// saleResponse is one row of GET /api/accounts/{id}/sales. Amounts and the volume are decimal text.
type saleResponse struct {
	XTBID string `json:"xtbId"`
	// Time is RFC 3339 text in UTC.
	Time       string `json:"time"`
	Ticker     string `json:"ticker"`
	Name       string `json:"name"`
	PositionID string `json:"positionId"`
	// Volume is the closed volume of the Lot.
	Volume string `json:"volume"`
	Amount string `json:"amount"`
	// Cost is the cost of the closed part. ADR 0001: it is exact. The frontend rounds for display.
	Cost           string `json:"cost"`
	RealizedProfit string `json:"realizedProfit"`
}

// listAccounts returns all Accounts, ordered by name.
func (a *api) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.store.Accounts(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	resp := make([]accountResponse, 0, len(accounts))
	for _, acc := range accounts {
		resp = append(resp, toAccountResponse(acc))
	}
	writeJSON(w, http.StatusOK, resp)
}

// renameAccount changes the name of the Account.
func (a *api) renameAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := accountID(w, r)
	if !ok {
		return
	}
	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "the name is empty")
		return
	}

	acc, err := a.store.RenameAccount(r.Context(), id, name)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAccountResponse(acc))
}

// accountSummary returns the value, the cash balance, the Net Deposits, the total result with its parts,
// and the price status of the Account.
func (a *api) accountSummary(w http.ResponseWriter, r *http.Request) {
	view, ok := a.accountView(w, r)
	if !ok {
		return
	}
	state := view.state
	writeJSON(w, http.StatusOK, summaryResponse{
		AccountID:        view.account.ID,
		Currency:         view.account.Currency,
		Cash:             formatAmount(state.Cash),
		NetDeposits:      formatAmount(state.NetDeposits),
		RealizedProfit:   formatAmount(state.RealizedProfit),
		NetDividends:     formatAmount(state.NetDividends),
		NetInterest:      formatAmount(state.NetInterest),
		Fees:             formatAmount(state.Fees),
		UnrealizedProfit: formatAmount(state.UnrealizedProfit),
		TotalResult:      formatAmount(state.TotalResult),
		Value:            formatAmount(state.Value),
		PriceDate:        view.priceDate(),
		ExchangeRateDate: view.exchangeRateDate(),
		StalePrices:      view.stalePrices(a.today()),
		StaleRates:       view.staleRates(exchange.StartOfDayInWarsaw(a.now())),
		UnpricedHoldings: view.unpricedHoldings(),
		SimpleReturn:     percent(state.SimpleReturn),
		SimpleReturnNote: returnNote(state.SimpleReturn),
		AnnualReturn:     percent(state.AnnualReturn),
		AnnualReturnNote: returnNote(state.AnnualReturn),
	})
}

// accountHoldings returns the Holdings of the Account with their value, ordered by ticker.
func (a *api) accountHoldings(w http.ResponseWriter, r *http.Request) {
	view, ok := a.accountView(w, r)
	if !ok {
		return
	}
	resp := make([]holdingResponse, 0, len(view.state.Holdings))
	for _, h := range view.state.Holdings {
		resp = append(resp, view.holdingRow(h))
	}
	writeJSON(w, http.StatusOK, resp)
}

// accountSales returns the sales of the Account with their Realized Profit, in time order and then in XTB ID order.
func (a *api) accountSales(w http.ResponseWriter, r *http.Request) {
	view, ok := a.accountView(w, r)
	if !ok {
		return
	}
	resp := make([]saleResponse, 0, len(view.state.Sales))
	for _, s := range view.state.Sales {
		resp = append(resp, saleResponse{
			XTBID:          s.XTBID,
			Time:           s.Time.UTC().Format(time.RFC3339),
			Ticker:         s.Instrument.Ticker,
			Name:           s.Instrument.Name,
			PositionID:     s.PositionID,
			Volume:         s.Volume.String(),
			Amount:         formatAmount(s.Amount),
			Cost:           formatAmount(s.Cost),
			RealizedProfit: formatAmount(s.RealizedProfit),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// accountView reads the Account of the path and calculates its state from the Ledger and the stored prices.
// It writes the error response and returns false if it cannot.
func (a *api) accountView(w http.ResponseWriter, r *http.Request) (accountView, bool) {
	id, ok := accountID(w, r)
	if !ok {
		return accountView{}, false
	}
	acc, err := a.store.Account(r.Context(), id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return accountView{}, false
	}
	view, err := a.readAccountView(r.Context(), acc)
	if err != nil {
		a.writeInternalError(w, r, err)
		return accountView{}, false
	}
	return view, true
}

// readAccountView reads the Ledger, the price data and the Exchange Rates, and calculates the state of the Account.
func (a *api) readAccountView(ctx context.Context, acc store.Account) (accountView, error) {
	in, err := a.readHeroInput(ctx, acc)
	if err != nil {
		return accountView{}, err
	}
	state, err := ledger.Calculate(in.ledger)
	if err != nil {
		return accountView{}, fmt.Errorf("calculate the state of account %d: %w", acc.ID, err)
	}
	return accountView{account: acc, state: state, instruments: in.instruments, prices: in.prices, rates: in.rates}, nil
}

// heroInput is the input of the state of one Account, with the price data that it comes from.
type heroInput struct {
	// ledger has the Cash Operations, the Lots, the current prices and the latest Exchange Rates.
	ledger ledger.Ledger
	// instruments are the price sources of the Instruments by broker ticker.
	instruments map[string]store.Instrument
	// prices are the last stored prices by broker ticker, in any currency.
	prices map[string]store.Price
	// rates are the latest Exchange Rates by currency.
	rates map[string]exchange.Latest
}

// readHeroInput reads the Ledger, the price data and the Exchange Rates of the Account.
func (a *api) readHeroInput(ctx context.Context, acc store.Account) (heroInput, error) {
	l, err := a.store.Ledger(ctx, acc.ID)
	if err != nil {
		return heroInput{}, err
	}
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		return heroInput{}, err
	}
	prices, err := a.store.LastPrices(ctx)
	if err != nil {
		return heroInput{}, err
	}
	rates, err := a.rates.LatestRates(ctx)
	if err != nil {
		return heroInput{}, err
	}
	l.Currency = acc.Currency
	l.QuoteCurrencies = make(map[string]string, len(instruments))
	for ticker, inst := range instruments {
		l.QuoteCurrencies[ticker] = inst.QuoteCurrency
	}
	l.Prices = quotePrices(instruments, prices)
	// The Annual Return counts days in Polish time, as the day of a flow in ADR 0004.
	l.Today = exchange.StartOfDayInWarsaw(a.now())
	l.ExchangeRates = make(map[string]ledger.ExchangeRate, len(rates))
	for currency, r := range rates {
		l.ExchangeRates[currency] = ledger.ExchangeRate{Rate: r.Mid, Date: r.Date}
	}
	return heroInput{ledger: l, instruments: instruments, prices: prices, rates: rates}, nil
}

// accountID returns the Account ID of the path. An ID that is not a number is an unknown Account.
// It writes the error response and returns false if the ID is not valid.
func accountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("account %q not found", r.PathValue("id")))
		return 0, false
	}
	return id, true
}

// writeAccountError maps an error of an Account read to a status.
func (a *api) writeAccountError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	a.writeInternalError(w, r, err)
}

func toAccountResponse(acc store.Account) accountResponse {
	return accountResponse{
		ID:            acc.ID,
		AccountNumber: acc.AccountNumber,
		Name:          acc.Name,
		Type:          acc.Type,
		Currency:      acc.Currency,
	}
}

// formatAmount writes an amount with at least 2 decimal places, for example "5000.00".
// It does not round: an amount with more decimal places keeps all of them.
// A result of a division can be exact to the cent, but with 16 decimal places in its internal form,
// so the check uses the value and not the exponent.
func formatAmount(d decimal.Decimal) string {
	if !d.Equal(d.Truncate(2)) {
		return d.String()
	}
	return d.StringFixed(2)
}
