package httpapi

import (
	"context"
	"encoding/json"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
)

// holdingsParameters is the JSON Schema of the arguments of get_holdings.
const holdingsParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "instrument": {"type": "string", "description": "The broker ticker of the Instrument, for example \"NVDA.US\". Omit it for all Instruments."}
  },
  "additionalProperties": false
}`

// holdingsArguments are the arguments of get_holdings.
type holdingsArguments struct {
	// AccountID is the Account. Nil means all Accounts.
	AccountID *int64 `json:"accountId"`
	// Instrument is the broker ticker. Empty means all Instruments.
	Instrument string `json:"instrument"`
}

// holdingsResult is the result of get_holdings.
type holdingsResult struct {
	Scope string `json:"scope"`
	// Instrument is the broker ticker of the filter. It is empty if the result has no filter.
	Instrument string                  `json:"instrument,omitempty"`
	Accounts   []holdingsAccountResult `json:"accounts"`
}

// holdingsAccountResult is the part of the get_holdings result for one Account.
type holdingsAccountResult struct {
	AccountID   int64           `json:"accountId"`
	AccountName string          `json:"accountName"`
	Currency    string          `json:"currency"`
	Holdings    []holdingResult `json:"holdings"`
}

// holdingResult is one Holding in the get_holdings result. Amounts and the volume are decimal text.
// The price fields are nil when the Holding has no value. Then NoValueReason tells the reason.
type holdingResult struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	Volume string `json:"volume"`
	// Currency is the currency of Cost, Value and UnrealizedProfit: the Account currency.
	Currency string `json:"currency"`
	Cost     string `json:"cost"`
	HasValue bool   `json:"hasValue"`
	// Price is the close price in QuoteCurrency.
	Price                   *string `json:"price"`
	QuoteCurrency           *string `json:"quoteCurrency"`
	PriceDate               *string `json:"priceDate"`
	Value                   *string `json:"value"`
	UnrealizedProfit        *string `json:"unrealizedProfit"`
	UnrealizedProfitPercent *string `json:"unrealizedProfitPercent"`
	NoValueReason           *string `json:"noValueReason"`
}

// holdingsFunction returns get_holdings: the Holdings of one Account or of all Accounts.
func (a *api) holdingsFunction() assistant.Function {
	return assistant.Function{
		Definition: assistant.FunctionDefinition{
			Name: "get_holdings",
			Description: "Returns the Holdings of one Account or of all Accounts, optionally of one Instrument. " +
				"For each Holding: the volume, the cost, the price, the value and the Unrealized Profit. " +
				"A Holding with no known value has hasValue false and the reason.",
			Parameters: json.RawMessage(holdingsParameters),
		},
		Run: a.runHoldings,
	}
}

// runHoldings returns the result of get_holdings. It uses the same calculation as GET /api/accounts/{id}/holdings.
func (a *api) runHoldings(ctx context.Context, arguments json.RawMessage) (any, error) {
	var args holdingsArguments
	if err := assistant.DecodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	accounts, scope, err := a.assistantScope(ctx, args.AccountID)
	if err != nil {
		return nil, err
	}
	ticker := ""
	if args.Instrument != "" {
		if ticker, err = a.assistantInstrument(ctx, args.Instrument); err != nil {
			return nil, err
		}
	}

	result := holdingsResult{Scope: scope, Instrument: ticker, Accounts: make([]holdingsAccountResult, 0, len(accounts))}
	for _, acc := range accounts {
		view, err := a.readAccountView(ctx, acc)
		if err != nil {
			return nil, err
		}
		part := holdingsAccountResult{AccountID: acc.ID, AccountName: acc.Name, Currency: acc.Currency, Holdings: []holdingResult{}}
		for _, h := range view.state.Holdings {
			if ticker != "" && h.Instrument.Ticker != ticker {
				continue
			}
			// The Price Warnings are not a part of the result.
			row := view.holdingRow(h, nil)
			part.Holdings = append(part.Holdings, holdingResult{
				Ticker: row.Ticker, Name: row.Name, Volume: row.Volume, Currency: acc.Currency, Cost: row.Cost,
				HasValue: row.Value != nil, Price: row.Price, QuoteCurrency: row.QuoteCurrency, PriceDate: row.PriceDate,
				Value: row.Value, UnrealizedProfit: row.UnrealizedProfit, UnrealizedProfitPercent: row.UnrealizedProfitPercent,
				NoValueReason: row.Note,
			})
		}
		result.Accounts = append(result.Accounts, part)
	}
	return result, nil
}
