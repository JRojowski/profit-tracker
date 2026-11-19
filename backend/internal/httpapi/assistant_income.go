package httpapi

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// incomeParameters is the JSON Schema of the arguments of get_income_and_fees.
const incomeParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "instrument": {"type": "string", "description": "The broker ticker of the Instrument, for example \"NVDA.US\". Omit it for all Instruments. Interest is not tied to an Instrument, so the result leaves it out."},
    "from": {"type": "string", "description": "The first day of the period, as YYYY-MM-DD. The day is included. Omit it for no first day."},
    "to": {"type": "string", "description": "The last day of the period, as YYYY-MM-DD. The day is included. Omit it for no last day."}
  },
  "additionalProperties": false
}`

// The texts of incomeResult.LeftOut for a filter by Instrument.
const (
	leftOutInterest = "net interest: the broker does not tie interest to an Instrument, so the Instrument filter leaves it out"
	leftOutFees     = "fees: only the fees that the broker ties to the Instrument are counted, fees with no Instrument (for example a Correction) are left out"
)

// incomeArguments are the arguments of get_income_and_fees.
type incomeArguments struct {
	// AccountID is the Account. Nil means all Accounts.
	AccountID *int64 `json:"accountId"`
	// Instrument is the broker ticker. Empty means all Instruments.
	Instrument string `json:"instrument"`
	// From and To are days as YYYY-MM-DD, both included. Empty means no limit.
	From string `json:"from"`
	To   string `json:"to"`
}

// incomeSums are the sums of one currency. Amounts are decimal text in Currency.
type incomeSums struct {
	Currency     string `json:"currency"`
	NetDividends string `json:"netDividends"`
	// NetInterest is nil when the Instrument filter leaves interest out. See incomeResult.LeftOut.
	NetInterest *string `json:"netInterest"`
	Fees        string  `json:"fees"`
}

// incomeAccountResult is the part of the get_income_and_fees result for one Account. Its currency is the Account currency.
type incomeAccountResult struct {
	AccountID   int64  `json:"accountId"`
	AccountName string `json:"accountName"`
	incomeSums
}

// incomeResult is the result of get_income_and_fees.
type incomeResult struct {
	Scope string `json:"scope"`
	// Instrument, From and To are the filters. They are empty if the result has no such filter.
	Instrument string `json:"instrument,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	// Totals has one sum per currency. The sums of two currencies are never added.
	Totals   []incomeSums          `json:"totals"`
	Accounts []incomeAccountResult `json:"accounts"`
	// LeftOut tells the parts that the Instrument filter leaves out. It is empty with no Instrument filter.
	LeftOut []string `json:"leftOut"`
}

// incomeFunction returns get_income_and_fees: the net dividends, the net interest and the fees of one Account or
// of all Accounts, optionally of one Instrument and of one period.
func (a *api) incomeFunction() assistant.Function {
	return assistant.Function{
		Definition: assistant.FunctionDefinition{
			Name: "get_income_and_fees",
			Description: "Returns the net dividends, the net interest and the fees of one Account or of all Accounts, " +
				"optionally of one Instrument and of a period (first day and last day, both included, in Polish time). " +
				"The result has one sum per currency. " +
				"With an Instrument, the result has only the amounts that the broker ties to that Instrument. " +
				"The net interest is not tied to an Instrument, so the result leaves it out (leftOut tells this).",
			Parameters: json.RawMessage(incomeParameters),
		},
		Run: a.runIncome,
	}
}

// runIncome returns the result of get_income_and_fees. It sums the Cash Operations of each Account with
// ledger.SumIncome, the same code that gives the totals in GET /api/accounts/{id}/summary.
func (a *api) runIncome(ctx context.Context, arguments json.RawMessage) (any, error) {
	var args incomeArguments
	if err := assistant.DecodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	filter, err := incomePeriodFilter(args)
	if err != nil {
		return nil, err
	}
	accounts, scope, err := a.assistantScope(ctx, args.AccountID)
	if err != nil {
		return nil, err
	}
	if args.Instrument != "" {
		if filter.Ticker, err = a.assistantInstrument(ctx, args.Instrument); err != nil {
			return nil, err
		}
	}

	result := incomeResult{
		Scope: scope, Instrument: filter.Ticker, From: args.From, To: args.To,
		Totals: []incomeSums{}, Accounts: make([]incomeAccountResult, 0, len(accounts)), LeftOut: []string{},
	}
	if filter.Ticker != "" {
		result.LeftOut = []string{leftOutInterest, leftOutFees}
	}
	totals := map[string]ledger.Income{}
	for _, acc := range accounts {
		l, err := a.store.Ledger(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		income := ledger.SumIncome(l.CashOperations, filter)
		result.Accounts = append(result.Accounts, incomeAccountResult{
			AccountID: acc.ID, AccountName: acc.Name, incomeSums: incomeSumsOf(acc.Currency, income, filter.Ticker != ""),
		})
		sum, ok := totals[acc.Currency]
		if !ok {
			sum = ledger.Income{NetDividends: decimal.Zero, NetInterest: decimal.Zero, Fees: decimal.Zero}
		}
		totals[acc.Currency] = ledger.Income{
			NetDividends: sum.NetDividends.Add(income.NetDividends),
			NetInterest:  sum.NetInterest.Add(income.NetInterest),
			Fees:         sum.Fees.Add(income.Fees),
		}
	}
	currencies := make([]string, 0, len(totals))
	for currency := range totals {
		currencies = append(currencies, currency)
	}
	slices.Sort(currencies)
	for _, currency := range currencies {
		result.Totals = append(result.Totals, incomeSumsOf(currency, totals[currency], filter.Ticker != ""))
	}
	return result, nil
}

// incomeSumsOf returns the sums as decimal text. If interestLeftOut is true, the net interest is nil.
func incomeSumsOf(currency string, income ledger.Income, interestLeftOut bool) incomeSums {
	sums := incomeSums{Currency: currency, NetDividends: formatAmount(income.NetDividends), Fees: formatAmount(income.Fees)}
	if !interestLeftOut {
		sums.NetInterest = new(formatAmount(income.NetInterest))
	}
	return sums
}

// incomeDateLayout is the layout of the arguments from and to.
const incomeDateLayout = time.DateOnly

// incomePeriodFilter returns the filter with the period of the arguments. The day limits are the same as on
// the Closed Positions page: a Cash Operation belongs to the day of its time in Polish time, and both limits
// are included. A date that is not valid, or a first day after the last day, is an ArgumentError.
func incomePeriodFilter(args incomeArguments) (ledger.IncomeFilter, error) {
	var filter ledger.IncomeFilter
	var first, last time.Time
	if args.From != "" {
		day, err := time.Parse(incomeDateLayout, args.From)
		if err != nil {
			return filter, assistant.Argumentf("from %q is not a date. Use the form YYYY-MM-DD, for example 2025-01-31.", args.From)
		}
		first = day
		filter.From = startOfPolishDay(day)
	}
	if args.To != "" {
		day, err := time.Parse(incomeDateLayout, args.To)
		if err != nil {
			return filter, assistant.Argumentf("to %q is not a date. Use the form YYYY-MM-DD, for example 2025-12-31.", args.To)
		}
		last = day
		// The period ends before the start of the next Polish day.
		filter.To = startOfPolishDay(day.AddDate(0, 0, 1))
	}
	if !first.IsZero() && !last.IsZero() && first.After(last) {
		return filter, assistant.Argumentf("from %s is after to %s. The first day must not be after the last day.", args.From, args.To)
	}
	return filter, nil
}

// startOfPolishDay returns the start of the calendar date of day in Polish time. day is a date at midnight UTC.
// Poland is ahead of UTC, so the Polish date of that instant is the same calendar date.
func startOfPolishDay(day time.Time) time.Time {
	return exchange.StartOfDayInWarsaw(day)
}
