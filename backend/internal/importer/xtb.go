package importer

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/lot"
	"github.com/jrojowski/profit-tracker/backend/internal/price"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// totalTolerance is half a cent. The Total row check accepts a difference that is smaller than totalTolerance.
// Excel stores the Total row as a float sum, so the Total can have float noise, for example 983.029999999997.
// A difference of one cent is a real difference and the check rejects it.
var totalTolerance = decimal.New(5, -3)

// checkTolerance is the largest difference between the Realized Profit and the Closed Positions
// Profit/Loss that is not an Import Conflict. See ADR 0001, "Check against Closed Positions".
var checkTolerance = decimal.RequireFromString("0.01")

// parseXTB reads an XTB export. It runs the Total row check and builds the Lots with the rule of ADR 0002.
// Its Ledger check is the Realized Profit check of ADR 0001.
func parseXTB(fileName string, data []byte) (export, error) {
	file, err := readXTB(data)
	if err != nil {
		return export{}, err
	}
	ops, err := xtbCashOperations(file)
	if err != nil {
		return export{}, err
	}
	var transfers []transfer
	for _, op := range file.CashOperations {
		if op.Transfer == "" {
			continue
		}
		transfers = append(transfers, transfer{
			ID: op.ID, Time: op.Time, Direction: TransferDirection(op.Transfer), Amount: op.Amount,
		})
	}
	rows := file.ClosedPositions
	checkRows := func(closedPositions []ledger.ClosedPosition, ops []store.CashOperation, newOps map[string]store.CashOperation) []store.NewImportConflict {
		return checkRealizedProfit(closedPositions, rows, ops, newOps)
	}
	return export{
		Provider:          ProviderXTB,
		AccountNumber:     file.AccountNumber,
		SuggestedCurrency: xtb.CurrencyFromFileName(fileName),
		CashOperations:    ops,
		Transfers:         transfers,
		Checks:            []ledgerCheck{checkRows},
	}, nil
}

// readXTB reads the XTB export and runs the Total row check.
func readXTB(data []byte) (xtb.Export, error) {
	file, err := xtb.Read(data)
	if err != nil {
		return xtb.Export{}, &RejectedError{Err: err}
	}

	sum := decimal.Zero
	for _, op := range file.CashOperations {
		sum = sum.Add(op.Amount)
	}
	// The Total row check: a difference means that the reader lost rows.
	if sum.Sub(file.Total).Abs().GreaterThanOrEqual(totalTolerance) {
		return xtb.Export{}, &RejectedError{Err: fmt.Errorf("%w: the sum is %s, the Total row is %s", ErrTotalMismatch, sum, file.Total)}
	}
	return file, nil
}

// xtbCashOperations converts the Cash Operations of the XTB export for the store.
// It builds the Lots of the Fills with the rule of ADR 0002. An order that does not fit the rule is a *RejectedError.
func xtbCashOperations(file xtb.Export) ([]store.CashOperation, error) {
	lots, err := lot.Build(file.CashOperations, file.ClosedPositions, file.OpenLots)
	if err != nil {
		return nil, &RejectedError{Err: err}
	}
	ops := make([]store.CashOperation, 0, len(file.CashOperations))
	for _, op := range file.CashOperations {
		saved := store.CashOperation{
			XTBID:          op.ID,
			Type:           string(op.Type),
			Ticker:         op.Ticker,
			InstrumentName: op.Instrument,
			Category:       op.Category,
			QuoteCurrency:  price.QuoteCurrency(op.Ticker),
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
			// ADR 0001: the volume of a Closed Position is the closed volume of its Lot.
			saved.Volume = decimal.NewNullDecimal(op.Sale.Volume)
			saved.Price = decimal.NewNullDecimal(op.Sale.Price)
		}
		ops = append(ops, saved)
	}
	return ops, nil
}

// checkRealizedProfit compares the Realized Profit of each new Closed Position with its row in Closed Positions.
// A difference of more than 0.01 is an Import Conflict of the kind "check-mismatch".
// Closed Positions are all Closed Positions of the Ledger after the Import, in time order. A Closed Position that is not in newOps
// only takes its row, so that a new Closed Position cannot take it. A Closed Position with no row gives no conflict.
// ops are all Cash Operations of the file. They give the sale price of each Closed Position in the file.
func checkRealizedProfit(closedPositions []ledger.ClosedPosition, rows []xtb.ClosedPosition, ops []store.CashOperation, newOps map[string]store.CashOperation) []store.NewImportConflict {
	// A Closed Position that is not in the file has no price, because the Ledger does not keep prices.
	prices := make(map[string]decimal.NullDecimal, len(ops))
	for _, op := range ops {
		prices[op.XTBID] = op.Price
	}
	var conflicts []store.NewImportConflict
	used := make([]bool, len(rows))
	for _, closedPosition := range closedPositions {
		i := matchClosedPosition(closedPosition, prices[closedPosition.XTBID], rows, used)
		if i < 0 {
			continue
		}
		used[i] = true
		op, isNew := newOps[closedPosition.XTBID]
		if !isNew {
			continue
		}
		broker := rows[i].ProfitLoss
		if closedPosition.RealizedProfit.Sub(broker).Abs().LessThanOrEqual(checkTolerance) {
			continue
		}
		conflicts = append(conflicts, store.NewImportConflict{
			Kind:  KindCheckMismatch,
			XTBID: closedPosition.XTBID,
			New:   op.Data(),
			Check: &store.Check{Calculated: closedPosition.RealizedProfit, ClosedPositions: broker},
		})
	}
	return conflicts
}

// matchClosedPosition returns the index of the unused Closed Positions row of the Closed Position, or -1.
// The row must have the position ID and the closed volume of the Closed Position. If more than one row fits,
// the row with the close time nearest to the Closed Position time wins. On a tie, the row with the Close Price
// nearest to the sale price wins. A difference, not equality, because the cell can carry float noise.
// If the sale price or a Close Price is not known, the first row of the tie wins.
func matchClosedPosition(closedPosition ledger.ClosedPosition, price decimal.NullDecimal, rows []xtb.ClosedPosition, used []bool) int {
	best := -1
	for i, row := range rows {
		if used[i] || row.PositionID != closedPosition.PositionID || !row.Volume.Equal(closedPosition.Volume) {
			continue
		}
		if best < 0 || nearer(row, rows[best], closedPosition.Time, price) {
			best = i
		}
	}
	return best
}

// nearer tells that the row a fits the Closed Position with the time at and the sale price better than the row b.
func nearer(a, b xtb.ClosedPosition, at time.Time, price decimal.NullDecimal) bool {
	da, db := absDuration(a.CloseTime.Sub(at)), absDuration(b.CloseTime.Sub(at))
	if da != db {
		return da < db
	}
	if !price.Valid || !a.ClosePrice.Valid || !b.ClosePrice.Valid {
		return false
	}
	pa := a.ClosePrice.Decimal.Sub(price.Decimal).Abs()
	pb := b.ClosePrice.Decimal.Sub(price.Decimal).Abs()
	return pa.LessThan(pb)
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
