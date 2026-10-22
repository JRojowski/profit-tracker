package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Errors of the Import Conflict methods.
var (
	// ErrLedgerChanged tells that the Ledger is not the Ledger that the preview saw.
	ErrLedgerChanged = errors.New("the ledger changed after the preview")
	// ErrConflictNotFound tells that the Import has no Import Conflict with the ID.
	ErrConflictNotFound = errors.New("import conflict not found")
	// ErrOpenConflicts tells that an Import Conflict of the Import has no decision.
	ErrOpenConflicts = errors.New("an import conflict has no decision")
	// ErrMixedLot tells that the Fills of one Lot have more than one Instrument.
	ErrMixedLot = errors.New("a lot has fills of more than one instrument")
)

// The decisions for an Import Conflict.
const (
	// DecisionKeepExisting keeps the stored Cash Operation.
	DecisionKeepExisting = "keep-existing"
	// DecisionUseNew replaces the stored Cash Operation with the Cash Operation of the file.
	DecisionUseNew = "use-new"
)

// OpenConflictsError tells how many Import Conflicts have no decision. It wraps ErrOpenConflicts.
type OpenConflictsError struct {
	ImportID int64
	Count    int
}

func (e *OpenConflictsError) Error() string {
	if e.Count == 1 {
		return fmt.Sprintf("import %d: 1 import conflict has no decision", e.ImportID)
	}
	return fmt.Sprintf("import %d: %d import conflicts have no decision", e.ImportID, e.Count)
}

// Unwrap returns ErrOpenConflicts.
func (e *OpenConflictsError) Unwrap() error { return ErrOpenConflicts }

// CashOperationData is the data of a Cash Operation that a re-import compares.
type CashOperationData struct {
	Type    string
	Time    time.Time
	Amount  decimal.Decimal
	Comment string
}

// Equal tells if d and o have the same type, time, amount and Comment.
// It compares the amounts as numbers, so "1000" and "1000.00" are equal.
func (d CashOperationData) Equal(o CashOperationData) bool {
	return d.Type == o.Type && d.Time.Equal(o.Time) && d.Amount.Equal(o.Amount) && d.Comment == o.Comment
}

// ImportConflict is one Import Conflict of an Import.
type ImportConflict struct {
	ID int64
	// Kind is "changed-data" or "check-mismatch".
	Kind     string
	XTBID    string
	Existing CashOperationData
	New      CashOperationData
	// Decision is "", "keep-existing" or "use-new". "" tells that the user did not decide.
	Decision string
}

// NewImportConflict is an Import Conflict that CreatePreview saves.
type NewImportConflict struct {
	Kind     string
	XTBID    string
	Existing CashOperationData
	New      CashOperationData
}

// KnownCashOperations returns the data of the Cash Operations of the Account, by XTB ID.
func (s *Store) KnownCashOperations(ctx context.Context, accountID int64) (map[string]CashOperationData, error) {
	rows, err := queries.New(s.db).KnownCashOperations(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("read cash operations of account %d: %w", accountID, err)
	}
	known := make(map[string]CashOperationData, len(rows))
	for _, row := range rows {
		data, err := toCashOperationData(row.Type, row.Time, row.Amount, row.Comment)
		if err != nil {
			return nil, fmt.Errorf("cash operation %s of account %d: %w", row.XtbID, accountID, err)
		}
		known[row.XtbID] = data
	}
	return known, nil
}

// ImportConflicts returns the Import Conflicts of the Import, in the order of their IDs.
func (s *Store) ImportConflicts(ctx context.Context, importID int64) ([]ImportConflict, error) {
	return importConflicts(ctx, queries.New(s.db), importID)
}

// DecideImportConflict records the decision for the Import Conflict and returns the conflict.
// It returns ErrNotFound if the Import does not exist, and ErrConflictNotFound if the conflict is not a conflict of the Import.
// It returns an *ImportClosedError if the Import is saved or cancelled.
func (s *Store) DecideImportConflict(ctx context.Context, importID, conflictID int64, decision string) (ImportConflict, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportConflict{}, fmt.Errorf("begin decision for import %d: %w", importID, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	if err := requireOpen(ctx, q, importID); err != nil {
		return ImportConflict{}, err
	}
	row, err := q.DecideImportConflict(ctx, queries.DecideImportConflictParams{
		Decision: sql.NullString{String: decision, Valid: true},
		ID:       conflictID,
		ImportID: importID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ImportConflict{}, fmt.Errorf("import conflict %d of import %d: %w", conflictID, importID, ErrConflictNotFound)
	}
	if err != nil {
		return ImportConflict{}, fmt.Errorf("record decision for import conflict %d: %w", conflictID, err)
	}
	conflict, err := toImportConflict(row)
	if err != nil {
		return ImportConflict{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportConflict{}, fmt.Errorf("commit decision for import conflict %d: %w", conflictID, err)
	}
	return conflict, nil
}

// CancelImport sets the state of the Import to 'cancelled' and drops its file.
// It returns ErrNotFound if the Import does not exist, and an *ImportClosedError if it is saved or cancelled.
func (s *Store) CancelImport(ctx context.Context, importID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cancel of import %d: %w", importID, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	if err := requireOpen(ctx, q, importID); err != nil {
		return err
	}
	if _, err := q.CancelImport(ctx, importID); err != nil {
		return fmt.Errorf("cancel import %d: %w", importID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cancel of import %d: %w", importID, err)
	}
	return nil
}

// ImportClosedError tells that the Import is saved or cancelled. It wraps ErrImportNotPending.
type ImportClosedError struct {
	ImportID int64
	// State is "saved" or "cancelled".
	State string
}

func (e *ImportClosedError) Error() string {
	return fmt.Sprintf("import %d is already %s", e.ImportID, e.State)
}

// Unwrap returns ErrImportNotPending.
func (e *ImportClosedError) Unwrap() error { return ErrImportNotPending }

// requireOpen returns nil if the Import is in preview or in needs-review.
func requireOpen(ctx context.Context, q *queries.Queries, importID int64) error {
	state, err := q.ImportState(ctx, importID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("import %d: %w", importID, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("read import %d: %w", importID, err)
	}
	if !IsOpen(state) {
		return &ImportClosedError{ImportID: importID, State: state}
	}
	return nil
}

// IsOpen tells if an Import in the state waits for the confirmation: the state is 'preview' or 'needs-review'.
func IsOpen(state string) bool {
	return state == StatePreview || state == StateNeedsReview
}

// resolutions reads the decisions for the Import Conflicts of the Import, by XTB ID.
// It returns an *OpenConflictsError if a conflict has no decision.
func resolutions(ctx context.Context, q *queries.Queries, importID int64) (map[string]resolution, error) {
	conflicts, err := importConflicts(ctx, q, importID)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]resolution, len(conflicts))
	open := 0
	for _, c := range conflicts {
		if c.Decision == "" {
			open++
			continue
		}
		resolved[c.XTBID] = resolution{existing: c.Existing, useNew: c.Decision == DecisionUseNew}
	}
	if open > 0 {
		return nil, &OpenConflictsError{ImportID: importID, Count: open}
	}
	return resolved, nil
}

func importConflicts(ctx context.Context, q *queries.Queries, importID int64) ([]ImportConflict, error) {
	rows, err := q.ImportConflicts(ctx, importID)
	if err != nil {
		return nil, fmt.Errorf("read import conflicts of import %d: %w", importID, err)
	}
	conflicts := make([]ImportConflict, 0, len(rows))
	for _, row := range rows {
		conflict, err := toImportConflict(row)
		if err != nil {
			return nil, err
		}
		conflicts = append(conflicts, conflict)
	}
	return conflicts, nil
}

func insertImportConflict(ctx context.Context, q *queries.Queries, importID int64, c NewImportConflict) error {
	err := q.InsertImportConflict(ctx, queries.InsertImportConflictParams{
		ImportID:        importID,
		Kind:            c.Kind,
		XtbID:           c.XTBID,
		ExistingType:    c.Existing.Type,
		ExistingTime:    formatTime(c.Existing.Time),
		ExistingAmount:  c.Existing.Amount,
		ExistingComment: c.Existing.Comment,
		NewType:         c.New.Type,
		NewTime:         formatTime(c.New.Time),
		NewAmount:       c.New.Amount,
		NewComment:      c.New.Comment,
	})
	if err != nil {
		return fmt.Errorf("save import conflict %s of import %d: %w", c.XTBID, importID, err)
	}
	return nil
}

func toImportConflict(row queries.ImportConflict) (ImportConflict, error) {
	existing, err := toCashOperationData(row.ExistingType, row.ExistingTime, row.ExistingAmount, row.ExistingComment)
	if err != nil {
		return ImportConflict{}, fmt.Errorf("import conflict %d: %w", row.ID, err)
	}
	newData, err := toCashOperationData(row.NewType, row.NewTime, row.NewAmount, row.NewComment)
	if err != nil {
		return ImportConflict{}, fmt.Errorf("import conflict %d: %w", row.ID, err)
	}
	return ImportConflict{
		ID:       row.ID,
		Kind:     row.Kind,
		XTBID:    row.XtbID,
		Existing: existing,
		New:      newData,
		Decision: row.Decision.String,
	}, nil
}

func toCashOperationData(typ, at string, amount decimal.Decimal, comment string) (CashOperationData, error) {
	t, err := time.Parse(timeLayout, at)
	if err != nil {
		return CashOperationData{}, fmt.Errorf("parse time %q: %w", at, err)
	}
	return CashOperationData{Type: typ, Time: t.UTC(), Amount: amount, Comment: comment}, nil
}

// formatTime returns the text form of t in the database.
func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}
