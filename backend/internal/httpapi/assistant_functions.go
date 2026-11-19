package httpapi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// assistantFunctions returns the functions that the model can call. ADR 0007: each function is read-only and
// returns a result that the backend calculates. A result never holds a broker comment, a broker account number
// or a raw row of an export.
func (a *api) assistantFunctions() []assistant.Function {
	return []assistant.Function{
		a.holdingsFunction(),
		a.closedPositionsFunction(),
		a.incomeFunction(),
	}
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

// assistantInstrument returns the broker ticker of the Instrument. The match ignores the case of the letters.
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
	return "", assistant.Argumentf("unknown Instrument %q. Use the broker ticker. The known Instruments are: %s.",
		ticker, strings.Join(known, ", "))
}
