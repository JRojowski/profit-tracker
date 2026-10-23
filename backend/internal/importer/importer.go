// Package importer makes an Import preview from an XTB export and saves the Import when the user confirms it.
package importer

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/lot"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// Errors of the Service.
var (
	// ErrRejected tells that the file cannot be imported. The error text tells the user why.
	ErrRejected = errors.New("the import is rejected")
	// ErrTotalMismatch tells that the sum of the Cash Operation amounts is not equal to the Total row.
	ErrTotalMismatch = errors.New("the sum of the cash operation amounts is not equal to the Total row")
	// ErrInvalidAccount tells that the data of a new Account is not valid.
	ErrInvalidAccount = errors.New("invalid account")
	// ErrImportNotFound tells that no Import has the ID.
	ErrImportNotFound = errors.New("import not found")
	// ErrImportClosed tells that the Import is already saved or cancelled.
	ErrImportClosed = errors.New("the import is closed")
	// ErrConflictNotFound tells that the Import has no Import Conflict with the ID.
	ErrConflictNotFound = errors.New("import conflict not found")
	// ErrInvalidDecision tells that the decision is not a known decision,
	// or that it is not valid for the kind of the Import Conflict.
	ErrInvalidDecision = store.ErrInvalidDecision
	// ErrOpenConflicts tells that an Import Conflict of the Import has no decision.
	// The error text tells how many conflicts have no decision.
	ErrOpenConflicts = store.ErrOpenConflicts
	// ErrLedgerChanged tells that a different Import changed the Ledger after the preview.
	// The conflicts and the decisions of the preview are not valid any more.
	ErrLedgerChanged = store.ErrLedgerChanged
)

// The kinds of an Import Conflict.
const (
	// KindChangedData is a known XTB ID with different data.
	KindChangedData = store.KindChangedData
	// KindCheckMismatch is a new sale whose Realized Profit does not agree with Closed Positions.
	KindCheckMismatch = store.KindCheckMismatch
)

// The decisions for an Import Conflict.
const (
	// DecisionKeepExisting keeps the stored Cash Operation. It is valid for the kind "changed-data".
	DecisionKeepExisting = store.DecisionKeepExisting
	// DecisionUseNew replaces the stored Cash Operation with the Cash Operation of the file.
	// It is valid for the kind "changed-data".
	DecisionUseNew = store.DecisionUseNew
	// DecisionAccept imports the sale with the difference noted. It is valid for the kind "check-mismatch".
	DecisionAccept = store.DecisionAccept
)

// RejectedError is an error in the file that stops the Import. Its text is for the user.
type RejectedError struct {
	Err error
}

func (e *RejectedError) Error() string { return e.Err.Error() }

// Unwrap returns the cause and ErrRejected.
func (e *RejectedError) Unwrap() []error { return []error{e.Err, ErrRejected} }

// Account types.
const (
	TypeRegular = "regular"
	TypeIKE     = "IKE"
	TypeIKZE    = "IKZE"
)

// totalTolerance is half a cent. The Total row check accepts a difference that is smaller than totalTolerance.
// Excel stores the Total row as a float sum, so the Total can have float noise, for example 983.029999999997.
// A difference of one cent is a real difference and the check rejects it.
var totalTolerance = decimal.New(5, -3)

// currencyCode matches a currency code, for example "USD".
var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

// Service makes Import previews and saves confirmed Imports.
type Service struct {
	store *store.Store
	now   func() time.Time
}

// NewService returns a Service that keeps the Imports in s.
func NewService(s *store.Store) *Service {
	return &Service{store: s, now: time.Now}
}

// Preview is the result of an upload, before the user confirms it.
type Preview struct {
	ID            int64
	FileName      string
	AccountNumber string
	// Account is the known Account of the account number. It is nil for a new account number.
	Account *store.Account
	// SuggestedCurrency is the currency prefix of the file name. It is "" if there is no prefix.
	SuggestedCurrency string
	// NewCashOperations and SkippedCashOperations do not count the Import Conflicts.
	NewCashOperations     int
	SkippedCashOperations int
	// State is "preview", or "needs-review" if there are Import Conflicts.
	State     string
	Conflicts []store.ImportConflict
	// Warnings are the notes that do not stop the Import. It is empty, not nil, if there are no warnings.
	Warnings []Warning
}

// NewAccount is the data that the user gives for a new Account.
type NewAccount struct {
	Name     string
	Type     string
	Currency string
}

// Preview reads the file and saves an Import in preview. Nothing goes into the Ledger.
// It runs the Realized Profit check of ADR 0001 on the Ledger as it is after the Import.
// It looks for the other side of each Transfer of the file by ADR 0003. The tool does not store the warnings.
// An error in the file is a *RejectedError.
func (s *Service) Preview(ctx context.Context, fileName string, data []byte) (Preview, error) {
	export, err := read(data)
	if err != nil {
		return Preview{}, err
	}

	account, err := s.account(ctx, export.AccountNumber)
	if err != nil {
		return Preview{}, err
	}
	known := map[string]store.CashOperationData{}
	var stored ledger.Ledger
	if account != nil {
		known, err = s.store.KnownCashOperations(ctx, account.ID)
		if err != nil {
			return Preview{}, err
		}
		stored, err = s.store.Ledger(ctx, account.ID)
		if err != nil {
			return Preview{}, err
		}
	}
	ops, err := cashOperations(export)
	if err != nil {
		return Preview{}, err
	}
	c := compare(ops, known)

	newOps := map[string]store.CashOperation{}
	// skippedFills are the Lot IDs of the known Fills that the file has with the same data. Confirm moves
	// each of them to this Lot. ADR 0002: the Lot can be different from the stored Lot.
	skippedFills := map[string]string{}
	for _, op := range ops {
		existing, ok := known[op.XTBID]
		switch {
		case !ok:
			newOps[op.XTBID] = op
		case op.IsFill && existing.Equal(op.Data()):
			skippedFills[op.XTBID] = op.LotID
		}
	}
	after, err := ledger.Calculate(withNewOperations(withLots(stored, skippedFills), ops, newOps))
	if err != nil {
		return Preview{}, &RejectedError{Err: fmt.Errorf("%w: %w", errLedgerNotValid, err)}
	}
	c.conflicts = append(c.conflicts, checkRealizedProfit(after.Sales, export.ClosedPositions, newOps)...)
	warnings, err := s.transferWarnings(ctx, account, export.CashOperations)
	if err != nil {
		return Preview{}, err
	}

	id, err := s.store.CreatePreview(ctx, store.NewPreview{
		AccountNumber:         export.AccountNumber,
		FileName:              fileName,
		File:                  data,
		CreatedAt:             s.now(),
		NewCashOperations:     c.newCount,
		SkippedCashOperations: c.skipped,
		Conflicts:             c.conflicts,
	})
	if err != nil {
		return Preview{}, err
	}
	conflicts, err := s.store.ImportConflicts(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	state := store.StatePreview
	if len(conflicts) > 0 {
		state = store.StateNeedsReview
	}
	return Preview{
		ID:                    id,
		FileName:              fileName,
		AccountNumber:         export.AccountNumber,
		Account:               account,
		SuggestedCurrency:     xtb.CurrencyFromFileName(fileName),
		NewCashOperations:     c.newCount,
		SkippedCashOperations: c.skipped,
		State:                 state,
		Conflicts:             conflicts,
		Warnings:              warnings,
	}, nil
}

// comparison is the result of a comparison of the Cash Operations of a file with the stored ones.
type comparison struct {
	newCount  int
	skipped   int
	conflicts []store.NewImportConflict
}

// compare compares the Cash Operations of a file with the stored Cash Operations, by XTB ID.
// A known XTB ID with the same data is skipped. A known XTB ID with different data is an Import Conflict.
func compare(ops []store.CashOperation, known map[string]store.CashOperationData) comparison {
	var c comparison
	for _, op := range ops {
		existing, ok := known[op.XTBID]
		switch {
		case !ok:
			c.newCount++
		case existing.Equal(op.Data()):
			c.skipped++
		default:
			c.conflicts = append(c.conflicts, store.NewImportConflict{
				Kind:     KindChangedData,
				XTBID:    op.XTBID,
				Existing: &existing,
				New:      op.Data(),
			})
		}
	}
	return c
}

// Confirm saves the Import with the ID: the Account (if new), the Instruments,
// the Cash Operations and the Lots, in one transaction.
// A known account number ignores newAccount. A new account number needs a valid newAccount.
// Each Import Conflict must have a decision. If a different Import changed the Ledger after the preview,
// Confirm returns ErrLedgerChanged and saves nothing.
func (s *Service) Confirm(ctx context.Context, id int64, newAccount NewAccount) (store.Import, error) {
	pending, err := s.store.PendingImport(ctx, id)
	if err != nil {
		return store.Import{}, openImportError(id, err)
	}
	// A saved or cancelled Import has no file. SaveImport checks the state again in its transaction.
	if !store.IsOpen(pending.State) {
		return store.Import{}, openImportError(id, &store.ImportClosedError{ImportID: id, State: pending.State})
	}

	export, err := read(pending.File)
	if err != nil {
		return store.Import{}, err
	}
	account, err := s.account(ctx, export.AccountNumber)
	if err != nil {
		return store.Import{}, err
	}
	if account == nil {
		if err := newAccount.validate(); err != nil {
			return store.Import{}, err
		}
		account = &store.Account{
			AccountNumber: export.AccountNumber,
			Name:          strings.TrimSpace(newAccount.Name),
			Type:          newAccount.Type,
			Currency:      newAccount.Currency,
		}
	}

	ops, err := cashOperations(export)
	if err != nil {
		return store.Import{}, err
	}
	saved, err := s.store.SaveImport(ctx, store.SaveImport{
		ImportID:       id,
		Account:        *account,
		ImportedAt:     s.now(),
		CashOperations: ops,
		CheckLedger:    checkLedger,
	})
	var invalid *invalidLedgerError
	switch {
	case err == nil:
		return saved, nil
	case errors.As(err, &invalid):
		// The decisions give a Ledger that the calculator cannot use. Nothing is saved, and the Import stays open.
		return store.Import{}, &RejectedError{Err: err}
	case errors.Is(err, store.ErrLedgerChanged):
		return store.Import{}, fmt.Errorf("import %d: upload the file again, because a different import changed the ledger after the preview: %w", id, err)
	case errors.Is(err, store.ErrMixedLot):
		return store.Import{}, &RejectedError{Err: fmt.Errorf("import %d: %w", id, err)}
	case errors.Is(err, store.ErrOpenConflicts):
		return store.Import{}, err
	}
	return store.Import{}, openImportError(id, err)
}

// Decide records the decision for the Import Conflict of the Import, and returns the conflict.
// The decision must be valid for the kind of the conflict: "keep-existing" or "use-new" for "changed-data",
// and "accept" for "check-mismatch". The store checks this rule.
// The user can change a decision until the Import is saved or cancelled.
func (s *Service) Decide(ctx context.Context, importID, conflictID int64, decision string) (store.ImportConflict, error) {
	conflict, err := s.store.DecideImportConflict(ctx, importID, conflictID, decision)
	if errors.Is(err, store.ErrConflictNotFound) {
		return store.ImportConflict{}, fmt.Errorf("import conflict %d of import %d: %w", conflictID, importID, ErrConflictNotFound)
	}
	if err != nil {
		return store.ImportConflict{}, openImportError(importID, err)
	}
	return conflict, nil
}

// Cancel cancels the Import. Nothing goes into the Ledger. A cancelled Import cannot be confirmed.
func (s *Service) Cancel(ctx context.Context, id int64) error {
	if err := s.store.CancelImport(ctx, id); err != nil {
		return openImportError(id, err)
	}
	return nil
}

// openImportError maps an error of the store for an Import that must be open to an error of the Service.
func openImportError(id int64, err error) error {
	var closed *store.ImportClosedError
	switch {
	case errors.As(err, &closed):
		return fmt.Errorf("import %d is already %s: %w", id, closed.State, ErrImportClosed)
	case errors.Is(err, store.ErrImportNotPending):
		return fmt.Errorf("import %d: %w", id, ErrImportClosed)
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("import %d: %w", id, ErrImportNotFound)
	}
	return err
}

// Imports returns the saved Imports, the newest first.
func (s *Service) Imports(ctx context.Context) ([]store.Import, error) {
	return s.store.Imports(ctx)
}

// account returns the Account of the account number, or nil if the number is new.
func (s *Service) account(ctx context.Context, number string) (*store.Account, error) {
	account, err := s.store.AccountByNumber(ctx, number)
	if errors.Is(err, store.ErrNotFound) {
		// A nil Account tells that the account number is new.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// read reads the export and runs the checks of the file.
func read(data []byte) (xtb.Export, error) {
	export, err := xtb.Read(data)
	if err != nil {
		return xtb.Export{}, &RejectedError{Err: err}
	}

	sum := decimal.Zero
	for _, op := range export.CashOperations {
		sum = sum.Add(op.Amount)
	}
	// The Total row check: a difference means that the reader lost rows.
	if sum.Sub(export.Total).Abs().GreaterThanOrEqual(totalTolerance) {
		return xtb.Export{}, &RejectedError{Err: fmt.Errorf("%w: the sum is %s, the Total row is %s", ErrTotalMismatch, sum, export.Total)}
	}
	return export, nil
}

// cashOperations converts the Cash Operations of the export for the store.
// It builds the Lots of the Fills with the rule of ADR 0002. An order that does not fit the rule is a *RejectedError.
func cashOperations(export xtb.Export) ([]store.CashOperation, error) {
	lots, err := lot.Build(export.CashOperations, export.ClosedPositions, export.OpenLots)
	if err != nil {
		return nil, &RejectedError{Err: err}
	}
	ops := make([]store.CashOperation, 0, len(export.CashOperations))
	for _, op := range export.CashOperations {
		saved := store.CashOperation{
			XTBID:          op.ID,
			Type:           string(op.Type),
			Ticker:         op.Ticker,
			InstrumentName: op.Instrument,
			Category:       op.Category,
			Time:           op.Time,
			Amount:         op.Amount,
			Comment:        op.Comment,
			PositionID:     op.PositionID,
		}
		switch {
		case op.Purchase != nil:
			// ADR 0002: the Lot of a Fill can have the position ID of a different Fill of the same order.
			saved.LotID = lots[op.ID]
			saved.Volume = decimal.NewNullDecimal(op.Purchase.Volume)
			saved.Price = decimal.NewNullDecimal(op.Purchase.Price)
			saved.IsFill = true
		case op.Sale != nil:
			// ADR 0001: the volume of a sale is the closed volume of its Lot.
			saved.Volume = decimal.NewNullDecimal(op.Sale.Volume)
			saved.Price = decimal.NewNullDecimal(op.Sale.Price)
		}
		ops = append(ops, saved)
	}
	return ops, nil
}

// validate checks the data of a new Account.
func (a NewAccount) validate() error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("%w: the name is empty", ErrInvalidAccount)
	}
	switch a.Type {
	case TypeRegular, TypeIKE, TypeIKZE:
	default:
		return fmt.Errorf("%w: the type %q is not %q, %q or %q", ErrInvalidAccount, a.Type, TypeRegular, TypeIKE, TypeIKZE)
	}
	if !currencyCode.MatchString(a.Currency) {
		return fmt.Errorf("%w: the currency %q is not a code of 3 capital letters, for example USD", ErrInvalidAccount, a.Currency)
	}
	return nil
}
