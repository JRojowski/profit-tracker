package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/importer"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// assistantFunctions returns the functions that the model can call. ADR 0007: each function is read-only and
// returns a result that the backend calculates. A result never holds a broker comment, a broker account number
// or a raw row of an export.
func (a *api) assistantFunctions() []assistant.Function {
	return []assistant.Function{
		a.holdingsFunction(),
		a.resultsFunction(),
		a.allocationFunction(),
		a.closedPositionsFunction(),
		a.incomeFunction(),
	}
}

// averageCostAmount is an amount of a function result that only an Account at the Average Cost has (ADR 0009):
// the Average Cost, the Rewards and the sum of the fees. Use it with the omitzero option. The zero value is not set,
// and JSON leaves it out, so the result of an XTB Account does not change. A set amount with no known value is JSON null.
type averageCostAmount struct {
	set    bool
	amount *string
}

// averageCostAmountOf returns the set amount. amount is nil if the amount has no known value.
func averageCostAmountOf(amount *string) averageCostAmount {
	return averageCostAmount{set: true, amount: amount}
}

// IsZero tells that the amount is not set. The omitzero option calls it.
func (a averageCostAmount) IsZero() bool {
	return !a.set
}

// MarshalJSON writes the amount as a JSON string, or null if it has no known value.
func (a averageCostAmount) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.amount)
}

// usesAverageCost tells that the Account keeps its Holdings at the Average Cost, not in Lots (ADR 0009).
func usesAverageCost(acc store.Account) bool {
	return acc.Provider == importer.ProviderKraken
}

// allAccountsScope is the scope of a result for all Accounts.
const allAccountsScope = "all Accounts"

// assistantScope returns the Accounts of the scope and the scope as text: the Account with the ID,
// or all Accounts if id is nil. An unknown Account is an ArgumentError.
func (a *api) assistantScope(ctx context.Context, id *int64) ([]store.Account, string, error) {
	if id == nil {
		accounts, err := a.store.Accounts(ctx)
		if err != nil {
			return nil, "", err
		}
		return accounts, allAccountsScope, nil
	}
	acc, err := a.store.Account(ctx, *id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "", a.unknownAccount(ctx, *id)
	}
	if err != nil {
		return nil, "", err
	}
	return []store.Account{acc}, fmt.Sprintf("Account %q", acc.Name), nil
}

// unknownAccount returns the ArgumentError for an unknown Account ID. It lists the known Accounts.
func (a *api) unknownAccount(ctx context.Context, id int64) error {
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		return err
	}
	known := make([]string, 0, len(accounts))
	for _, acc := range accounts {
		known = append(known, fmt.Sprintf("%d (%q)", acc.ID, acc.Name))
	}
	return assistant.Argumentf("unknown Account %d. The Account IDs are: %s. Omit accountId for all Accounts.",
		id, strings.Join(known, ", "))
}

// assistantInstrument returns the ticker of the Instrument. The match ignores the case of the letters.
// An unknown Instrument is an ArgumentError.
func (a *api) assistantInstrument(ctx context.Context, ticker string) (string, error) {
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		return "", err
	}
	known := make([]string, 0, len(instruments))
	for t := range instruments {
		if strings.EqualFold(t, ticker) {
			return t, nil
		}
		known = append(known, t)
	}
	slices.Sort(known)
	return "", assistant.Argumentf("unknown Instrument %q. Use the ticker or the asset code. The known Instruments are: %s.",
		ticker, strings.Join(known, ", "))
}
