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

// Errors of the Import methods.
var (
	// ErrNotFound tells that the requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrImportNotPending tells that the Import is not in the state 'preview'.
	ErrImportNotPending = errors.New("import is not in preview")
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

// PendingImport is an Import in preview, with the file that the user uploaded.
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

// SaveImport is the data that SaveImport writes in one transaction.
type SaveImport struct {
	ImportID int64
	// Account is the Account of the Import. If its ID is 0, SaveImport uses the Account with the same
	// account number. If no Account has that number, SaveImport makes a new Account.
	Account    Account
	ImportedAt time.Time
	// CashOperations are all Cash Operations of the file.
	// SaveImport skips each operation whose XTB ID already exists on the Account.
	CashOperations []CashOperation
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

// CashOperationIDs returns the XTB IDs of the Cash Operations of the Account.
func (s *Store) CashOperationIDs(ctx context.Context, accountID int64) (map[string]bool, error) {
	ids, err := queries.New(s.db).CashOperationIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("read cash operation IDs of account %d: %w", accountID, err)
	}
	known := make(map[string]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	return known, nil
}

// CreatePreview saves a new Import in the state 'preview' and returns its ID.
// The Import holds the file. It is not in the list of saved Imports.
func (s *Store) CreatePreview(ctx context.Context, p NewPreview) (int64, error) {
	id, err := queries.New(s.db).CreatePreview(ctx, queries.CreatePreviewParams{
		AccountNumber:         p.AccountNumber,
		FileName:              p.FileName,
		File:                  p.File,
		CreatedAt:             p.CreatedAt.UTC().Format(timeLayout),
		NewCashOperations:     int64(p.NewCashOperations),
		SkippedCashOperations: int64(p.SkippedCashOperations),
	})
	if err != nil {
		return 0, fmt.Errorf("save import preview: %w", err)
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
// It returns ErrImportNotPending if the Import is not in preview.
func (s *Store) SaveImport(ctx context.Context, p SaveImport) (Import, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Import{}, fmt.Errorf("begin import %d: %w", p.ImportID, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	accountID, err := saveAccount(ctx, q, p.Account)
	if err != nil {
		return Import{}, err
	}

	w := ledgerWriter{q: q, accountID: accountID, importID: p.ImportID, instruments: map[string]int64{}}
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

	n, err := q.MarkImportSaved(ctx, queries.MarkImportSavedParams{
		AccountID:             sql.NullInt64{Int64: accountID, Valid: true},
		ImportedAt:            sql.NullString{String: p.ImportedAt.UTC().Format(timeLayout), Valid: true},
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
}

// save writes one Cash Operation and its Lot. It returns false if the XTB ID already exists on the Account.
func (w *ledgerWriter) save(ctx context.Context, op CashOperation) (bool, error) {
	var instrumentID sql.NullInt64
	if op.Ticker != "" {
		id, err := w.instrument(ctx, op)
		if err != nil {
			return false, err
		}
		instrumentID = sql.NullInt64{Int64: id, Valid: true}
	}

	opID, err := w.q.InsertCashOperation(ctx, queries.InsertCashOperationParams{
		AccountID:    w.accountID,
		XtbID:        op.XTBID,
		Type:         op.Type,
		InstrumentID: instrumentID,
		Time:         op.Time.UTC().Format(timeLayout),
		Amount:       op.Amount,
		Comment:      op.Comment,
		PositionID:   sql.NullString{String: op.PositionID, Valid: op.PositionID != ""},
		Volume:       op.Volume,
		Price:        op.Price,
		ImportID:     w.importID,
	})
	// ON CONFLICT DO NOTHING returns no row for a known XTB ID.
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("save cash operation %s: %w", op.XTBID, err)
	}
	if !op.IsFill {
		return true, nil
	}

	lotID, err := w.q.UpsertLot(ctx, queries.UpsertLotParams{AccountID: w.accountID, PositionID: op.PositionID, InstrumentID: instrumentID.Int64})
	if err != nil {
		return false, fmt.Errorf("save lot %s: %w", op.PositionID, err)
	}
	if err := w.q.InsertLotFill(ctx, queries.InsertLotFillParams{LotID: lotID, CashOperationID: opID}); err != nil {
		return false, fmt.Errorf("save fill %s of lot %s: %w", op.XTBID, op.PositionID, err)
	}
	return true, nil
}

// instrument returns the ID of the Instrument of op. It makes the Instrument if it does not exist.
func (w *ledgerWriter) instrument(ctx context.Context, op CashOperation) (int64, error) {
	if id, ok := w.instruments[op.Ticker]; ok {
		return id, nil
	}
	id, err := w.q.UpsertInstrument(ctx, queries.UpsertInstrumentParams{Ticker: op.Ticker, Name: op.InstrumentName, Category: op.Category})
	if err != nil {
		return 0, fmt.Errorf("save instrument %s: %w", op.Ticker, err)
	}
	w.instruments[op.Ticker] = id
	return id, nil
}
