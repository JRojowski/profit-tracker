// Package ledger calculates the state of one Account from its Ledger, the prices and the Exchange Rates.
// The state has the cash balance, the Net Deposits and the Holdings with their value and Unrealized Profit.
// It also has the sales with their Realized Profit, and the total result with its parts.
// It is a pure calculation with no I/O.
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
	TypeDeposit        = "Deposit"
	TypeWithdrawal     = "Withdrawal"
	TypeStockSell      = "Stock sell"
	TypeDividend       = "Dividend"
	TypeWithholdingTax = "Withholding tax"
	TypeInterest       = "Free funds interest"
	TypeInterestTax    = "Free funds interest tax"
	TypeSECFee         = "SEC fee"
	TypeCorrection     = "Correction"
	// TypeTransfer is a Transfer between two Accounts. See ADR 0003.
	TypeTransfer = "IKZE deposit"
)

// Errors of Calculate. Each error that Calculate returns wraps one of them.
var (
	// ErrUnknownLot tells that a sale closes a Lot that the Ledger does not have.
	ErrUnknownLot = errors.New("the sale closes a lot that the ledger does not have")
	// ErrLotOverclosed tells that the sales of a Lot close more than the Lot volume.
	ErrLotOverclosed = errors.New("the sales close more than the lot volume")
)

// PLN is the currency of the Exchange Rates. Each rate is the PLN amount for 1 unit of a currency.
const PLN = "PLN"

// Ledger is the input of Calculate: all Cash Operations of one Account, its Lots, the prices and the Exchange Rates.
type Ledger struct {
	// Currency is the Account currency, for example "USD".
	Currency string
	// CashOperations are all Cash Operations of the Account, in time order.
	CashOperations []CashOperation
	// Lots are the Lots of the Account, each with its Fills.
	Lots []Lot
	// QuoteCurrencies are the Quote Currencies of the Instruments, by broker ticker.
	// An Instrument that is not in the map has an unknown Quote Currency.
	QuoteCurrencies map[string]string
	// Prices are the close prices in the Quote Currency of each Instrument, by broker ticker.
	// A Holding with no price has no value and no Unrealized Profit.
	Prices map[string]decimal.Decimal
	// ExchangeRates are the latest Exchange Rates, by currency.
	ExchangeRates map[string]ExchangeRate
}

// ExchangeRate is one Exchange Rate of a currency against PLN.
type ExchangeRate struct {
	// Rate is the PLN amount for 1 unit of the currency.
	Rate decimal.Decimal
	// Date is the NBP table date.
	Date time.Time
}

// NoValueReason tells why a Holding has no value.
type NoValueReason int

const (
	// ReasonNone tells that the Holding has a value.
	ReasonNone NoValueReason = iota
	// ReasonUnknownQuoteCurrency tells that the Quote Currency of the Instrument is not known.
	ReasonUnknownQuoteCurrency
	// ReasonCrossRate tells that the Quote Currency and the Account currency are both not PLN and not equal.
	// The tool has no rate between two such currencies.
	ReasonCrossRate
	// ReasonNoPrice tells that the Ledger has no price for the Instrument.
	ReasonNoPrice
	// ReasonNoExchangeRate tells that the Ledger has no Exchange Rate for the conversion to the Account currency.
	ReasonNoExchangeRate
)

// rateCurrency returns the currency whose Exchange Rate converts an amount in the Quote Currency quote
// to the Account currency account. It is the currency of the two that is not PLN.
// It returns "" and true if quote equals account, because then no conversion is necessary.
// It returns false if quote and account are both not PLN and not equal.
func rateCurrency(quote, account string) (string, bool) {
	switch {
	case quote == account:
		return "", true
	case account == PLN:
		return quote, true
	case quote == PLN:
		return account, true
	default:
		return "", false
	}
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
	// RealizedProfit is the sum of the Realized Profit of the sales.
	RealizedProfit decimal.Decimal
	// NetDividends is the sum of the dividends and the withholding tax. The tax is negative.
	NetDividends decimal.Decimal
	// NetInterest is the sum of the free funds interest and its tax. The tax is negative.
	NetInterest decimal.Decimal
	// Fees is the sum of the SEC fees and the corrections, with the sign of the export. It is usually negative.
	Fees decimal.Decimal
	// UnrealizedProfit is the sum of the Unrealized Profit of the Holdings that have a price.
	UnrealizedProfit decimal.Decimal
	// Value is the cash balance plus the value of the Holdings that have a price.
	Value decimal.Decimal
	// TotalResult is RealizedProfit + UnrealizedProfit + NetDividends + NetInterest + Fees.
	TotalResult decimal.Decimal
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
	// NoValueReason tells why the Holding has no value in the Account currency. It is ReasonNone for a Holding
	// with a value. For a Holding with no value, Price, ExchangeRate, Value and UnrealizedProfit are zero.
	NoValueReason NoValueReason
	// Price is the close price in the Quote Currency.
	Price decimal.Decimal
	// ExchangeRateCurrency is the currency whose Exchange Rate converts the value to the Account currency:
	// the currency of the two that is not PLN. It is empty if the value needs no conversion or no conversion is
	// possible. It is set also when the Ledger has no price or no rate.
	ExchangeRateCurrency string
	// ExchangeRate is the Exchange Rate that the value uses. It is zero if the value uses no rate.
	ExchangeRate ExchangeRate
	// Value is the open volume × the price in the Account currency.
	Value decimal.Decimal
	// UnrealizedProfit is Value minus Cost. It can be negative.
	UnrealizedProfit decimal.Decimal
}

// HasValue tells if the Holding has a value in the Account currency.
func (h Holding) HasValue() bool {
	return h.NoValueReason == ReasonNone
}

// UsesExchangeRate tells if the value of the Holding uses ExchangeRate.
func (h Holding) UsesExchangeRate() bool {
	return h.HasValue() && h.ExchangeRateCurrency != ""
}

// AverageCost returns the open cost per unit. The division keeps decimal.DivisionPrecision (16) decimal places.
// It does not round to the cent. The frontend rounds for display.
func (h Holding) AverageCost() decimal.Decimal {
	return h.Cost.Div(h.Volume)
}

// UnrealizedProfitPercent returns the Unrealized Profit × 100 / the open cost. The multiplication comes first,
// so an exact result stays exact. The division keeps decimal.DivisionPrecision (16) decimal places.
// It returns false if the Holding has no value or no cost.
func (h Holding) UnrealizedProfitPercent() (decimal.Decimal, bool) {
	if !h.HasValue() || h.Cost.IsZero() {
		return decimal.Zero, false
	}
	return h.UnrealizedProfit.Mul(decimal.NewFromInt(100)).Div(h.Cost), true
}

// Calculate returns the state of the Account from its Ledger. The values are exact. Calculate does not round.
// It returns an error that wraps ErrUnknownLot or ErrLotOverclosed if a sale does not fit the Lots.
func Calculate(l Ledger) (State, error) {
	s := State{
		Cash: decimal.Zero, NetDeposits: decimal.Zero, RealizedProfit: decimal.Zero,
		NetDividends: decimal.Zero, NetInterest: decimal.Zero, Fees: decimal.Zero,
	}
	for _, op := range l.CashOperations {
		s.add(op)
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
	s.Holdings = holdings(l, lots)
	for _, sale := range sales {
		s.RealizedProfit = s.RealizedProfit.Add(sale.RealizedProfit)
	}
	s.UnrealizedProfit, s.Value = decimal.Zero, s.Cash
	for _, h := range s.Holdings {
		s.UnrealizedProfit = s.UnrealizedProfit.Add(h.UnrealizedProfit)
		s.Value = s.Value.Add(h.Value)
	}
	s.TotalResult = s.RealizedProfit.Add(s.UnrealizedProfit).Add(s.NetDividends).Add(s.NetInterest).Add(s.Fees)
	return s, nil
}

// add adds the amount of op to the cash balance and to the parts of the state that op counts in.
// The amounts keep the sign of the export: a tax, a fee and a Withdrawal are negative.
func (s *State) add(op CashOperation) {
	s.Cash = s.Cash.Add(op.Amount)
	s.NetDeposits = s.NetDeposits.Add(netDeposit(op))
	switch op.Type {
	case TypeDividend, TypeWithholdingTax:
		s.NetDividends = s.NetDividends.Add(op.Amount)
	case TypeInterest, TypeInterestTax:
		s.NetInterest = s.NetInterest.Add(op.Amount)
	case TypeSECFee, TypeCorrection:
		s.Fees = s.Fees.Add(op.Amount)
	}
}

// netDeposit returns the part of op that counts in the Net Deposits.
// A Withdrawal and a Transfer Out have a negative amount in the export, so they decrease the Net Deposits.
// ADR 0003: a Transfer In counts as a deposit and a Transfer Out counts as a withdrawal.
func netDeposit(op CashOperation) decimal.Decimal {
	switch op.Type {
	case TypeDeposit, TypeWithdrawal, TypeTransfer:
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
// A Holding with a value gets its Unrealized Profit.
func holdings(l Ledger, states map[string]*lotState) []Holding {
	byTicker := map[string]*Holding{}
	for _, lot := range l.Lots {
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
		h.Price, h.Value, h.UnrealizedProfit = decimal.Zero, decimal.Zero, decimal.Zero
		h.setValue(l)
		result = append(result, *h)
	}
	slices.SortFunc(result, func(a, b Holding) int { return cmp.Compare(a.Instrument.Ticker, b.Instrument.Ticker) })
	return result
}

// setValue sets the value of the Holding in the Account currency from its price and, if necessary,
// the Exchange Rate. Otherwise it sets the reason why the Holding has no value.
func (h *Holding) setValue(l Ledger) {
	quote, ok := l.QuoteCurrencies[h.Instrument.Ticker]
	if !ok || quote == "" {
		h.NoValueReason = ReasonUnknownQuoteCurrency
		return
	}
	currency, ok := rateCurrency(quote, l.Currency)
	if !ok {
		h.NoValueReason = ReasonCrossRate
		return
	}
	h.ExchangeRateCurrency = currency
	closePrice, ok := l.Prices[h.Instrument.Ticker]
	if !ok {
		h.NoValueReason = ReasonNoPrice
		return
	}
	value := h.Volume.Mul(closePrice)
	if currency != "" {
		rate, ok := l.ExchangeRates[currency]
		// A rate of zero or less is not valid. It would give a wrong value or a division by zero.
		if !ok || !rate.Rate.IsPositive() {
			h.NoValueReason = ReasonNoExchangeRate
			return
		}
		value = convert(value, quote, rate.Rate)
		h.ExchangeRate = rate
	}
	h.Price, h.Value = closePrice, value
	h.UnrealizedProfit = value.Sub(h.Cost)
}

// convert converts the amount in the currency from to the other currency of the rate.
// The rate is the PLN amount for 1 unit of the currency that is not PLN.
// The division keeps decimal.DivisionPrecision (16) decimal places.
func convert(amount decimal.Decimal, from string, rate decimal.Decimal) decimal.Decimal {
	if from == PLN {
		return amount.Div(rate)
	}
	return amount.Mul(rate)
}
