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
	// ErrAlreadyConfirmed tells that the Import is not in preview any more.
	ErrAlreadyConfirmed = errors.New("import is already confirmed")
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
	SuggestedCurrency     string
	NewCashOperations     int
	SkippedCashOperations int
}

// NewAccount is the data that the user gives for a new Account.
type NewAccount struct {
	Name     string
	Type     string
	Currency string
}

// Preview reads the file and saves an Import in preview. Nothing goes into the Ledger.
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
	newCount, skipped := len(export.CashOperations), 0
	if account != nil {
		known, err := s.store.CashOperationIDs(ctx, account.ID)
		if err != nil {
			return Preview{}, err
		}
		newCount, skipped = 0, 0
		for _, op := range export.CashOperations {
			if known[op.ID] {
				skipped++
			} else {
				newCount++
			}
		}
	}

	id, err := s.store.CreatePreview(ctx, store.NewPreview{
		AccountNumber:         export.AccountNumber,
		FileName:              fileName,
		File:                  data,
		CreatedAt:             s.now(),
		NewCashOperations:     newCount,
		SkippedCashOperations: skipped,
	})
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		ID:                    id,
		FileName:              fileName,
		AccountNumber:         export.AccountNumber,
		Account:               account,
		SuggestedCurrency:     xtb.CurrencyFromFileName(fileName),
		NewCashOperations:     newCount,
		SkippedCashOperations: skipped,
	}, nil
}

// Confirm saves the Import in preview with the ID: the Account (if new), the Instruments,
// the Cash Operations and the Lots, in one transaction.
// A known account number ignores newAccount. A new account number needs a valid newAccount.
func (s *Service) Confirm(ctx context.Context, id int64, newAccount NewAccount) (store.Import, error) {
	pending, err := s.store.PendingImport(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.Import{}, fmt.Errorf("%w: %d", ErrImportNotFound, id)
	}
	if err != nil {
		return store.Import{}, err
	}
	if pending.State != "preview" {
		return store.Import{}, fmt.Errorf("%w: import %d", ErrAlreadyConfirmed, id)
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

	saved, err := s.store.SaveImport(ctx, store.SaveImport{
		ImportID:       id,
		Account:        *account,
		ImportedAt:     s.now(),
		CashOperations: cashOperations(export),
	})
	if errors.Is(err, store.ErrImportNotPending) {
		return store.Import{}, fmt.Errorf("%w: import %d", ErrAlreadyConfirmed, id)
	}
	return saved, err
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
	if !sum.Equal(export.Total) {
		return xtb.Export{}, &RejectedError{Err: fmt.Errorf("%w: the sum is %s, the Total row is %s", ErrTotalMismatch, sum, export.Total)}
	}
	return export, nil
}

// cashOperations converts the Cash Operations of the export for the store.
func cashOperations(export xtb.Export) []store.CashOperation {
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
		if op.Purchase != nil {
			// One Fill per Lot. The Lot ID is the XTB position ID. Ticket 06 adds the rule of ADR 0002.
			saved.Volume = decimal.NewNullDecimal(op.Purchase.Volume)
			saved.Price = decimal.NewNullDecimal(op.Purchase.Price)
			saved.IsFill = true
		}
		ops = append(ops, saved)
	}
	return ops
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
