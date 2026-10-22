package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

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
	AccountID   int64  `json:"accountId"`
	Currency    string `json:"currency"`
	Cash        string `json:"cash"`
	NetDeposits string `json:"netDeposits"`
}

// holdingResponse is one row of GET /api/accounts/{id}/holdings. Amounts and the volume are decimal text.
type holdingResponse struct {
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	Volume      string `json:"volume"`
	Cost        string `json:"cost"`
	AverageCost string `json:"averageCost"`
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

// accountSummary returns the cash balance and the Net Deposits of the Account.
func (a *api) accountSummary(w http.ResponseWriter, r *http.Request) {
	acc, state, ok := a.accountState(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, summaryResponse{
		AccountID:   acc.ID,
		Currency:    acc.Currency,
		Cash:        formatAmount(state.Cash),
		NetDeposits: formatAmount(state.NetDeposits),
	})
}

// accountHoldings returns the Holdings of the Account, ordered by ticker.
func (a *api) accountHoldings(w http.ResponseWriter, r *http.Request) {
	_, state, ok := a.accountState(w, r)
	if !ok {
		return
	}
	resp := make([]holdingResponse, 0, len(state.Holdings))
	for _, h := range state.Holdings {
		resp = append(resp, holdingResponse{
			Ticker: h.Instrument.Ticker,
			Name:   h.Instrument.Name,
			Volume: h.Volume.String(),
			Cost:   formatAmount(h.Cost),
			// The frontend rounds for display. ADR 0001: the tool rounds only for display.
			AverageCost: h.AverageCost().String(),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// accountSales returns the sales of the Account with their Realized Profit, in time order and then in XTB ID order.
func (a *api) accountSales(w http.ResponseWriter, r *http.Request) {
	_, state, ok := a.accountState(w, r)
	if !ok {
		return
	}
	resp := make([]saleResponse, 0, len(state.Sales))
	for _, s := range state.Sales {
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

// accountState reads the Account of the path and calculates its state from the Ledger.
// It writes the error response and returns false if it cannot.
func (a *api) accountState(w http.ResponseWriter, r *http.Request) (store.Account, ledger.State, bool) {
	id, ok := accountID(w, r)
	if !ok {
		return store.Account{}, ledger.State{}, false
	}
	acc, err := a.store.Account(r.Context(), id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return store.Account{}, ledger.State{}, false
	}
	l, err := a.store.Ledger(r.Context(), id)
	if err != nil {
		a.writeInternalError(w, r, err)
		return store.Account{}, ledger.State{}, false
	}
	state, err := ledger.Calculate(l)
	if err != nil {
		a.writeInternalError(w, r, fmt.Errorf("calculate the state of account %d: %w", id, err))
		return store.Account{}, ledger.State{}, false
	}
	return acc, state, true
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
func formatAmount(d decimal.Decimal) string {
	if d.Exponent() < -2 {
		return d.String()
	}
	return d.StringFixed(2)
}
