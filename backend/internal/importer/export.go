package importer

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// The Providers. See GLOSSARY.md, "Provider".
const (
	ProviderXTB    = "xtb"
	ProviderKraken = "kraken"
)

// TransferDirection is the direction of a Transfer. See ADR 0003.
type TransferDirection string

// The directions of a Transfer.
const (
	// TransferIn is the side of the Account that gets the money.
	TransferIn TransferDirection = "in"
	// TransferOut is the side of the Account that sends the money.
	TransferOut TransferDirection = "out"
)

// export is the Provider-neutral result of a parser. The Service uses only this shape.
// A parser runs the checks of the file before it returns. A failed check is a *RejectedError.
type export struct {
	// Provider is the Provider of the file, for example ProviderXTB.
	Provider      string
	AccountNumber string
	// SuggestedCurrency is the Account currency that the file tells. It is "" if the file does not tell it.
	SuggestedCurrency string
	// CashOperations are the Cash Operations of the file, for the store. A Fill has its Lot ID,
	// so the Cash Operations hold the Lots of the file too.
	CashOperations []store.CashOperation
	// Transfers are the Transfers of the file. The Service looks for the other side of each one. See ADR 0003.
	Transfers []transfer
	// Checks are the checks of the Ledger after the Import. Each check returns its Import Conflicts.
	Checks []ledgerCheck
}

// transfer is one Transfer of a file.
type transfer struct {
	// ID is the Provider ID of the Cash Operation.
	ID        string
	Time      time.Time
	Direction TransferDirection
	// Amount has the sign of the export.
	Amount decimal.Decimal
}

// ledgerCheck checks the Ledger after the Import. closedPositions are all Closed Positions of that Ledger,
// in time order. ops are all Cash Operations of the file, and newOps are the ones that are not stored, by Provider ID.
type ledgerCheck func(closedPositions []ledger.ClosedPosition, ops []store.CashOperation, newOps map[string]store.CashOperation) []store.NewImportConflict

// parse reads the file with the parser of its Provider. The XTB parser is the only parser now.
// An error in the file is a *RejectedError.
func parse(fileName string, data []byte) (export, error) {
	return parseXTB(fileName, data)
}
