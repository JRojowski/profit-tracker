package importer

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// transferWindow is the largest time difference between the two sides of one Transfer, inclusive.
// ADR 0003: the times of the two sides differ by milliseconds. The reader rounds each time to the second.
const transferWindow = 5 * time.Second

// WarningTransferWithoutOtherSide is the kind of a Warning for a Transfer with no other side in the other Accounts.
const WarningTransferWithoutOtherSide = "transfer-without-other-side"

// Warning is a note in the Import preview. A Warning never stops the Import.
type Warning struct {
	Kind string
	// XTBID is the XTB ID of the Cash Operation in the file.
	XTBID string
	Time  time.Time
	// Direction is the direction of the Transfer.
	Direction xtb.TransferDirection
	// Amount has the sign of the export.
	Amount decimal.Decimal
}

// transferWarnings returns a Warning for each Transfer of the file with no other side in the stored Cash Operations
// of the other Accounts. ADR 0003, rule 5: the other side has the opposite amount and a time within transferWindow.
// A stored Transfer is the other side of at most one Transfer of the file. The Transfers of the file take
// the other sides in time order and then in XTB ID order, so the result does not depend on the file order.
// A nil account is a new Account, so all stored Accounts are other Accounts.
// The warnings are in the same order. The result is empty, not nil, if there are no warnings.
func (s *Service) transferWarnings(ctx context.Context, account *store.Account, ops []xtb.CashOperation) ([]Warning, error) {
	var transfers []xtb.CashOperation
	for _, op := range ops {
		if op.Transfer != "" {
			transfers = append(transfers, op)
		}
	}
	if len(transfers) == 0 {
		return []Warning{}, nil
	}
	slices.SortFunc(transfers, func(a, b xtb.CashOperation) int {
		return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(a.ID, b.ID))
	})

	var accountID int64
	if account != nil {
		accountID = account.ID
	}
	others, err := s.store.TransfersOfOtherAccounts(ctx, accountID)
	if err != nil {
		return nil, err
	}

	warnings := []Warning{}
	used := make([]bool, len(others))
	for _, op := range transfers {
		if i := otherSide(op, others, used); i >= 0 {
			used[i] = true
			continue
		}
		warnings = append(warnings, Warning{
			Kind: WarningTransferWithoutOtherSide, XTBID: op.ID, Time: op.Time, Direction: op.Transfer, Amount: op.Amount,
		})
	}
	return warnings, nil
}

// otherSide returns the index of the first Transfer in others that is not used and is the other side of op,
// or -1 if there is none.
func otherSide(op xtb.CashOperation, others []store.Transfer, used []bool) int {
	for i, other := range others {
		if used[i] || !other.Amount.Equal(op.Amount.Neg()) {
			continue
		}
		if op.Time.Sub(other.Time).Abs() <= transferWindow {
			return i
		}
	}
	return -1
}
