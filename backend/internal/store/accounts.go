package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Accounts returns all Accounts, ordered by name and then by ID.
func (s *Store) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := queries.New(s.db).Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	accounts := make([]Account, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, toAccount(row))
	}
	return accounts, nil
}

// Account returns the Account with the ID, or ErrNotFound.
func (s *Store) Account(ctx context.Context, id int64) (Account, error) {
	row, err := queries.New(s.db).AccountByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("account %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account %d: %w", id, err)
	}
	return toAccount(row), nil
}

// RenameAccount sets the name of the Account with the ID and returns the Account, or ErrNotFound.
func (s *Store) RenameAccount(ctx context.Context, id int64, name string) (Account, error) {
	row, err := queries.New(s.db).RenameAccount(ctx, queries.RenameAccountParams{Name: name, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("account %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Account{}, fmt.Errorf("rename account %d: %w", id, err)
	}
	return toAccount(row), nil
}

// Ledger returns the Cash Operations and the Lots of the Account, the input of ledger.Calculate.
// An Account with no Cash Operations, or an unknown Account, gives an empty Ledger.
func (s *Store) Ledger(ctx context.Context, accountID int64) (ledger.Ledger, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("begin ledger read of account %d: %w", accountID, err)
	}
	// The transaction only reads. A rollback ends it.
	defer func() { _ = tx.Rollback() }()
	return readLedger(ctx, queries.New(tx), accountID)
}

// readLedger reads the Cash Operations and the Lots of the Account with q.
func readLedger(ctx context.Context, q *queries.Queries, accountID int64) (ledger.Ledger, error) {
	opRows, err := q.LedgerCashOperations(ctx, accountID)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("read cash operations of account %d: %w", accountID, err)
	}
	fillRows, err := q.LedgerFills(ctx, accountID)
	if err != nil {
		return ledger.Ledger{}, fmt.Errorf("read lots of account %d: %w", accountID, err)
	}

	l := ledger.Ledger{CashOperations: make([]ledger.CashOperation, 0, len(opRows))}
	for _, row := range opRows {
		at, err := time.Parse(timeLayout, row.Time)
		if err != nil {
			return ledger.Ledger{}, fmt.Errorf("cash operation %s of account %d: parse time %q: %w", row.XtbID, accountID, row.Time, err)
		}
		l.CashOperations = append(l.CashOperations, ledger.CashOperation{
			XTBID:      row.XtbID,
			Type:       row.Type,
			Time:       at.UTC(),
			Amount:     row.Amount,
			PositionID: row.PositionID.String,
			// A missing volume gives zero. The Ledger calculator uses the volume only for a sale.
			Volume: row.Volume.Decimal,
		})
	}
	// The rows are ordered by position ID, so the Fills of one Lot are next to each other.
	for _, row := range fillRows {
		if !row.Volume.Valid {
			return ledger.Ledger{}, fmt.Errorf("lot %s of account %d: a fill has no volume", row.PositionID, accountID)
		}
		if n := len(l.Lots); n == 0 || l.Lots[n-1].PositionID != row.PositionID {
			l.Lots = append(l.Lots, ledger.Lot{
				PositionID: row.PositionID,
				Instrument: ledger.Instrument{Ticker: row.Ticker, Name: row.InstrumentName},
			})
		}
		lot := &l.Lots[len(l.Lots)-1]
		lot.Fills = append(lot.Fills, ledger.Fill{XTBID: row.XtbID, Volume: row.Volume.Decimal, Amount: row.Amount})
	}
	return l, nil
}

func toAccount(row queries.Account) Account {
	return Account{ID: row.ID, AccountNumber: row.AccountNumber, Name: row.Name, Type: row.Type, Currency: row.Currency}
}
