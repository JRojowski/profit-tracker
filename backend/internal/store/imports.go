package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
)

// Errors of the Import methods.
var (
	// ErrNotFound tells that the requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrImportNotPending tells that the Import is not in the state 'preview' or 'needs-review'.
	ErrImportNotPending = errors.New("import is not in preview")
)

// The states of an Import.
const (
	// StatePreview is an Import that waits for the confirmation.
	StatePreview = "preview"
	// StateNeedsReview is an Import with Import Conflicts. It waits for the decisions and the confirmation.
	StateNeedsReview = "needs-review"
	// StateSaved is an Import that is in the Ledger.
	StateSaved = "saved"
	// StateCancelled is an Import that the user cancelled.
	StateCancelled = "cancelled"
)

// Account is one broker Account.
type Account struct {
	ID            int64
	AccountNumber string
	Name          string
	// Type is "regular", "IKE" or "IKZE".
	Type     string
	Currency string
}

// PendingImport is an Import with the file that the user uploaded. The file is nil after the save or the cancel.
type PendingImport struct {
	ID            int64
	AccountNumber string
	FileName      string
	File          []byte
	State         string
}

// Import is one saved Import.
type Import struct {
	ID                    int64
	FileName              string
	ImportedAt            time.Time
	AccountID             int64
	AccountNumber         string
	AccountName           string
	NewCashOperations     int
	SkippedCashOperations int
}

// NewPreview is the data of a new Import in preview.
type NewPreview struct {
	AccountNumber         string
	FileName              string
	File                  []byte
	CreatedAt             time.Time
	NewCashOperations     int
	SkippedCashOperations int
	// Conflicts are the Import Conflicts. An Import with conflicts gets the state 'needs-review'.
	Conflicts []NewImportConflict
}

// CashOperation is one Cash Operation to save.
type CashOperation struct {
	// XTBID is the XTB ID of the operation. It is unique on the Account.
	XTBID string
	Type  string
	// Ticker, InstrumentName and Category identify the Instrument. Ticker is empty for an operation with no Instrument.
	Ticker         string
	InstrumentName string
	Category       string
	Time           time.Time
	Amount         decimal.Decimal
	Comment        string
	// PositionID is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
	// Volume and Price are the parsed volume and price. They are not valid for an operation with no volume.
	Volume decimal.NullDecimal
	Price  decimal.NullDecimal
	// IsFill tells that the operation is a Fill of the Lot PositionID.
	IsFill bool
}

// Data returns the data of op that a re-import compares.
func (op CashOperation) Data() CashOperationData {
	return CashOperationData{
		Type: op.Type, Time: op.Time, Amount: op.Amount, Comment: op.Comment,
		Ticker: op.Ticker, PositionID: op.PositionID, Volume: op.Volume, Price: op.Price,
	}
}

// resolution is the decision of the user for one Import Conflict of the kind "changed-data".
type resolution struct {
	// existing is the stored data that the user saw in the preview.
	existing CashOperationData
	// useNew tells to replace the stored Cash Operation with the operation of the file.
	// If it is false, the stored Cash Operation stays.
	useNew bool
}

// SaveImport is the data that SaveImport writes in one transaction.
type SaveImport struct {
	ImportID int64
	// Account is the Account of the Import. If its ID is 0, SaveImport uses the Account with the same
	// account number. If no Account has that number, SaveImport makes a new Account.
	Account    Account
	ImportedAt time.Time
	// CashOperations are all Cash Operations of the file.
	// SaveImport skips each operation whose XTB ID already exists on the Account with the same data.
	// For a known XTB ID with different data, the decision of its Import Conflict applies.
	CashOperations []CashOperation
	// CheckLedger checks the Ledger of the Account after all writes, in the same transaction.
	// If it returns an error, SaveImport saves nothing and returns that error. A nil CheckLedger checks nothing.
	CheckLedger func(ledger.Ledger) error
}

// timeLayout is the text form of the times in the database. In UTC, the text sorts in time order.
const timeLayout = time.RFC3339

// AccountByNumber returns the Account with the account number, or ErrNotFound.
func (s *Store) AccountByNumber(ctx context.Context, number string) (Account, error) {
	row, err := queries.New(s.db).AccountByNumber(ctx, number)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("account %s: %w", number, ErrNotFound)
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account %s: %w", number, err)
	}
	return Account{ID: row.ID, AccountNumber: row.AccountNumber, Name: row.Name, Type: row.Type, Currency: row.Currency}, nil
}

// CreatePreview saves a new Import and its Import Conflicts, and returns the ID of the Import.
// The state is 'needs-review' if there are conflicts, and 'preview' if not.
// The Import holds the file. It is not in the list of saved Imports.
func (s *Store) CreatePreview(ctx context.Context, p NewPreview) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import preview: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	state := StatePreview
	if len(p.Conflicts) > 0 {
		state = StateNeedsReview
	}
	id, err := q.CreatePreview(ctx, queries.CreatePreviewParams{
		AccountNumber:         p.AccountNumber,
		FileName:              p.FileName,
		File:                  p.File,
		State:                 state,
		CreatedAt:             formatTime(p.CreatedAt),
		NewCashOperations:     int64(p.NewCashOperations),
		SkippedCashOperations: int64(p.SkippedCashOperations),
	})
	if err != nil {
		return 0, fmt.Errorf("save import preview: %w", err)
	}
	for _, c := range p.Conflicts {
		if err := insertImportConflict(ctx, q, id, c); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import preview %d: %w", id, err)
	}
	return id, nil
}

// PendingImport returns the Import with the ID, or ErrNotFound.
func (s *Store) PendingImport(ctx context.Context, id int64) (PendingImport, error) {
	row, err := queries.New(s.db).PendingImport(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingImport{}, fmt.Errorf("import %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return PendingImport{}, fmt.Errorf("read import %d: %w", id, err)
	}
	return PendingImport{ID: row.ID, AccountNumber: row.AccountNumber, FileName: row.FileName, File: row.File, State: row.State}, nil
}

// SaveImport saves the Account (if new), the Instruments, the Cash Operations and the Lots of an Import
// in one transaction, and returns the saved Import.
// It returns an *ImportClosedError if the Import is not in preview or in needs-review,
// and an *OpenConflictsError if an Import Conflict has no decision.
// It returns ErrLedgerChanged if the Ledger does not agree with the Import Conflicts and their decisions,
// and ErrMixedLot if the Fills of a Lot have more than one Instrument after the save.
// It returns the error of p.CheckLedger if the Ledger after the save fails the check.
func (s *Store) SaveImport(ctx context.Context, p SaveImport) (Import, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Import{}, fmt.Errorf("begin import %d: %w", p.ImportID, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	// The checks are in the transaction, so a decision or a different Import cannot change the result.
	if err := requireOpen(ctx, q, p.ImportID); err != nil {
		return Import{}, err
	}
	resolved, err := resolutions(ctx, q, p.ImportID)
	if err != nil {
		return Import{}, err
	}
	accountID, err := saveAccount(ctx, q, p.Account)
	if err != nil {
		return Import{}, err
	}

	w := ledgerWriter{q: q, accountID: accountID, importID: p.ImportID, instruments: map[string]int64{}, resolutions: resolved}
	saved, skipped := 0, 0
	for _, op := range p.CashOperations {
		isNew, err := w.save(ctx, op)
		if err != nil {
			return Import{}, err
		}
		if isNew {
			saved++
		} else {
			skipped++
		}
	}
	if err := w.fixLots(ctx); err != nil {
		return Import{}, err
	}
	if p.CheckLedger != nil {
		// The decisions of the Import Conflicts can change the Lots and the sales. The check sees the result.
		after, err := readLedger(ctx, q, accountID)
		if err != nil {
			return Import{}, err
		}
		if err := p.CheckLedger(after); err != nil {
			return Import{}, fmt.Errorf("import %d: %w", p.ImportID, err)
		}
	}

	n, err := q.MarkImportSaved(ctx, queries.MarkImportSavedParams{
		AccountID:             sql.NullInt64{Int64: accountID, Valid: true},
		ImportedAt:            sql.NullString{String: formatTime(p.ImportedAt), Valid: true},
		NewCashOperations:     int64(saved),
		SkippedCashOperations: int64(skipped),
		ID:                    p.ImportID,
	})
	if err != nil {
		return Import{}, fmt.Errorf("mark import %d saved: %w", p.ImportID, err)
	}
	if n == 0 {
		return Import{}, fmt.Errorf("import %d: %w", p.ImportID, ErrImportNotPending)
	}

	row, err := q.SavedImport(ctx, p.ImportID)
	if err != nil {
		return Import{}, fmt.Errorf("read saved import %d: %w", p.ImportID, err)
	}
	if err := tx.Commit(); err != nil {
		return Import{}, fmt.Errorf("commit import %d: %w", p.ImportID, err)
	}
	return toImport(queries.SavedImportsRow(row))
}

// saveAccount returns the ID of the Account. If the ID is 0, it finds the Account by its account number,
// or makes a new Account. The lookup is in the transaction, so a different Import that made the Account
// after the preview does not cause a UNIQUE error.
func saveAccount(ctx context.Context, q *queries.Queries, a Account) (int64, error) {
	if a.ID != 0 {
		return a.ID, nil
	}
	row, err := q.AccountByNumber(ctx, a.AccountNumber)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read account %s: %w", a.AccountNumber, err)
	}
	id, err := q.CreateAccount(ctx, queries.CreateAccountParams{
		AccountNumber: a.AccountNumber,
		Name:          a.Name,
		Type:          a.Type,
		Currency:      a.Currency,
	})
	if err != nil {
		return 0, fmt.Errorf("create account %s: %w", a.AccountNumber, err)
	}
	return id, nil
}

// Imports returns the saved Imports, the newest first.
func (s *Store) Imports(ctx context.Context) ([]Import, error) {
	rows, err := queries.New(s.db).SavedImports(ctx)
	if err != nil {
		return nil, fmt.Errorf("read imports: %w", err)
	}
	imports := make([]Import, 0, len(rows))
	for _, row := range rows {
		imp, err := toImport(row)
		if err != nil {
			return nil, err
		}
		imports = append(imports, imp)
	}
	return imports, nil
}

func toImport(row queries.SavedImportsRow) (Import, error) {
	importedAt, err := time.Parse(timeLayout, row.ImportedAt.String)
	if err != nil {
		return Import{}, fmt.Errorf("import %d: parse imported_at %q: %w", row.ID, row.ImportedAt.String, err)
	}
	return Import{
		ID:                    row.ID,
		FileName:              row.FileName,
		ImportedAt:            importedAt.UTC(),
		AccountID:             row.AccountID,
		AccountNumber:         row.AccountNumber,
		AccountName:           row.AccountName,
		NewCashOperations:     int(row.NewCashOperations),
		SkippedCashOperations: int(row.SkippedCashOperations),
	}, nil
}

// ledgerWriter writes the Cash Operations of one Import in a transaction.
type ledgerWriter struct {
	q           *queries.Queries
	accountID   int64
	importID    int64
	instruments map[string]int64 // ticker to Instrument ID
	// resolutions are the decisions for the Import Conflicts, by XTB ID.
	resolutions map[string]resolution
}

// save writes one Cash Operation and its Lot. It returns true if it adds the operation or replaces
// the stored operation, and false if the stored operation stays.
// For a known XTB ID with different data, the decision of its Import Conflict applies.
// It returns ErrLedgerChanged if the stored operation does not agree with the Import Conflicts.
// It writes nothing for an operation that it skips.
func (w *ledgerWriter) save(ctx context.Context, op CashOperation) (bool, error) {
	stored, err := w.q.CashOperationByXTBID(ctx, queries.CashOperationByXTBIDParams{AccountID: w.accountID, XtbID: op.XTBID})
	if errors.Is(err, sql.ErrNoRows) {
		return true, w.insert(ctx, op)
	}
	if err != nil {
		return false, fmt.Errorf("read cash operation %s: %w", op.XTBID, err)
	}
	storedData, err := withTime(stored.Time, CashOperationData{
		Type: stored.Type, Amount: stored.Amount, Comment: stored.Comment,
		Ticker: stored.Ticker, PositionID: stored.PositionID, Volume: stored.Volume, Price: stored.Price,
	})
	if err != nil {
		return false, fmt.Errorf("cash operation %s: %w", op.XTBID, err)
	}

	res, resolved := w.resolutions[op.XTBID]
	if storedData.Equal(op.Data()) {
		if resolved {
			return false, fmt.Errorf("cash operation %s already has the data of the file: %w", op.XTBID, ErrLedgerChanged)
		}
		return false, nil
	}
	if !resolved || !res.existing.Equal(storedData) {
		return false, fmt.Errorf("cash operation %s has data that the preview did not show: %w", op.XTBID, ErrLedgerChanged)
	}
	if !res.useNew {
		return false, nil
	}
	return true, w.replace(ctx, stored.ID, op)
}

// insert adds a new Cash Operation and its Fill.
func (w *ledgerWriter) insert(ctx context.Context, op CashOperation) error {
	instrumentID, err := w.instrument(ctx, op)
	if err != nil {
		return err
	}
	opID, err := w.q.InsertCashOperation(ctx, queries.InsertCashOperationParams{
		AccountID:    w.accountID,
		XtbID:        op.XTBID,
		Type:         op.Type,
		InstrumentID: instrumentID,
		Time:         formatTime(op.Time),
		Amount:       op.Amount,
		Comment:      op.Comment,
		PositionID:   sql.NullString{String: op.PositionID, Valid: op.PositionID != ""},
		Volume:       op.Volume,
		Price:        op.Price,
		ImportID:     w.importID,
	})
	if err != nil {
		return fmt.Errorf("save cash operation %s: %w", op.XTBID, err)
	}
	return w.addFill(ctx, opID, op, instrumentID)
}

// replace writes the data of op into the stored Cash Operation with the ID, and moves its Fill to the Lot of op.
func (w *ledgerWriter) replace(ctx context.Context, opID int64, op CashOperation) error {
	instrumentID, err := w.instrument(ctx, op)
	if err != nil {
		return err
	}
	err = w.q.UpdateCashOperation(ctx, queries.UpdateCashOperationParams{
		Type:         op.Type,
		InstrumentID: instrumentID,
		Time:         formatTime(op.Time),
		Amount:       op.Amount,
		Comment:      op.Comment,
		PositionID:   sql.NullString{String: op.PositionID, Valid: op.PositionID != ""},
		Volume:       op.Volume,
		Price:        op.Price,
		ImportID:     w.importID,
		ID:           opID,
	})
	if err != nil {
		return fmt.Errorf("replace cash operation %s: %w", op.XTBID, err)
	}
	if err := w.q.DeleteLotFill(ctx, opID); err != nil {
		return fmt.Errorf("remove fill %s: %w", op.XTBID, err)
	}
	return w.addFill(ctx, opID, op, instrumentID)
}

// fixLots runs after all writes of the Import. It removes each Lot with no Fills, and gives each Lot
// the Instrument of its Fills. A "use-new" decision can move a Fill to a different Lot or Instrument.
// It returns ErrMixedLot if the Fills of a Lot have more than one Instrument.
func (w *ledgerWriter) fixLots(ctx context.Context) error {
	if err := w.q.DeleteEmptyLots(ctx, w.accountID); err != nil {
		return fmt.Errorf("remove lots with no fills: %w", err)
	}
	mixed, err := w.q.MixedLots(ctx, w.accountID)
	if err != nil {
		return fmt.Errorf("check the instruments of the lots: %w", err)
	}
	if len(mixed) > 0 {
		return fmt.Errorf("lot %s: %w", strings.Join(mixed, ", "), ErrMixedLot)
	}
	if err := w.q.SetLotInstruments(ctx, w.accountID); err != nil {
		return fmt.Errorf("set the instruments of the lots: %w", err)
	}
	return nil
}

// addFill adds the Cash Operation with the ID to its Lot if op is a Fill.
func (w *ledgerWriter) addFill(ctx context.Context, opID int64, op CashOperation, instrumentID sql.NullInt64) error {
	if !op.IsFill {
		return nil
	}
	lotID, err := w.q.UpsertLot(ctx, queries.UpsertLotParams{AccountID: w.accountID, PositionID: op.PositionID, InstrumentID: instrumentID.Int64})
	if err != nil {
		return fmt.Errorf("save lot %s: %w", op.PositionID, err)
	}
	if err := w.q.InsertLotFill(ctx, queries.InsertLotFillParams{LotID: lotID, CashOperationID: opID}); err != nil {
		return fmt.Errorf("save fill %s of lot %s: %w", op.XTBID, op.PositionID, err)
	}
	return nil
}

// instrument returns the ID of the Instrument of op. It makes the Instrument if it does not exist.
// It returns a NULL ID for an operation with no Instrument.
func (w *ledgerWriter) instrument(ctx context.Context, op CashOperation) (sql.NullInt64, error) {
	if op.Ticker == "" {
		return sql.NullInt64{}, nil
	}
	if id, ok := w.instruments[op.Ticker]; ok {
		return sql.NullInt64{Int64: id, Valid: true}, nil
	}
	id, err := w.q.UpsertInstrument(ctx, queries.UpsertInstrumentParams{Ticker: op.Ticker, Name: op.InstrumentName, Category: op.Category})
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("save instrument %s: %w", op.Ticker, err)
	}
	w.instruments[op.Ticker] = id
	return sql.NullInt64{Int64: id, Valid: true}, nil
}
