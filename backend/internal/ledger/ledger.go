// Package ledger calculates the state of one Account from its Ledger: the cash balance,
// the Net Deposits, the Holdings and the sales with their Realized Profit. It is a pure calculation with no I/O.
// The tool calculates this state on each request and does not store it.
package ledger

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// The Cash Operation types that the calculator uses, as the broker export writes them.
const (
	TypeDeposit   = "Deposit"
	TypeStockSell = "Stock sell"
)

// Errors of Calculate. Each error that Calculate returns wraps one of them.
var (
	// ErrUnknownLot tells that a sale closes a Lot that the Ledger does not have.
	ErrUnknownLot = errors.New("the sale closes a lot that the ledger does not have")
	// ErrLotOverclosed tells that the sales of a Lot close more than the Lot volume.
	ErrLotOverclosed = errors.New("the sales close more than the lot volume")
)

// Ledger is the input of Calculate: all Cash Operations of one Account and its Lots.
type Ledger struct {
	// CashOperations are all Cash Operations of the Account, in time order.
	CashOperations []CashOperation
	// Lots are the Lots of the Account, each with its Fills.
	Lots []Lot
}

// CashOperation is one entry of the Ledger.
type CashOperation struct {
	// Type is the type as the broker export writes it, for example "Deposit" or "Stock purchase".
	Type string
	// Amount is in the Account currency. A purchase has a negative amount.
	Amount decimal.Decimal
	// XTBID is the XTB ID of the operation.
	XTBID string
	Time  time.Time
	// PositionID is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
	// Volume is the parsed volume. For a Stock sell, it is the closed volume of the Lot.
	// It is zero for an operation with no volume.
	Volume decimal.Decimal
}

// Instrument is one security, identified by its broker ticker.
type Instrument struct {
	Ticker string
	Name   string
}

// Lot is one or more Fills of an Instrument that the broker tracks as one unit.
type Lot struct {
	// PositionID is the Lot ID, the XTB position ID.
	PositionID string
	Instrument Instrument
	Fills      []Fill
}

// Fill is one purchase Cash Operation of a Lot.
type Fill struct {
	// XTBID is the XTB ID of the purchase.
	XTBID  string
	Volume decimal.Decimal
	// Amount is the purchase amount in the Account currency. It is negative.
	Amount decimal.Decimal
}

// State is the result of Calculate.
type State struct {
	// Cash is the cash balance: the sum of all Cash Operation amounts.
	Cash decimal.Decimal
	// NetDeposits is the money that the user put into the Account. See GLOSSARY.md, "Net Deposits".
	NetDeposits decimal.Decimal
	// Holdings has one Holding per Instrument with an open volume, in ticker order.
	Holdings []Holding
	// Sales are the Stock sell operations with their Realized Profit, in time order and then in XTB ID order.
	Sales []Sale
}

// Sale is one Stock sell: the close of a part of one Lot. See ADR 0001.
type Sale struct {
	XTBID string
	Time  time.Time
	// Instrument is the Instrument of the Lot.
	Instrument Instrument
	// PositionID is the ID of the Lot that the sale closes.
	PositionID string
	// Volume is the closed volume.
	Volume decimal.Decimal
	// Amount is the sale amount in the Account currency.
	Amount decimal.Decimal
	// Cost is the cost of the closed part: the Lot purchase amount × the closed volume / the Lot volume.
	Cost decimal.Decimal
	// RealizedProfit is Amount minus Cost. It can be negative.
	RealizedProfit decimal.Decimal
}

// Holding is all open Lots of one Instrument, taken together.
type Holding struct {
	Instrument Instrument
	// Volume is the open volume.
	Volume decimal.Decimal
	// Cost is the open cost in the Account currency. It is positive.
	Cost decimal.Decimal
}

// AverageCost returns the open cost per unit. The division keeps decimal.DivisionPrecision (16) decimal places.
// It does not round to the cent. The frontend rounds for display.
func (h Holding) AverageCost() decimal.Decimal {
	return h.Cost.Div(h.Volume)
}

// Calculate returns the state of the Account from its Ledger. The values are exact. Calculate does not round.
// It returns an error that wraps ErrUnknownLot or ErrLotOverclosed if a sale does not fit the Lots.
func Calculate(l Ledger) (State, error) {
	s := State{Cash: decimal.Zero, NetDeposits: decimal.Zero}
	for _, op := range l.CashOperations {
		s.Cash = s.Cash.Add(op.Amount)
		s.NetDeposits = s.NetDeposits.Add(netDeposit(op))
	}

	lots := make(map[string]*lotState, len(l.Lots))
	for _, lot := range l.Lots {
		lots[lot.PositionID] = newLotState(lot)
	}
	sales, err := closeLots(l.CashOperations, lots)
	if err != nil {
		return State{}, err
	}
	s.Sales = sales
	s.Holdings = holdings(l.Lots, lots)
	return s, nil
}

// netDeposit returns the part of op that counts in the Net Deposits.
// ADR 0003 adds Transfers and withdrawals here when the reader accepts them.
func netDeposit(op CashOperation) decimal.Decimal {
	switch op.Type {
	case TypeDeposit:
		return op.Amount
	default:
		return decimal.Zero
	}
}

// lotState is one Lot with the volume and the cost that its sales closed.
type lotState struct {
	instrument Instrument
	// volume is the Lot volume: the sum of the Fill volumes.
	volume decimal.Decimal
	// purchase is the purchase amount of the Lot. It is positive.
	purchase decimal.Decimal
	// closedVolume and closedCost are the sums over the sales of the Lot.
	closedVolume decimal.Decimal
	closedCost   decimal.Decimal
}

func newLotState(lot Lot) *lotState {
	st := &lotState{instrument: lot.Instrument, volume: decimal.Zero, purchase: decimal.Zero, closedVolume: decimal.Zero, closedCost: decimal.Zero}
	for _, f := range lot.Fills {
		st.volume = st.volume.Add(f.Volume)
		st.purchase = st.purchase.Sub(f.Amount)
	}
	return st
}

// openVolume is the volume that no sale closed.
func (st *lotState) openVolume() decimal.Decimal {
	return st.volume.Sub(st.closedVolume)
}

// openCost is the remaining part of the purchase amount. ADR 0001, rule 5.
// A closed Lot has no open cost, also when the cost of its parts has a rest from the division.
func (st *lotState) openCost() decimal.Decimal {
	if st.openVolume().IsZero() {
		return decimal.Zero
	}
	return st.purchase.Sub(st.closedCost)
}

// closeLots calculates the Realized Profit of each Stock sell and records the closed part in its Lot.
// It returns the sales in time order and then in XTB ID order.
func closeLots(ops []CashOperation, lots map[string]*lotState) ([]Sale, error) {
	var sales []Sale
	for _, op := range ops {
		if op.Type != TypeStockSell {
			continue
		}
		st, ok := lots[op.PositionID]
		if !ok {
			return nil, fmt.Errorf("sale %s closes lot %s: %w", op.XTBID, op.PositionID, ErrUnknownLot)
		}
		st.closedVolume = st.closedVolume.Add(op.Volume)
		if !st.volume.IsPositive() || st.openVolume().IsNegative() {
			return nil, fmt.Errorf("sale %s closes %s of lot %s, but the lot volume is %s: %w",
				op.XTBID, op.Volume, op.PositionID, st.volume, ErrLotOverclosed)
		}
		// ADR 0001, rule 3. The multiplication comes first, so an exact result stays exact.
		cost := st.purchase.Mul(op.Volume).Div(st.volume)
		st.closedCost = st.closedCost.Add(cost)
		sales = append(sales, Sale{
			XTBID:          op.XTBID,
			Time:           op.Time,
			Instrument:     st.instrument,
			PositionID:     op.PositionID,
			Volume:         op.Volume,
			Amount:         op.Amount,
			Cost:           cost,
			RealizedProfit: op.Amount.Sub(cost),
		})
	}
	slices.SortStableFunc(sales, func(a, b Sale) int {
		return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(a.XTBID, b.XTBID))
	})
	return sales, nil
}

// holdings adds the open part of the Lots per Instrument. A fully closed Lot adds nothing.
func holdings(lots []Lot, states map[string]*lotState) []Holding {
	byTicker := map[string]*Holding{}
	for _, lot := range lots {
		st := states[lot.PositionID]
		if !st.openVolume().IsPositive() {
			continue
		}
		h, ok := byTicker[lot.Instrument.Ticker]
		if !ok {
			h = &Holding{Instrument: lot.Instrument, Volume: decimal.Zero, Cost: decimal.Zero}
			byTicker[lot.Instrument.Ticker] = h
		}
		h.Volume = h.Volume.Add(st.openVolume())
		h.Cost = h.Cost.Add(st.openCost())
	}

	result := make([]Holding, 0, len(byTicker))
	for _, h := range byTicker {
		result = append(result, *h)
	}
	slices.SortFunc(result, func(a, b Holding) int { return cmp.Compare(a.Instrument.Ticker, b.Instrument.Ticker) })
	return result
}
