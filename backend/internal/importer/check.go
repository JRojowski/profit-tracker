package importer

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// checkTolerance is the largest difference between the Realized Profit and the Closed Positions
// Profit/Loss that is not an Import Conflict. See ADR 0001, "Check against Closed Positions".
var checkTolerance = decimal.RequireFromString("0.01")

// errLedgerNotValid starts the text of an error for a Ledger that the calculator cannot use.
var errLedgerNotValid = errors.New("the ledger after the import is not valid")

// invalidLedgerError tells that the Ledger after the Import fails ledger.Calculate.
type invalidLedgerError struct {
	Err error
}

func (e *invalidLedgerError) Error() string { return fmt.Sprintf("%v: %v", errLedgerNotValid, e.Err) }

// Unwrap returns the error of ledger.Calculate.
func (e *invalidLedgerError) Unwrap() error { return e.Err }

// checkLedger returns an *invalidLedgerError if ledger.Calculate fails for l.
// Confirm runs it on the Ledger after the decisions. A "use-new" decision can change a purchase or a sale,
// so that a sale names an unknown Lot or closes more than the Lot volume.
func checkLedger(l ledger.Ledger) error {
	if _, err := ledger.Calculate(l); err != nil {
		return &invalidLedgerError{Err: err}
	}
	return nil
}

// withLots returns the Ledger l with each Fill in lotIDs moved to its Lot ID, by XTB ID.
// It makes a Lot that does not exist, and it removes a Lot with no Fills. Store.SaveImport makes the same moves.
func withLots(l ledger.Ledger, lotIDs map[string]string) ledger.Ledger {
	lots := make([]ledger.Lot, 0, len(l.Lots))
	index := make(map[string]int, len(l.Lots))
	for _, lot := range l.Lots {
		index[lot.PositionID] = len(lots)
		lots = append(lots, ledger.Lot{PositionID: lot.PositionID, Instrument: lot.Instrument})
	}
	for _, lot := range l.Lots {
		for _, f := range lot.Fills {
			id, ok := lotIDs[f.XTBID]
			if !ok {
				id = lot.PositionID
			}
			i, ok := index[id]
			if !ok {
				i = len(lots)
				index[id] = i
				lots = append(lots, ledger.Lot{PositionID: id, Instrument: lot.Instrument})
			}
			lots[i].Fills = append(lots[i].Fills, f)
		}
	}
	l.Lots = slices.DeleteFunc(lots, func(lot ledger.Lot) bool { return len(lot.Fills) == 0 })
	return l
}

// withNewOperations returns the Ledger l with the new Cash Operations of the Import added.
// ops are all Cash Operations of the file, and newOps are the ones that are not stored, by XTB ID.
// A known XTB ID keeps its stored data, because a decision for a changed-data conflict is not known yet.
func withNewOperations(l ledger.Ledger, ops []store.CashOperation, newOps map[string]store.CashOperation) ledger.Ledger {
	lots := make(map[string]int, len(l.Lots))
	for i, lot := range l.Lots {
		lots[lot.PositionID] = i
	}
	for _, op := range ops {
		if _, ok := newOps[op.XTBID]; !ok {
			continue
		}
		l.CashOperations = append(l.CashOperations, ledger.CashOperation{
			XTBID:      op.XTBID,
			Type:       op.Type,
			Time:       op.Time,
			Amount:     op.Amount,
			PositionID: op.PositionID,
			Volume:     op.Volume.Decimal,
		})
		if !op.IsFill {
			continue
		}
		i, ok := lots[op.LotID]
		if !ok {
			i = len(l.Lots)
			lots[op.LotID] = i
			l.Lots = append(l.Lots, ledger.Lot{
				PositionID: op.LotID,
				Instrument: ledger.Instrument{Ticker: op.Ticker, Name: op.InstrumentName},
			})
		}
		l.Lots[i].Fills = append(l.Lots[i].Fills, ledger.Fill{XTBID: op.XTBID, Volume: op.Volume.Decimal, Amount: op.Amount})
	}
	return l
}

// checkRealizedProfit compares the Realized Profit of each new sale with its row in Closed Positions.
// A difference of more than 0.01 is an Import Conflict of the kind "check-mismatch".
// sales are all sales of the Ledger after the Import, in time order. A sale that is not in newOps
// only takes its row, so that a new sale cannot take it. A sale with no row gives no conflict.
func checkRealizedProfit(sales []ledger.Sale, rows []xtb.ClosedPosition, newOps map[string]store.CashOperation) []store.NewImportConflict {
	var conflicts []store.NewImportConflict
	used := make([]bool, len(rows))
	for _, sale := range sales {
		i := matchClosedPosition(sale, rows, used)
		if i < 0 {
			continue
		}
		used[i] = true
		op, isNew := newOps[sale.XTBID]
		if !isNew {
			continue
		}
		broker := rows[i].ProfitLoss
		if sale.RealizedProfit.Sub(broker).Abs().LessThanOrEqual(checkTolerance) {
			continue
		}
		conflicts = append(conflicts, store.NewImportConflict{
			Kind:  KindCheckMismatch,
			XTBID: sale.XTBID,
			New:   op.Data(),
			Check: &store.Check{Calculated: sale.RealizedProfit, ClosedPositions: broker},
		})
	}
	return conflicts
}

// matchClosedPosition returns the index of the unused Closed Positions row of the sale, or -1.
// The row must have the position ID and the closed volume of the sale. If more than one row fits,
// the row with the close time nearest to the sale time wins. The first such row wins a tie.
func matchClosedPosition(sale ledger.Sale, rows []xtb.ClosedPosition, used []bool) int {
	best := -1
	for i, row := range rows {
		if used[i] || row.PositionID != sale.PositionID || !row.Volume.Equal(sale.Volume) {
			continue
		}
		if best < 0 || absDuration(row.CloseTime.Sub(sale.Time)) < absDuration(rows[best].CloseTime.Sub(sale.Time)) {
			best = i
		}
	}
	return best
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
