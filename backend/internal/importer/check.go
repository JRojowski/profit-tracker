package importer

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

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
// Confirm runs it on the Ledger after the decisions. A "use-new" decision can change a purchase or a Closed Position,
// so that a Closed Position names an unknown Lot or closes more than the Lot volume.
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
			Ticker:     op.Ticker,
			PositionID: op.PositionID,
			Volume:     op.Volume.Decimal,
			Price:      op.Price,
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
		l.Lots[i].Fills = append(l.Lots[i].Fills, ledger.Fill{XTBID: op.XTBID, Time: op.Time, Volume: op.Volume.Decimal, Price: op.Price, Amount: op.Amount})
	}
	return l
}
