package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

// periodResults is the result of get_results with a period. It holds the Total Result of the period and the values
// that give it, and no return. For one Account the amounts are in the Account currency. For all Accounts they are in PLN.
// The amounts are nil when the Value History is not available. Then Reason and Warnings tell why.
type periodResults struct {
	Scope string `json:"scope"`
	// AccountID and AccountName are empty for all Accounts.
	AccountID   *int64   `json:"accountId,omitempty"`
	AccountName string   `json:"accountName,omitempty"`
	Currency    string   `json:"currency"`
	Available   bool     `json:"available"`
	Warnings    []string `json:"warnings"`
	Reason      *string  `json:"reason"`

	// From and To are the period that the model asked for.
	From string `json:"from"`
	To   string `json:"to"`
	// StartDate and EndDate are the days that the calculation used: the values are the values at the end of
	// these days. StartDate is the day before From. EndDate is To, or today if To is after today.
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`

	StartValue  *string `json:"startValue"`
	EndValue    *string `json:"endValue"`
	NetDeposits *string `json:"netDeposits"`
	TotalResult *string `json:"totalResult"`
	// ValuedAtCost are the tickers of the Instruments that the Value History valued at cost on StartDate or on
	// EndDate. If it is not empty, the numbers are not exact.
	ValuedAtCost []string `json:"valuedAtCost"`
}

// periodResults returns the result of get_results for the period of the arguments. It uses the points of
// GET /api/accounts/{id}/value-history, or of GET /api/all-accounts/value-history.
func (a *api) periodResults(ctx context.Context, args resultsArguments, accounts []store.Account, scope string) (any, error) {
	first, last, err := closedPositionsPeriod(args.From, args.To)
	if err != nil {
		return nil, err
	}
	result := periodResults{
		Scope: scope, Currency: "PLN", Warnings: []string{}, From: args.From, To: args.To, ValuedAtCost: []string{},
	}
	var points []valuehistory.Point
	if args.AccountID == nil {
		history, err := a.allValueHistoryOf(ctx)
		if err != nil {
			return nil, err
		}
		if !history.Available {
			reason := allValueHistoryNote
			result.Reason, result.Warnings = &reason, history.Warnings
			return result, nil
		}
		points = history.points
	} else {
		acc := accounts[0]
		history, err := a.accountValueHistoryBody(ctx, acc)
		if err != nil {
			return nil, err
		}
		points = history.points
		result.AccountID, result.AccountName, result.Currency = &acc.ID, acc.Name, acc.Currency
	}

	period, err := valuehistory.TotalResultOfPeriod(points, first, last, exchange.DayInWarsaw(a.now()))
	if errors.Is(err, valuehistory.ErrPeriodAfterToday) {
		return nil, assistant.Argumentf("the period %s to %s is after today. Today is %s.",
			args.From, args.To, exchange.DayInWarsaw(a.now()).Format(time.DateOnly))
	}
	if err != nil {
		return nil, err
	}
	result.Available = true
	result.StartDate, result.EndDate = period.StartDay.Format(time.DateOnly), period.EndDay.Format(time.DateOnly)
	result.StartValue, result.EndValue = new(formatAmount(period.StartValue)), new(formatAmount(period.EndValue))
	result.NetDeposits, result.TotalResult = new(formatAmount(period.NetDeposits)), new(formatAmount(period.TotalResult))
	if period.ValuedAtCost != nil {
		result.ValuedAtCost = period.ValuedAtCost
	}
	return result, nil
}
