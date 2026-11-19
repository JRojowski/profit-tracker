package httpapi

import (
	"context"
	"encoding/json"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

// resultsParameters is the JSON Schema of the arguments of get_results.
const resultsParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "from": {"type": "string", "description": "The first day of a period, as YYYY-MM-DD. The day is included. Give from and to together, or neither."},
    "to": {"type": "string", "description": "The last day of a period, as YYYY-MM-DD. The day is included. Give from and to together, or neither."}
  },
  "additionalProperties": false
}`

// resultsArguments are the arguments of get_results.
type resultsArguments struct {
	// AccountID is the Account. Nil means all Accounts.
	AccountID *int64 `json:"accountId"`
	// From and To are the period, as YYYY-MM-DD. Both are empty or both are set.
	From string `json:"from"`
	To   string `json:"to"`
}

// resultsResult is the result of get_results. For one Account, the amounts are in the Account currency.
// For all Accounts, they are in PLN. An amount is nil when it is not available, and then Warnings tell why.
type resultsResult struct {
	Scope string `json:"scope"`
	// AccountID and AccountName are empty for all Accounts.
	AccountID   *int64   `json:"accountId,omitempty"`
	AccountName string   `json:"accountName,omitempty"`
	Currency    string   `json:"currency"`
	Available   bool     `json:"available"`
	Warnings    []string `json:"warnings"`

	Value            *string `json:"value"`
	Cash             *string `json:"cash"`
	NetDeposits      *string `json:"netDeposits"`
	TotalResult      *string `json:"totalResult"`
	RealizedProfit   *string `json:"realizedProfit"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	NetDividends     *string `json:"netDividends"`
	NetInterest      *string `json:"netInterest"`
	Fees             *string `json:"fees"`
	// CurrencyResult exists only for all Accounts. The tool calculates it only in PLN (ADR 0004).
	CurrencyResult   *string `json:"currencyResult,omitempty"`
	PriceDate        *string `json:"priceDate"`
	UnpricedHoldings int     `json:"unpricedHoldings"`

	SimpleReturn       returnResult           `json:"simpleReturn"`
	AnnualReturn       returnResult           `json:"annualReturn"`
	TimeWeightedReturn timeWeightedReturnsOut `json:"timeWeightedReturn"`
	// ValuedAtCost are the tickers of the Instruments that the Value History valued at cost on one or more days.
	ValuedAtCost []string `json:"valuedAtCost"`
}

// returnResult is one return as a percentage in decimal text. Percent is nil when the return is not available.
// Then Reason tells why.
type returnResult struct {
	Percent *string `json:"percent"`
	Reason  *string `json:"reason"`
}

// timeWeightedReturnsOut are the Time-Weighted Returns of the fixed periods of the tool.
type timeWeightedReturnsOut struct {
	OneMonth    returnResult `json:"1M"`
	ThreeMonths returnResult `json:"3M"`
	YearToDate  returnResult `json:"YTD"`
	OneYear     returnResult `json:"1Y"`
	All         returnResult `json:"All"`
}

// allValueHistoryNote is the reason of a Time-Weighted Return when the Value History of all Accounts is not available.
const allValueHistoryNote = "The Value History of all Accounts is not available. The warnings tell why."

// resultsFunction returns get_results: the Total Result, its parts and the returns of one Account or of all Accounts.
func (a *api) resultsFunction() assistant.Function {
	return assistant.Function{
		Definition: assistant.FunctionDefinition{
			Name: "get_results",
			Description: "Returns the value, the cash, the Net Deposits, the Total Result and its parts " +
				"(Realized Profit, Unrealized Profit, net dividends, net interest, fees, Currency Result), " +
				"the Simple Return, the Annual Return and the Time-Weighted Return of one Account or of all Accounts. " +
				"A return that is not available has no number and a reason. " +
				"For all Accounts the amounts are in PLN. For one Account they are in the Account currency. " +
				"With the arguments from and to (a period, both days included) it returns only the Total Result of the period " +
				"and the values that give it: the value at the start and at the end, and the Net Deposits of the period. " +
				"It has no returns and no parts of the Total Result then.",
			Parameters: json.RawMessage(resultsParameters),
		},
		Run: a.runResults,
	}
}

// runResults returns the result of get_results. It uses the calculations behind GET /api/accounts/{id}/summary
// and GET /api/accounts/{id}/value-history, or behind GET /api/all-accounts/summary
// and GET /api/all-accounts/value-history.
func (a *api) runResults(ctx context.Context, arguments json.RawMessage) (any, error) {
	var args resultsArguments
	if err := assistant.DecodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if (args.From == "") != (args.To == "") {
		return nil, assistant.Argumentf("give from and to together, or neither. A period needs its first day and its last day.")
	}
	accounts, scope, err := a.assistantScope(ctx, args.AccountID)
	if err != nil {
		return nil, err
	}
	if args.From != "" {
		return a.periodResults(ctx, args, accounts, scope)
	}
	if args.AccountID == nil {
		return a.allAccountsResults(ctx, scope)
	}

	acc := accounts[0]
	view, err := a.readAccountView(ctx, acc)
	if err != nil {
		return nil, err
	}
	history, err := a.accountValueHistoryBody(ctx, acc)
	if err != nil {
		return nil, err
	}
	s := a.summaryOf(view)
	return resultsResult{
		Scope: scope, AccountID: &acc.ID, AccountName: acc.Name, Currency: acc.Currency, Available: true, Warnings: []string{},
		Value: new(roundAmountText(s.Value)), Cash: new(roundAmountText(s.Cash)), NetDeposits: new(roundAmountText(s.NetDeposits)),
		TotalResult: new(roundAmountText(s.TotalResult)), RealizedProfit: new(roundAmountText(s.RealizedProfit)),
		UnrealizedProfit: new(roundAmountText(s.UnrealizedProfit)), NetDividends: new(roundAmountText(s.NetDividends)),
		NetInterest: new(roundAmountText(s.NetInterest)), Fees: new(roundAmountText(s.Fees)), PriceDate: s.PriceDate, UnpricedHoldings: s.UnpricedHoldings,
		SimpleReturn:       returnResult{Percent: s.SimpleReturn, Reason: s.SimpleReturnNote},
		AnnualReturn:       returnResult{Percent: s.AnnualReturn, Reason: s.AnnualReturnNote},
		TimeWeightedReturn: timeWeightedOf(history.TimeWeightedReturn, ""),
		ValuedAtCost:       tickersOf(history.ValuedAtCost),
	}, nil
}

// allAccountsResults returns the result of get_results for all Accounts, in PLN.
func (a *api) allAccountsResults(ctx context.Context, scope string) (resultsResult, error) {
	summary, err := a.allAccountsSummaryOf(ctx)
	if err != nil {
		return resultsResult{}, err
	}
	history, err := a.allValueHistoryOf(ctx)
	if err != nil {
		return resultsResult{}, err
	}
	unavailable := ""
	if !history.Available {
		unavailable = allValueHistoryNote
	}
	return resultsResult{
		Scope: scope, Currency: "PLN", Available: summary.Available, Warnings: summary.Warnings,
		Value: roundAmountPointer(summary.Value), Cash: roundAmountPointer(summary.Cash),
		NetDeposits: roundAmountPointer(summary.NetDeposits), TotalResult: roundAmountPointer(summary.TotalResult),
		RealizedProfit: roundAmountPointer(summary.RealizedProfit), UnrealizedProfit: roundAmountPointer(summary.UnrealizedProfit),
		NetDividends: roundAmountPointer(summary.NetDividends), NetInterest: roundAmountPointer(summary.NetInterest),
		Fees: roundAmountPointer(summary.Fees), CurrencyResult: roundAmountPointer(summary.CurrencyResult), PriceDate: summary.PriceDate, UnpricedHoldings: summary.UnpricedHoldings,
		SimpleReturn:       returnResult{Percent: summary.SimpleReturn, Reason: summary.SimpleReturnNote},
		AnnualReturn:       returnResult{Percent: summary.AnnualReturn, Reason: summary.AnnualReturnNote},
		TimeWeightedReturn: timeWeightedOf(history.TimeWeightedReturn, unavailable),
		ValuedAtCost:       tickersOf(history.ValuedAtCost),
	}, nil
}

// timeWeightedOf returns the Time-Weighted Returns with a reason for each period that has no number.
// The reason is unavailable if it is not empty. Otherwise it is the reason that the Value History has no day to start from.
func timeWeightedOf(twr timeWeightedResponse, unavailable string) timeWeightedReturnsOut {
	reason := returnNote(returns.Rate{Reason: returns.ReasonNoValueHistory})
	if unavailable != "" {
		reason = &unavailable
	}
	of := func(p *string) returnResult {
		if p != nil {
			return returnResult{Percent: p}
		}
		return returnResult{Reason: reason}
	}
	return timeWeightedReturnsOut{
		OneMonth: of(twr.OneMonth), ThreeMonths: of(twr.ThreeMonths), YearToDate: of(twr.YearToDate),
		OneYear: of(twr.OneYear), All: of(twr.All),
	}
}

// tickersOf returns the tickers of the Instruments. It returns an empty list, not nil, for no Instruments.
func tickersOf(instruments []valuedAtCostResponse) []string {
	tickers := make([]string, 0, len(instruments))
	for _, inst := range instruments {
		tickers = append(tickers, inst.Ticker)
	}
	return tickers
}
