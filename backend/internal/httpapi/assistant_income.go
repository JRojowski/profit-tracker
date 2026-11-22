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
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// incomeParameters is the JSON Schema of the arguments of get_income_and_fees.
const incomeParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "instrument": {"type": "string", "description": "The ticker or the asset code of the Instrument, for example \"NVDA.US\" or \"BTC\". Omit it for all Instruments. Interest is not tied to an Instrument, so the result leaves it out."},
    "from": {"type": "string", "description": "The first day of the period, as YYYY-MM-DD. The day is included. Omit it for no first day."},
    "to": {"type": "string", "description": "The last day of the period, as YYYY-MM-DD. The day is included. Omit it for no last day."}
  },
  "additionalProperties": false
}`

// The texts of incomeResult.LeftOut for a filter by Instrument.
const (
	leftOutInterest = "net interest: the broker does not tie interest to an Instrument, so the Instrument filter leaves it out"
	leftOutFees     = "fees: only the fees that the Provider ties to the Instrument are counted, fees with no Instrument (for example a Correction) are left out"
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
	// Rewards and FeeSum exist only for an Account at the Average Cost (Kraken), and for its currency in the totals.
	// FeeSum is the sum of the fees of the crypto purchases. It is a part of the cost of the Holdings, not of Fees.
	// Rewards is null if a Reward has no known value.
	Rewards averageCostAmount `json:"rewards,omitzero"`
	FeeSum  averageCostAmount `json:"feeSum,omitzero"`
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
				"With an Instrument, the result has only the amounts that the Provider ties to that Instrument. " +
				"The net interest is not tied to an Instrument, so the result leaves it out (leftOut tells this). " +
				"For a Kraken Account and its currency, the result also has the Rewards and feeSum. " +
				"feeSum is the sum of the fees of the crypto purchases. It is a part of the cost of the Holdings, not of the fees. " +
				"The Rewards are null if a Reward has no known value.",
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
	cryptoTotals := map[string]ledger.RewardsAndFees{}
	for _, acc := range accounts {
		l, err := a.incomeLedger(ctx, acc)
		if err != nil {
			return nil, err
		}
		income := ledger.SumIncome(l.CashOperations, filter)
		sums := incomeSumsOf(acc.Currency, income, filter.Ticker != "")
		if usesAverageCost(acc) {
			crypto := ledger.SumRewardsAndFees(l, filter)
			setRewardsAndFees(&sums, crypto)
			cryptoTotals[acc.Currency] = addRewardsAndFees(cryptoTotals, acc.Currency, crypto)
		}
		result.Accounts = append(result.Accounts, incomeAccountResult{AccountID: acc.ID, AccountName: acc.Name, incomeSums: sums})
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
		sums := incomeSumsOf(currency, totals[currency], filter.Ticker != "")
		if crypto, ok := cryptoTotals[currency]; ok {
			setRewardsAndFees(&sums, crypto)
		}
		result.Totals = append(result.Totals, sums)
	}
	return result, nil
}

// incomeLedger returns the Ledger of the Account for get_income_and_fees. An Account at the Average Cost also
// needs the closes and the Quote Currencies, because the value of a Reward comes from them (ADR 0009).
func (a *api) incomeLedger(ctx context.Context, acc store.Account) (ledger.Ledger, error) {
	if !usesAverageCost(acc) {
		return a.store.Ledger(ctx, acc.ID)
	}
	in, err := a.readHeroInput(ctx, acc)
	if err != nil {
		return ledger.Ledger{}, err
	}
	return in.ledger, nil
}

// addRewardsAndFees returns the sums of the currency in totals plus sum. A Rewards part with no known value
// makes the total Rewards not known.
func addRewardsAndFees(totals map[string]ledger.RewardsAndFees, currency string, sum ledger.RewardsAndFees) ledger.RewardsAndFees {
	total, ok := totals[currency]
	if !ok {
		return sum
	}
	total.FeeSum = total.FeeSum.Add(sum.FeeSum)
	if !sum.Rewards.Valid {
		total.Rewards = decimal.NullDecimal{}
	}
	if total.Rewards.Valid {
		total.Rewards.Decimal = total.Rewards.Decimal.Add(sum.Rewards.Decimal)
	}
	return total
}

// setRewardsAndFees sets the Rewards and the sum of the fees of the sums, rounded to 2 decimal places.
func setRewardsAndFees(sums *incomeSums, crypto ledger.RewardsAndFees) {
	var rewards *string
	if crypto.Rewards.Valid {
		rewards = new(formatRoundedAmount(crypto.Rewards.Decimal))
	}
	sums.Rewards = averageCostAmountOf(rewards)
	sums.FeeSum = averageCostAmountOf(new(formatRoundedAmount(crypto.FeeSum)))
}

// incomeSumsOf returns the sums as decimal text. If interestLeftOut is true, the net interest is nil.
func incomeSumsOf(currency string, income ledger.Income, interestLeftOut bool) incomeSums {
	sums := incomeSums{Currency: currency, NetDividends: formatRoundedAmount(income.NetDividends), Fees: formatRoundedAmount(income.Fees)}
	if !interestLeftOut {
		sums.NetInterest = new(formatRoundedAmount(income.NetInterest))
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
