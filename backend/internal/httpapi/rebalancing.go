package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/rebalancing"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// rebalancingResponse is the body of GET /api/all-accounts/rebalancing. Amounts are decimal text. The amounts of
// the Tiers, the parts and the groups are in PLN. Each Planned Purchase and each amount of an Account is in the
// Account currency. A share is a percentage with at most 16 decimal places, for example "12.5" for 12.5 %.
type rebalancingResponse struct {
	// Available is false when the latest USD Exchange Rate is missing and a USD Tier Account exists. Then Warnings
	// tell why, the amounts are nil and the lists are empty.
	Available bool     `json:"available"`
	Warnings  []string `json:"warnings"`
	// ExchangeRate is the latest USD rate that converts the USD amounts, or nil if the tool has no USD rate.
	ExchangeRate *rebalancingRateResponse `json:"exchangeRate"`
	// Accounts are the Tier Accounts in the order of GET /api/accounts.
	Accounts []rebalancingAccountResponse `json:"accounts"`
	// MissingAccounts are the currencies, "PLN" or "USD", with no Tier Account.
	MissingAccounts []string `json:"missingAccounts"`
	// Value is the value of all priced Holdings. Base is Value plus the cash for Rebalancing of all Tier Accounts.
	Value *string `json:"value"`
	Base  *string `json:"base"`
	// Tiers are in the order of the order numbers.
	Tiers []rebalancingTierResponse `json:"tiers"`
	// NoTier is the group of the Holdings with no Tier. It is nil when the response is not available.
	NoTier *rebalancingNoTierResponse `json:"noTier"`
	// UnpricedHoldings are the Holdings with no price. They are not part of any value, share or Gap.
	UnpricedHoldings []rebalancingUnpricedResponse `json:"unpricedHoldings"`
}

// rebalancingRateResponse is the Exchange Rate of the response.
type rebalancingRateResponse struct {
	Currency string `json:"currency"`
	// Rate is the PLN amount for 1 unit. Date is the table date as YYYY-MM-DD.
	Rate string `json:"rate"`
	Date string `json:"date"`
}

// rebalancingAccountResponse is one Tier Account. The amounts are in the Account currency.
type rebalancingAccountResponse struct {
	AccountID int64  `json:"accountId"`
	Name      string `json:"name"`
	Currency  string `json:"currency"`
	// FreeCash is the cash of the Account. Cash is the cash for Rebalancing: the cash parameter or FreeCash.
	FreeCash string `json:"freeCash"`
	Cash     string `json:"cash"`
	// PlannedPurchases is the sum of the Planned Purchases of the Account. UnusedCash is Cash − PlannedPurchases.
	PlannedPurchases string `json:"plannedPurchases"`
	UnusedCash       string `json:"unusedCash"`
}

// rebalancingPartResponse holds the amounts of a part in PLN and its shares.
type rebalancingPartResponse struct {
	Value       string `json:"value"`
	ActualShare string `json:"actualShare"`
	TargetShare string `json:"targetShare"`
	TargetValue string `json:"targetValue"`
	Gap         string `json:"gap"`
}

// rebalancingPurchaseResponse is a Planned Purchase, or a sum of them, in the currency of its Account.
// AccountID and Amount are nil when the tool has no Tier Account in the currency.
type rebalancingPurchaseResponse struct {
	AccountID *int64  `json:"accountId"`
	Currency  string  `json:"currency"`
	Amount    *string `json:"amount"`
}

// rebalancingTierResponse is one Tier.
type rebalancingTierResponse struct {
	Order int    `json:"order"`
	Name  string `json:"name"`
	rebalancingPartResponse
	// PlannedPurchases are the sums of the Planned Purchases of the Tier, one for each Account for purchases.
	PlannedPurchases []rebalancingPurchaseResponse `json:"plannedPurchases"`
	// InstrumentTargets are in ticker order.
	InstrumentTargets []rebalancingTargetResponse   `json:"instrumentTargets"`
	SharedPart        rebalancingSharedPartResponse `json:"sharedPart"`
}

// rebalancingTargetResponse is an Instrument Target.
type rebalancingTargetResponse struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// NoValue tells that the Instrument has Holdings, but none with a price.
	NoValue bool `json:"noValue"`
	rebalancingPartResponse
	PlannedPurchase rebalancingPurchaseResponse `json:"plannedPurchase"`
}

// rebalancingSharedPartResponse is the Shared Part of a Tier. Name is empty if the user gave none.
type rebalancingSharedPartResponse struct {
	Name string `json:"name"`
	rebalancingPartResponse
	PlannedPurchase rebalancingPurchaseResponse `json:"plannedPurchase"`
	// Instruments are the Instruments of the Shared Part with a Holding, in ticker order. Share is the share of the
	// Shared Part.
	Instruments []rebalancingHoldingResponse `json:"instruments"`
}

// rebalancingHoldingResponse is an Instrument in a Shared Part or in the "No Tier" group, with all Tier Accounts
// together. Value and Share are nil when NoValue is true.
type rebalancingHoldingResponse struct {
	Ticker  string  `json:"ticker"`
	Name    string  `json:"name"`
	NoValue bool    `json:"noValue"`
	Value   *string `json:"value"`
	Share   *string `json:"share"`
}

// rebalancingNoTierResponse is the "No Tier" group. Its Target Share is 0 and it has no Planned Purchase.
type rebalancingNoTierResponse struct {
	rebalancingPartResponse
	// Holdings are in ticker order. Share is the share of the group.
	Holdings []rebalancingHoldingResponse `json:"holdings"`
}

// rebalancingUnpricedResponse is a Holding with no price.
type rebalancingUnpricedResponse struct {
	AccountID int64  `json:"accountId"`
	Ticker    string `json:"ticker"`
	Name      string `json:"name"`
}

// allAccountsRebalancing returns the Rebalancing table of the Tier Accounts. Spec #103, section "Rebalancing API".
//
// The optional query parameter "cash" sets the cash for Rebalancing of one Tier Account in the Account currency.
// Its form is "<account ID>:<amount>", for example "?cash=1:5000&cash=2:1200.50". Give one parameter for each
// Account. An Account with no parameter uses its free cash. A parameter that is not valid, a negative amount, two
// parameters for one Account or an Account that is not a Tier Account gives 400.
func (a *api) allAccountsRebalancing(w http.ResponseWriter, r *http.Request) {
	cash, err := parseRebalancingCash(r.URL.Query()["cash"])
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := r.Context()
	views, err := a.allAccountViews(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	tiers, err := a.store.Tiers(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	latest, err := a.rates.LatestRates(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	tierViews := slices.DeleteFunc(views, func(v accountView) bool {
		return !tier.IsTierAccount(v.account.Type, v.account.Provider, v.account.Currency)
	})
	for id := range cash {
		if !slices.ContainsFunc(tierViews, func(v accountView) bool { return v.account.ID == id }) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("account %d of the cash parameter is not a Tier Account", id))
			return
		}
	}
	hasCurrency := func(c string) bool {
		return slices.ContainsFunc(tierViews, func(v accountView) bool { return v.account.Currency == c })
	}

	resp := rebalancingResponse{
		Available: true, Warnings: []string{}, Accounts: []rebalancingAccountResponse{}, MissingAccounts: []string{},
		Tiers: []rebalancingTierResponse{}, UnpricedHoldings: []rebalancingUnpricedResponse{},
	}
	usd, hasUSD := latest[rebalancing.USD]
	if hasUSD {
		resp.ExchangeRate = &rebalancingRateResponse{
			Currency: rebalancing.USD, Rate: usd.Mid.String(), Date: usd.Date.Format(time.DateOnly),
		}
	}
	for _, c := range []string{rebalancing.PLN, rebalancing.USD} {
		if !hasCurrency(c) {
			resp.MissingAccounts = append(resp.MissingAccounts, c)
		}
	}
	if !hasUSD && hasCurrency(rebalancing.USD) {
		resp.Available = false
		resp.Warnings = append(resp.Warnings, noRateWarning(rebalancing.USD))
		writeJSON(w, http.StatusOK, resp)
		return
	}

	in := rebalancing.Input{
		Accounts:    make([]rebalancing.Account, 0, len(tierViews)),
		Tiers:       tiers,
		Instruments: make(map[string]rebalancing.Instrument, len(instruments)),
	}
	for ticker, inst := range instruments {
		in.Instruments[ticker] = rebalancing.Instrument{Name: inst.Name, QuoteCurrency: inst.QuoteCurrency}
	}
	for _, v := range tierViews {
		acc := rebalancing.Account{ID: v.account.ID, Currency: v.account.Currency, Rate: decimal.NewFromInt(1), Cash: v.state.Cash}
		if acc.Currency == rebalancing.USD {
			acc.Rate = usd.Mid
		}
		if c, ok := cash[acc.ID]; ok {
			acc.Cash = c
		}
		for _, h := range v.state.Holdings {
			acc.Holdings = append(acc.Holdings, rebalancing.Holding{Ticker: h.Instrument.Ticker, HasValue: h.HasValue(), Value: h.Value})
		}
		in.Accounts = append(in.Accounts, acc)
	}
	result := rebalancing.Calculate(in)

	resp.Value, resp.Base = new(formatAmount(result.Value)), new(formatAmount(result.Base))
	for i, acc := range result.Accounts {
		v := tierViews[i]
		resp.Accounts = append(resp.Accounts, rebalancingAccountResponse{
			AccountID: acc.ID, Name: v.account.Name, Currency: acc.Currency, FreeCash: formatAmount(v.state.Cash),
			Cash: formatAmount(acc.Cash), PlannedPurchases: formatAmount(acc.PlannedPurchases), UnusedCash: formatAmount(acc.UnusedCash),
		})
	}
	for _, t := range result.Tiers {
		resp.Tiers = append(resp.Tiers, toRebalancingTier(t))
	}
	resp.NoTier = &rebalancingNoTierResponse{
		rebalancingPartResponse: toRebalancingPart(result.NoTier.Part),
		Holdings:                toRebalancingHoldings(result.NoTier.Holdings),
	}
	for _, u := range result.Unpriced {
		resp.UnpricedHoldings = append(resp.UnpricedHoldings, rebalancingUnpricedResponse{
			AccountID: u.AccountID, Ticker: u.Ticker, Name: instruments[u.Ticker].Name,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// parseRebalancingCash reads the cash parameters as amounts by Account ID.
func parseRebalancingCash(params []string) (map[int64]decimal.Decimal, error) {
	cash := make(map[int64]decimal.Decimal, len(params))
	for _, p := range params {
		idText, amountText, ok := strings.Cut(p, ":")
		id, idErr := strconv.ParseInt(idText, 10, 64)
		amount, amountErr := decimal.NewFromString(amountText)
		if !ok || idErr != nil || amountErr != nil {
			return nil, fmt.Errorf("the cash parameter %q is not <account ID>:<amount>", p)
		}
		if amount.IsNegative() {
			return nil, fmt.Errorf("the cash parameter %q has a negative amount", p)
		}
		if _, dup := cash[id]; dup {
			return nil, fmt.Errorf("the cash parameter has account %d more than one time", id)
		}
		cash[id] = amount
	}
	return cash, nil
}

// toRebalancingTier returns the response item of the Tier.
func toRebalancingTier(t rebalancing.Tier) rebalancingTierResponse {
	resp := rebalancingTierResponse{
		Order: t.Order, Name: t.Name, rebalancingPartResponse: toRebalancingPart(t.Part),
		PlannedPurchases:  make([]rebalancingPurchaseResponse, 0, len(t.Purchases)),
		InstrumentTargets: make([]rebalancingTargetResponse, 0, len(t.InstrumentTargets)),
		SharedPart: rebalancingSharedPartResponse{
			Name: t.SharedPart.Name, rebalancingPartResponse: toRebalancingPart(t.SharedPart.Part),
			PlannedPurchase: toRebalancingPurchase(t.SharedPart.Purchase),
			Instruments:     toRebalancingHoldings(t.SharedPart.Instruments),
		},
	}
	for _, p := range t.Purchases {
		resp.PlannedPurchases = append(resp.PlannedPurchases, toRebalancingPurchase(p))
	}
	for _, target := range t.InstrumentTargets {
		resp.InstrumentTargets = append(resp.InstrumentTargets, rebalancingTargetResponse{
			Ticker: target.Ticker, Name: target.Name, NoValue: target.NoValue,
			rebalancingPartResponse: toRebalancingPart(target.Part),
			PlannedPurchase:         toRebalancingPurchase(target.Purchase),
		})
	}
	return resp
}

// toRebalancingPart returns the amounts as decimal text.
func toRebalancingPart(p rebalancing.Part) rebalancingPartResponse {
	return rebalancingPartResponse{
		Value: formatAmount(p.Value), ActualShare: p.ActualShare.String(), TargetShare: p.TargetShare.String(),
		TargetValue: formatAmount(p.TargetValue), Gap: formatAmount(p.Gap),
	}
}

// toRebalancingPurchase returns the Planned Purchase as decimal text.
func toRebalancingPurchase(p rebalancing.Purchase) rebalancingPurchaseResponse {
	resp := rebalancingPurchaseResponse{Currency: p.Currency}
	if p.HasAccount {
		resp.AccountID = new(p.AccountID)
		resp.Amount = new(formatAmount(p.Amount))
	}
	return resp
}

// toRebalancingHoldings returns the Holdings of a group as decimal text.
func toRebalancingHoldings(holdings []rebalancing.GroupHolding) []rebalancingHoldingResponse {
	list := make([]rebalancingHoldingResponse, 0, len(holdings))
	for _, h := range holdings {
		item := rebalancingHoldingResponse{Ticker: h.Ticker, Name: h.Name, NoValue: h.NoValue}
		if !h.NoValue {
			item.Value = new(formatAmount(h.Value))
			item.Share = new(h.Share.String())
		}
		list = append(list, item)
	}
	return list
}
