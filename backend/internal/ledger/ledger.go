// Package ledger calculates the state of one Account from its Ledger: the cash balance,
// the Net Deposits and the Holdings. It is a pure calculation with no I/O.
// The tool calculates this state on each request and does not store it.
package ledger

import (
	"cmp"
	"slices"

	"github.com/shopspring/decimal"
)

// TypeDeposit is the Cash Operation type of a deposit, as the broker export writes it.
const TypeDeposit = "Deposit"

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
	// PositionID is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
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
func Calculate(l Ledger) State {
	s := State{Cash: decimal.Zero, NetDeposits: decimal.Zero}
	for _, op := range l.CashOperations {
		s.Cash = s.Cash.Add(op.Amount)
		s.NetDeposits = s.NetDeposits.Add(netDeposit(op))
	}
	s.Holdings = holdings(l.Lots)
	return s
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

// holdings adds the open part of the Lots per Instrument.
// Each Lot is fully open, because the reader does not accept sales yet. ADR 0001 adds the sales.
func holdings(lots []Lot) []Holding {
	byTicker := map[string]*Holding{}
	for _, lot := range lots {
		h, ok := byTicker[lot.Instrument.Ticker]
		if !ok {
			h = &Holding{Instrument: lot.Instrument, Volume: decimal.Zero, Cost: decimal.Zero}
			byTicker[lot.Instrument.Ticker] = h
		}
		for _, f := range lot.Fills {
			h.Volume = h.Volume.Add(f.Volume)
			h.Cost = h.Cost.Sub(f.Amount)
		}
	}

	result := make([]Holding, 0, len(byTicker))
	for _, h := range byTicker {
		if h.Volume.IsPositive() {
			result = append(result, *h)
		}
	}
	slices.SortFunc(result, func(a, b Holding) int { return cmp.Compare(a.Instrument.Ticker, b.Instrument.Ticker) })
	return result
}
