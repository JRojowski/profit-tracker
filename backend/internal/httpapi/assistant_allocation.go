package httpapi

import (
	"context"
	"encoding/json"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ikze"
	"github.com/jrojowski/profit-tracker/backend/internal/importer"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// allocationParameters is the JSON Schema of the arguments of get_allocation_and_net_deposits.
const allocationParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "year": {"type": "integer", "description": "The calendar year of the IKZE data. Omit it for the current year."}
  },
  "additionalProperties": false
}`

// The years that get_allocation_and_net_deposits accepts for the IKZE data.
const (
	minIKZEYear = 2000
	maxIKZEYear = 2100
)

// allocationArguments are the arguments of get_allocation_and_net_deposits.
type allocationArguments struct {
	// AccountID is the Account. Nil means all Accounts.
	AccountID *int64 `json:"accountId"`
	// Year is the calendar year of the IKZE data. Nil means the current year.
	Year *int `json:"year"`
}

// allocationFunctionResult is the result of get_allocation_and_net_deposits.
type allocationFunctionResult struct {
	Scope string `json:"scope"`
	// Currency is the currency of NetDeposits and of the Allocation: the Account currency, or PLN for all Accounts.
	Currency string `json:"currency"`
	// NetDeposits is nil for all Accounts when the tool has no total in PLN. Each Account still has its own amount.
	NetDeposits *string                   `json:"netDeposits"`
	Allocation  allocationResponse        `json:"allocation"`
	Accounts    []allocationAccountResult `json:"accounts"`
}

// allocationAccountResult is the part of the get_allocation_and_net_deposits result for one Account.
type allocationAccountResult struct {
	AccountID   int64  `json:"accountId"`
	AccountName string `json:"accountName"`
	Type        string `json:"type"`
	Currency    string `json:"currency"`
	// NetDeposits is in the Account currency.
	NetDeposits string `json:"netDeposits"`
	// IKZE is nil for an Account that is not IKZE.
	IKZE *ikzeYearResult `json:"ikze,omitempty"`
}

// ikzeYearResult is the IKZE Contributions of one year against the IKZE Limit, in PLN, as decimal text.
// Limit, Unused and OverLimit are nil for a year with no IKZE Limit in the tool. An IKZE Account that is not
// in PLN has only Note.
type ikzeYearResult struct {
	Year          int     `json:"year"`
	Contributions *string `json:"contributions,omitempty"`
	Limit         *string `json:"limit,omitempty"`
	Unused        *string `json:"unused,omitempty"`
	OverLimit     *string `json:"overLimit,omitempty"`
	Note          *string `json:"note,omitempty"`
}

// allocationFunction returns get_allocation_and_net_deposits: the Allocation and the Net Deposits of one Account
// or of all Accounts, with the IKZE Limit of a year for each IKZE Account.
func (a *api) allocationFunction() assistant.Function {
	return assistant.Function{
		Definition: assistant.FunctionDefinition{
			Name: "get_allocation_and_net_deposits",
			Description: "Returns the Allocation parts with their shares, and the Net Deposits, of one Account or of all Accounts. " +
				"For an IKZE Account it also returns the IKZE Contributions and the IKZE Limit of a year. " +
				"The Allocation of all Accounts and the total of the Net Deposits are in PLN. " +
				"Each Account also has its Net Deposits in its own currency.",
			Parameters: json.RawMessage(allocationParameters),
		},
		Run: a.runAllocation,
	}
}

// runAllocation returns the result of get_allocation_and_net_deposits. It uses the calculations behind
// GET /api/accounts/{id}/allocation, GET /api/all-accounts/allocation, the summary endpoints and
// GET /api/accounts/{id}/ikze-limit.
func (a *api) runAllocation(ctx context.Context, arguments json.RawMessage) (any, error) {
	var args allocationArguments
	if err := assistant.DecodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	if args.Year != nil && (*args.Year < minIKZEYear || *args.Year > maxIKZEYear) {
		return nil, assistant.Argumentf("year %d is not valid. Use a year from %d to %d. Omit year for the current year.",
			*args.Year, minIKZEYear, maxIKZEYear)
	}
	accounts, scope, err := a.assistantScope(ctx, args.AccountID)
	if err != nil {
		return nil, err
	}
	// The IKZE year is the calendar year in Polish time.
	year := exchange.StartOfDayInWarsaw(a.now()).Year()
	if args.Year != nil {
		year = *args.Year
	}

	views := make([]accountView, 0, len(accounts))
	rows := make([]allocationAccountResult, 0, len(accounts))
	for _, acc := range accounts {
		view, err := a.readAccountView(ctx, acc)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
		row := allocationAccountResult{
			AccountID: acc.ID, AccountName: acc.Name, Type: acc.Type, Currency: acc.Currency,
			NetDeposits: formatRoundedAmount(view.state.NetDeposits),
		}
		if acc.Type == importer.TypeIKZE {
			if row.IKZE, err = a.ikzeYearOf(ctx, view, year); err != nil {
				return nil, err
			}
		}
		rows = append(rows, row)
	}

	result := allocationFunctionResult{Scope: scope, Accounts: rows}
	if args.AccountID != nil {
		result.Currency = accounts[0].Currency
		result.NetDeposits = &rows[0].NetDeposits
		result.Allocation = roundedAllocation(a.allocation(views, result.Currency))
		return result, nil
	}
	summary, err := a.allAccountsSummaryOf(ctx)
	if err != nil {
		return nil, err
	}
	result.Currency = ledger.PLN
	result.NetDeposits = roundAmountPointer(summary.NetDeposits)
	result.Allocation = roundedAllocation(a.allocation(views, ledger.PLN))
	return result, nil
}

// ikzeYearOf returns the IKZE data of the year for the Account of the view.
func (a *api) ikzeYearOf(ctx context.Context, view accountView, year int) (*ikzeYearResult, error) {
	acc := view.account
	if acc.Currency != ledger.PLN {
		return &ikzeYearResult{Year: year, Note: new(ikzeCurrencyMessage(acc))}, nil
	}
	l, err := a.store.Ledger(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	row := toIKZELimitRow(ikze.ForYear(l.CashOperations, ikze.BusinessLimit, year, exchange.StartOfDayInWarsaw(a.now()).Location()))
	return &ikzeYearResult{
		Year: row.Year, Contributions: new(roundAmountText(row.Contributions)), Limit: roundAmountPointer(row.Limit),
		Unused: roundAmountPointer(row.Unused), OverLimit: roundAmountPointer(row.OverLimit),
	}, nil
}

// roundedAllocation rounds the amounts of the Allocation to 2 decimal places. The shares are percentages and stay as they are.
func roundedAllocation(resp allocationResponse) allocationResponse {
	resp.Total = roundAmountPointer(resp.Total)
	resp.ByInstrument.Holdings = roundedParts(resp.ByInstrument.Holdings)
	if c := resp.ByInstrument.Cash; c != nil {
		resp.ByInstrument.Cash = new(roundedPart(*c))
	}
	if o := resp.ByInstrument.Other; o != nil {
		resp.ByInstrument.Other = &allocationOtherDTO{
			Count: o.Count, Value: roundAmountText(o.Value), Share: o.Share, Holdings: roundedParts(o.Holdings),
		}
	}
	currencies := make([]allocationCurrencyPartDTO, 0, len(resp.ByQuoteCurrency))
	for _, c := range resp.ByQuoteCurrency {
		c.Value = roundAmountText(c.Value)
		currencies = append(currencies, c)
	}
	resp.ByQuoteCurrency = currencies
	return resp
}

func roundedPart(p allocationPartDTO) allocationPartDTO {
	p.Value = roundAmountText(p.Value)
	return p
}

func roundedParts(parts []allocationPartDTO) []allocationPartDTO {
	rounded := make([]allocationPartDTO, 0, len(parts))
	for _, p := range parts {
		rounded = append(rounded, roundedPart(p))
	}
	return rounded
}
