package store

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Transfer is one stored side of a Transfer between two Accounts. See ADR 0003.
type Transfer struct {
	AccountID int64
	XTBID     string
	Time      time.Time
	// Amount is positive for a Transfer In and negative for a Transfer Out.
	Amount decimal.Decimal
}

// TransfersOfOtherAccounts returns the stored Transfers of all Accounts except the Account with the ID,
// in time order. An ID of 0, for a new Account, gives the Transfers of all Accounts.
func (s *Store) TransfersOfOtherAccounts(ctx context.Context, accountID int64) ([]Transfer, error) {
	rows, err := queries.New(s.db).TransfersOfOtherAccounts(ctx, queries.TransfersOfOtherAccountsParams{
		Type:      ledger.TypeTransfer,
		AccountID: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("read transfers of the accounts other than %d: %w", accountID, err)
	}
	transfers := make([]Transfer, 0, len(rows))
	for _, row := range rows {
		at, err := time.Parse(timeLayout, row.Time)
		if err != nil {
			return nil, fmt.Errorf("transfer %s of account %d: parse time %q: %w", row.XtbID, row.AccountID, row.Time, err)
		}
		transfers = append(transfers, Transfer{AccountID: row.AccountID, XTBID: row.XtbID, Time: at.UTC(), Amount: row.Amount})
	}
	return transfers, nil
}
