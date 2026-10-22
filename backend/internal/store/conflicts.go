package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	// ErrNoLotID tells that a Fill to save has no Lot ID. It is a defect of the caller.
	ErrNoLotID = errors.New("a fill has no lot id")
	// ErrInvalidDecision tells that the decision is not valid for the kind of the Import Conflict.
	ErrInvalidDecision = errors.New("invalid decision")
)

// The kinds of an Import Conflict.
const (
	// KindChangedData is a known XTB ID with a different type, time, amount, Comment, ticker,
	// position ID, volume or price.
	KindChangedData = "changed-data"
	// KindCheckMismatch is a new sale whose Realized Profit differs by more than 0.01
	// from the Profit/Loss in the Closed Positions sheet. See ADR 0001.
	KindCheckMismatch = "check-mismatch"
)

// The decisions for an Import Conflict.
const (
	// DecisionKeepExisting keeps the stored Cash Operation. It is valid for the kind "changed-data".
	DecisionKeepExisting = "keep-existing"
	// DecisionUseNew replaces the stored Cash Operation with the Cash Operation of the file.
	// It is valid for the kind "changed-data".
	DecisionUseNew = "use-new"
	// DecisionAccept imports the sale and notes the difference. It is valid for the kind "check-mismatch".
	// The Ledger stays the source of truth. The tool copies nothing from Closed Positions.
	DecisionAccept = "accept"
)

// decisionsByKind are the valid decisions for each kind of Import Conflict.
var decisionsByKind = map[string][]string{
	KindChangedData:   {DecisionKeepExisting, DecisionUseNew},
	KindCheckMismatch: {DecisionAccept},
}

// InvalidDecisionError tells that the decision is not valid for the kind of the Import Conflict.
// It wraps ErrInvalidDecision.
type InvalidDecisionError struct {
	Kind     string
	Decision string
}

func (e *InvalidDecisionError) Error() string {
	valid := decisionsByKind[e.Kind]
	quoted := make([]string, len(valid))
	for i, d := range valid {
		quoted[i] = strconv.Quote(d)
	}
	return fmt.Sprintf("the decision %q is not valid for an import conflict of the kind %q: the valid decisions are %s",
		e.Decision, e.Kind, strings.Join(quoted, " and "))
}

// Unwrap returns ErrInvalidDecision.
func (e *InvalidDecisionError) Unwrap() error { return ErrInvalidDecision }

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
	// Ticker and PositionID are "" for an operation with no Instrument or no Lot.
	Ticker     string
	PositionID string
	// Volume and Price are not valid for an operation with no volume.
	Volume decimal.NullDecimal
	Price  decimal.NullDecimal
}

// Equal tells if d and o have the same type, time, amount, Comment, ticker, position ID, volume and price.
// It compares the amounts, volumes and prices as numbers, so "1000" and "1000.00" are equal.
// A missing volume or price equals only a missing volume or price.
func (d CashOperationData) Equal(o CashOperationData) bool {
	return d.Type == o.Type && d.Time.Equal(o.Time) && d.Amount.Equal(o.Amount) && d.Comment == o.Comment &&
		d.Ticker == o.Ticker && d.PositionID == o.PositionID &&
		nullEqual(d.Volume, o.Volume) && nullEqual(d.Price, o.Price)
}

// nullEqual tells if a and b are both missing, or both valid with the same number.
func nullEqual(a, b decimal.NullDecimal) bool {
	if !a.Valid || !b.Valid {
		return a.Valid == b.Valid
	}
	return a.Decimal.Equal(b.Decimal)
}

// ImportConflict is one Import Conflict of an Import.
type ImportConflict struct {
	ID int64
	// Kind is "changed-data" or "check-mismatch".
	Kind  string
	XTBID string
	// Existing is the stored data. It is nil for the kind "check-mismatch".
	Existing *CashOperationData
	// New is the data of the file. For the kind "check-mismatch", it is the sale.
	New CashOperationData
	// Check is the Realized Profit check. It is nil for the kind "changed-data".
	Check *Check
	// Decision is "", "keep-existing", "use-new" or "accept". "" tells that the user did not decide.
	Decision string
}

// Check is the result of the Realized Profit check of one sale. See ADR 0001.
type Check struct {
	// Calculated is the Realized Profit from the Ledger.
	Calculated decimal.Decimal
	// ClosedPositions is the Profit/Loss of the sale in the Closed Positions sheet.
	ClosedPositions decimal.Decimal
}

// NewImportConflict is an Import Conflict that CreatePreview saves.
type NewImportConflict struct {
	Kind  string
	XTBID string
	// Existing is the stored data. It must be set for the kind "changed-data" and nil for "check-mismatch".
	Existing *CashOperationData
	New      CashOperationData
	// Check must be set for the kind "check-mismatch" and nil for "changed-data".
	Check *Check
}

// KnownCashOperations returns the data of the Cash Operations of the Account, by XTB ID.
func (s *Store) KnownCashOperations(ctx context.Context, accountID int64) (map[string]CashOperationData, error) {
	rows, err := queries.New(s.db).KnownCashOperations(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("read cash operations of account %d: %w", accountID, err)
	}
	known := make(map[string]CashOperationData, len(rows))
	for _, row := range rows {
		data, err := withTime(row.Time, CashOperationData{
			Type: row.Type, Amount: row.Amount, Comment: row.Comment,
			Ticker: row.Ticker, PositionID: row.PositionID, Volume: row.Volume, Price: row.Price,
		})
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
	kind, err := q.ImportConflictKind(ctx, queries.ImportConflictKindParams{ID: conflictID, ImportID: importID})
	if errors.Is(err, sql.ErrNoRows) {
		return ImportConflict{}, fmt.Errorf("import conflict %d of import %d: %w", conflictID, importID, ErrConflictNotFound)
	}
	if err != nil {
		return ImportConflict{}, fmt.Errorf("read import conflict %d: %w", conflictID, err)
	}
	if !slices.Contains(decisionsByKind[kind], decision) {
		return ImportConflict{}, &InvalidDecisionError{Kind: kind, Decision: decision}
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

// resolutions reads the decisions for the Import Conflicts of the kind "changed-data", by XTB ID.
// A conflict of the kind "check-mismatch" changes no write: its sale is new, and the decision "accept" saves it.
// It returns an *OpenConflictsError if a conflict of any kind has no decision.
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
		if c.Kind != KindChangedData || c.Existing == nil {
			continue
		}
		resolved[c.XTBID] = resolution{existing: *c.Existing, useNew: c.Decision == DecisionUseNew}
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
	params := queries.InsertImportConflictParams{
		ImportID:      importID,
		Kind:          c.Kind,
		XtbID:         c.XTBID,
		NewType:       c.New.Type,
		NewTime:       formatTime(c.New.Time),
		NewAmount:     c.New.Amount,
		NewComment:    c.New.Comment,
		NewTicker:     c.New.Ticker,
		NewPositionID: c.New.PositionID,
		NewVolume:     c.New.Volume,
		NewPrice:      c.New.Price,
	}
	if e := c.Existing; e != nil {
		params.ExistingType = validString(e.Type)
		params.ExistingTime = validString(formatTime(e.Time))
		params.ExistingAmount = decimal.NewNullDecimal(e.Amount)
		params.ExistingComment = validString(e.Comment)
		params.ExistingTicker = validString(e.Ticker)
		params.ExistingPositionID = validString(e.PositionID)
		params.ExistingVolume = e.Volume
		params.ExistingPrice = e.Price
	}
	if ch := c.Check; ch != nil {
		params.CheckCalculated = decimal.NewNullDecimal(ch.Calculated)
		params.CheckClosedPositions = decimal.NewNullDecimal(ch.ClosedPositions)
	}
	if err := q.InsertImportConflict(ctx, params); err != nil {
		return fmt.Errorf("save import conflict %s of import %d: %w", c.XTBID, importID, err)
	}
	return nil
}

// validString returns s as a valid sql.NullString, also when s is "".
func validString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

func toImportConflict(row queries.ImportConflict) (ImportConflict, error) {
	newData, err := withTime(row.NewTime, CashOperationData{
		Type: row.NewType, Amount: row.NewAmount, Comment: row.NewComment,
		Ticker: row.NewTicker, PositionID: row.NewPositionID, Volume: row.NewVolume, Price: row.NewPrice,
	})
	if err != nil {
		return ImportConflict{}, fmt.Errorf("import conflict %d: %w", row.ID, err)
	}
	conflict := ImportConflict{
		ID:       row.ID,
		Kind:     row.Kind,
		XTBID:    row.XtbID,
		New:      newData,
		Decision: row.Decision.String,
	}
	// The table CHECK makes all existing_* columns of a "changed-data" conflict NOT NULL.
	if row.ExistingTime.Valid {
		existing, err := withTime(row.ExistingTime.String, CashOperationData{
			Type: row.ExistingType.String, Amount: row.ExistingAmount.Decimal, Comment: row.ExistingComment.String,
			Ticker: row.ExistingTicker.String, PositionID: row.ExistingPositionID.String,
			Volume: row.ExistingVolume, Price: row.ExistingPrice,
		})
		if err != nil {
			return ImportConflict{}, fmt.Errorf("import conflict %d: %w", row.ID, err)
		}
		conflict.Existing = &existing
	}
	if row.CheckCalculated.Valid && row.CheckClosedPositions.Valid {
		conflict.Check = &Check{Calculated: row.CheckCalculated.Decimal, ClosedPositions: row.CheckClosedPositions.Decimal}
	}
	return conflict, nil
}

// withTime returns d with the time from its text form in the database.
func withTime(at string, d CashOperationData) (CashOperationData, error) {
	t, err := time.Parse(timeLayout, at)
	if err != nil {
		return CashOperationData{}, fmt.Errorf("parse time %q: %w", at, err)
	}
	d.Time = t.UTC()
	return d, nil
}

// formatTime returns the text form of t in the database.
func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}
